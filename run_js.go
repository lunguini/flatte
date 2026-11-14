//go:build js && wasm

package flatte

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"syscall/js"

	"github.com/charmbracelet/colorprofile"
)

// Run uses the browser page by default on js/wasm, mounting the first element
// marked data-flatte. An Option may select another Host; application code
// keeps the same entry point used by native builds.
func Run[S any](ctx context.Context, app App[S], opts ...Option) (runErr error) {
	if app.State == nil {
		panic("flat: App.State is nil")
	}
	if app.View == nil {
		panic("flat: App.View is nil")
	}
	configured := applyRunOptions(runConfig{bracketedPaste: true}, opts...)
	if handled, err := runWithHost(ctx, app, configured); handled {
		return err
	}
	cfg := configured.config
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return errors.New("flat: browser document is unavailable")
	}
	surface := doc.Call("querySelector", "[data-flatte]")
	if surface.IsNull() || surface.IsUndefined() {
		return errors.New("flat: browser mount element [data-flatte] not found")
	}

	runCtx, cancel := context.WithCancel(ctx)
	events := make(chan []Event, 256)
	session := newSession(runCtx, app)
	host := newBrowserHost(runCtx, doc, surface, cfg, func(ev []Event) {
		select {
		case events <- ev:
		case <-runCtx.Done():
		}
	})
	session.UseHostServices(HostServices{
		Capabilities:  browserCapabilities(cfg),
		SetClipboard:  host.writeClipboard,
		ReadClipboard: host.readClipboard,
		Notify:        host.notify,
		CellSurface:   true,
		PublishMedia:  cfg.publishMedia,
		ShowDialog: func(d Dialog, done func(DialogResult)) {
			if d.Alert == nil {
				done(DialogResult{Err: ErrNoDialogs})
				return
			}
			session.cell.dialogs.show(d, done)
		},
	})
	session.useCellPolicy(sessionPolicy{defaultQuit: cfg.defaultQuit, dialogResizeRoutesThroughMenus: true})
	defer cancel()
	defer host.close()
	defer session.close()
	defer func() {
		if cfg.publishMedia != nil {
			cfg.publishMedia(nil)
		}
	}()

	initialResize := ResizeEvent{Width: host.columns(), Height: host.rows()}
	started := session.Step(StepInput{Start: &initialResize})
	if started.Quit {
		return nil
	}
	renderContext := func() RenderContext {
		return RenderContext{
			Width: host.columns(), Height: host.rows(), ColorProfile: colorprofile.TrueColor,
			Capabilities: session.caps(),
		}
	}
	step := func(input StepInput) bool {
		input.RenderContext = renderContext()
		result := session.Step(input)
		if result.Paint {
			host.render(result.Frame)
		}
		return !result.Quit
	}
	if !step(StepInput{FrameDue: true}) {
		return nil
	}

	for {
		select {
		case <-runCtx.Done():
			return nil
		case batch := <-events:
			if !step(StepInput{Events: batch, UpdatesPending: true, FrameDue: true}) {
				return nil
			}
		case <-session.Updates():
			if !step(StepInput{UpdatesPending: true, FrameDue: true}) {
				return nil
			}
		case event, ok := <-cfg.mediaCommands:
			if !ok {
				cfg.mediaCommands = nil
				continue
			}
			if !step(StepInput{Events: []Event{event}, UpdatesPending: true, FrameDue: true}) {
				return nil
			}
		}
	}
}

type browserListener struct {
	target js.Value
	event  string
	fn     js.Func
}

type browserHost struct {
	ctx                                  context.Context
	doc                                  js.Value
	surface                              js.Value
	content                              js.Value
	cursor                               js.Value
	input                                js.Value
	styles                               js.Value
	cfg                                  runConfig
	send                                 func([]Event)
	listeners                            []browserListener
	observer                             js.Value
	observeFn                            js.Func
	origTitle                            string
	composing                            bool
	closed                               bool
	cellWidth                            float64
	lineHeight                           float64
	padLeft, padRight, padTop, padBottom float64
	wheel                                browserWheelAccumulator
	wheelDelta                           float64
	hwheel                               browserWheelAccumulator
	hwheelDelta                          float64
	wheelX, wheelY                       float64
	wheelMod                             Mod
	frameID                              js.Value
	frameFn                              js.Func
	frameCancel                          string
	framePending                         bool
}

func newBrowserHost(ctx context.Context, doc, surface js.Value, cfg runConfig, send func([]Event)) *browserHost {
	h := &browserHost{ctx: ctx, doc: doc, surface: surface, cfg: cfg, send: send, origTitle: doc.Get("title").String()}
	style := surface.Get("style")
	if js.Global().Call("getComputedStyle", surface).Get("position").String() == "static" {
		style.Set("position", "relative")
	}
	surface.Set("tabIndex", 0)

	h.content = doc.Call("createElement", "span")
	h.content.Set("data-flatte-content", "")
	h.cursor = doc.Call("createElement", "span")
	h.cursor.Set("ariaHidden", "true")
	cs := h.cursor.Get("style")
	cs.Set("position", "absolute")
	cs.Set("pointerEvents", "none")
	cs.Set("display", "none")

	h.input = doc.Call("createElement", "textarea")
	h.input.Set("ariaLabel", "Flatte application input")
	h.input.Set("rows", 1)
	h.input.Set("spellcheck", false)
	is := h.input.Get("style")
	is.Set("position", "absolute")
	is.Set("padding", "0")
	is.Set("border", "0")
	is.Set("margin", "0")
	is.Set("resize", "none")
	is.Set("overflow", "hidden")
	is.Set("background", "transparent")
	is.Set("color", "transparent")
	is.Set("caretColor", "transparent")
	is.Set("opacity", "0.01")
	is.Set("zIndex", "2")
	h.styles = doc.Call("createElement", "style")
	h.styles.Set("textContent", "@keyframes flatte-blink{50%{opacity:0}}")
	if head := doc.Get("head"); !head.IsNull() && !head.IsUndefined() {
		head.Call("appendChild", h.styles)
	}

	surface.Call("replaceChildren", h.content, h.cursor, h.input)
	h.measureMetrics()
	h.bindInput()
	h.bindMouse()
	h.bindResize()
	h.input.Call("focus", map[string]any{"preventScroll": true})
	return h
}

func (h *browserHost) emit(ev Event) { h.emitBatch([]Event{ev}) }

func (h *browserHost) emitBatch(events []Event) {
	if h.closed || len(events) == 0 {
		return
	}
	h.send(events)
}

func (h *browserHost) listen(target js.Value, event string, fn func(js.Value)) {
	cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var ev js.Value
		if len(args) > 0 {
			ev = args[0]
		}
		fn(ev)
		return nil
	})
	target.Call("addEventListener", event, cb)
	h.listeners = append(h.listeners, browserListener{target: target, event: event, fn: cb})
}

func (h *browserHost) close() {
	h.closed = true
	if h.framePending {
		if cancel := js.Global().Get(h.frameCancel); cancel.Type() == js.TypeFunction {
			cancel.Invoke(h.frameID)
		}
		h.frameFn.Release()
		h.framePending = false
	}
	for i := len(h.listeners) - 1; i >= 0; i-- {
		l := h.listeners[i]
		l.target.Call("removeEventListener", l.event, l.fn)
		l.fn.Release()
	}
	if !h.observer.IsUndefined() && !h.observer.IsNull() {
		h.observer.Call("disconnect")
		h.observeFn.Release()
	}
	h.input.Call("remove")
	h.styles.Call("remove")
	h.doc.Set("title", h.origTitle)
}

func (h *browserHost) bindInput() {
	h.listen(h.surface, "pointerdown", func(js.Value) {
		h.input.Call("focus", map[string]any{"preventScroll": true})
	})
	// The pointer's default action then focuses the surface itself (it is
	// focusable for keyboard users); keys must keep reaching the input.
	h.listen(h.surface, "focus", func(js.Value) {
		h.input.Call("focus", map[string]any{"preventScroll": true})
	})
	h.listen(h.input, "keydown", func(ev js.Value) {
		// While an IME composes, its keys (Enter to commit, arrows to pick a
		// candidate, Backspace to edit) belong to it; the committed text
		// arrives through compositionend. Some engines omit isComposing, so
		// the bridge's own composition state and keyCode 229 also count.
		if h.composing || ev.Get("isComposing").Truthy() || ev.Get("keyCode").Equal(js.ValueOf(229)) {
			return
		}
		if browserPasteGesture(browserKeyFromDOM(ev)) {
			return // let the browser deliver its paste event
		}
		if key, ok := browserKeyEvent(ev); ok {
			ev.Call("preventDefault")
			h.emit(key)
		}
	})
	h.listen(h.input, "compositionstart", func(js.Value) { h.composing = true })
	h.listen(h.input, "compositionend", func(ev js.Value) {
		h.composing = false
		text := ev.Get("data").String()
		if text == "" {
			text = h.input.Get("value").String()
		}
		h.sendText(text)
		h.input.Set("value", "")
	})
	h.listen(h.input, "input", func(js.Value) {
		if h.composing {
			return
		}
		text := h.input.Get("value").String()
		h.sendText(text)
		h.input.Set("value", "")
	})
	h.listen(h.input, "paste", func(ev js.Value) {
		clipboard := ev.Get("clipboardData")
		if clipboard.IsNull() || clipboard.IsUndefined() {
			return
		}
		text := clipboard.Call("getData", "text").String()
		if text == "" {
			return
		}
		ev.Call("preventDefault")
		h.emitBatch(browserTextEvents(text, h.cfg.bracketedPaste))
		h.input.Set("value", "")
	})
	h.listen(h.input, "focus", func(js.Value) { h.emit(FocusEvent{Focused: true}) })
	h.listen(h.input, "blur", func(js.Value) { h.emit(FocusEvent{Focused: false}) })
}

func (h *browserHost) sendText(text string) {
	h.emitBatch(browserTextEvents(text, false))
}

func browserKeyEvent(ev js.Value) (KeyEvent, bool) {
	return translateBrowserKey(browserKeyFromDOM(ev))
}

func browserKeyFromDOM(ev js.Value) browserKey {
	return browserKey{Name: ev.Get("key").String(), Mod: browserMod(ev), AltGraph: browserAltGraph(ev)}
}

func browserMod(ev js.Value) Mod {
	return browserModifierMask(browserModifiers{
		Shift: ev.Get("shiftKey").Bool(), Alt: ev.Get("altKey").Bool(),
		Ctrl: ev.Get("ctrlKey").Bool(), Super: ev.Get("metaKey").Bool(),
	})
}

func browserAltGraph(ev js.Value) bool {
	get := ev.Get("getModifierState")
	return get.Type() == js.TypeFunction && ev.Call("getModifierState", "AltGraph").Truthy()
}

func (h *browserHost) bindMouse() {
	if h.cfg.mouse == MouseModeNone {
		return
	}
	pointer := func(action MouseAction) func(js.Value) {
		return func(ev js.Value) {
			if action == MouseMotion && h.cfg.mouse == MouseModeCellMotion && ev.Get("buttons").Int() == 0 {
				return
			}
			x, y := h.cell(ev)
			h.emit(MouseEvent{X: x, Y: y, Button: browserMouseButton(ev), Action: action, Mod: browserMod(ev)})
		}
	}
	h.listen(h.surface, "pointerdown", pointer(MousePress))
	h.listen(h.surface, "pointerup", pointer(MouseRelease))
	h.listen(h.surface, "pointermove", pointer(MouseMotion))
	h.listen(h.surface, "wheel", func(ev js.Value) {
		ev.Call("preventDefault")
		delta, across := ev.Get("deltaY").Float(), ev.Get("deltaX").Float()
		switch ev.Get("deltaMode").Int() {
		case 1: // DOM_DELTA_LINE
			delta *= h.lineHeight
			across *= h.lineHeight
		case 2: // DOM_DELTA_PAGE
			delta *= float64(h.surface.Get("clientHeight").Int())
			across *= float64(h.surface.Get("clientWidth").Int())
		}
		h.wheelDelta += delta
		h.hwheelDelta += across
		h.wheelX, h.wheelY = ev.Get("clientX").Float(), ev.Get("clientY").Float()
		h.wheelMod = browserMod(ev)
		h.scheduleWheel()
	})
}

func (h *browserHost) scheduleWheel() {
	if h.framePending {
		return
	}
	h.framePending = true
	h.frameFn = js.FuncOf(func(js.Value, []js.Value) any {
		h.framePending = false
		h.flushWheel()
		h.frameFn.Release()
		return nil
	})
	if request := js.Global().Get("requestAnimationFrame"); request.Type() == js.TypeFunction {
		h.frameCancel = "cancelAnimationFrame"
		h.frameID = request.Invoke(h.frameFn)
		return
	}
	h.frameCancel = "clearTimeout"
	h.frameID = js.Global().Call("setTimeout", h.frameFn, 16)
}

func (h *browserHost) flushWheel() {
	if h.closed {
		return
	}
	down, across := h.wheelDelta, h.hwheelDelta
	h.wheelDelta, h.hwheelDelta = 0, 0
	x, y := h.cellAt(h.wheelX, h.wheelY)
	var events []Event
	for _, axis := range []struct {
		notches    int
		less, more MouseButton
	}{
		{h.wheel.notches(down, h.lineHeight), MouseWheelUp, MouseWheelDown},
		{h.hwheel.notches(across, h.lineHeight), MouseWheelLeft, MouseWheelRight},
	} {
		notches, button := axis.notches, axis.more
		if notches < 0 {
			notches, button = -notches, axis.less
		}
		// Bound one frame's batch so a malformed DOM event cannot allocate an
		// unbounded event slice. Any discarded whole notches are already consumed.
		for range min(notches, 1024) {
			events = append(events, MouseEvent{X: x, Y: y, Button: button, Action: MousePress, Mod: h.wheelMod})
		}
	}
	if len(events) > 0 {
		h.emitBatch(events)
	}
}

func browserMouseButton(ev js.Value) MouseButton {
	return browserMouseButtonValue(ev.Get("button").Int())
}

func (h *browserHost) bindResize() {
	resize := func(js.Value) {
		w, rows := h.size()
		h.emit(ResizeEvent{Width: w, Height: rows})
	}
	observer := js.Global().Get("ResizeObserver")
	if observer.Type() == js.TypeFunction {
		h.observeFn = js.FuncOf(func(js.Value, []js.Value) any { resize(js.Value{}); return nil })
		h.observer = observer.New(h.observeFn)
		h.observer.Call("observe", h.surface)
	}
	h.listen(js.Global(), "resize", resize)
}

func (h *browserHost) size() (int, int) { return h.columns(), h.rows() }

func (h *browserHost) columns() int {
	width := float64(h.surface.Get("clientWidth").Int()) - h.padLeft - h.padRight
	if width < 1 {
		width = float64(js.Global().Get("innerWidth").Int()) - h.padLeft - h.padRight
	}
	return max(int(width/h.cellWidth), 1)
}

func (h *browserHost) rows() int {
	height := float64(h.surface.Get("clientHeight").Int()) - h.padTop - h.padBottom
	if height < 1 {
		height = float64(js.Global().Get("innerHeight").Int()) - h.padTop - h.padBottom
	}
	return max(int(height/h.lineHeight), 1)
}

// measureMetrics fixes the cell size for the lifetime of this mount. Resize
// events then map the container's changing pixel size onto the same grid.
func (h *browserHost) measureMetrics() {
	computed := js.Global().Call("getComputedStyle", h.surface)
	h.padLeft = browserPixels(computed.Call("getPropertyValue", "padding-left").String())
	h.padRight = browserPixels(computed.Call("getPropertyValue", "padding-right").String())
	h.padTop = browserPixels(computed.Call("getPropertyValue", "padding-top").String())
	h.padBottom = browserPixels(computed.Call("getPropertyValue", "padding-bottom").String())
	fontSize := browserPixels(computed.Call("getPropertyValue", "font-size").String())
	if fontSize <= 0 {
		fontSize = 13
	}
	h.lineHeight = browserPixels(computed.Call("getPropertyValue", "line-height").String())
	if h.lineHeight <= 0 {
		h.lineHeight = fontSize * 1.2
	}
	h.cellWidth = h.measureCharacterWidth()
	if h.cellWidth <= 0 {
		h.cellWidth = fontSize * 0.62
	}
}

func (h *browserHost) padding(side string) float64 {
	switch side {
	case "left":
		return h.padLeft
	case "right":
		return h.padRight
	case "top":
		return h.padTop
	case "bottom":
		return h.padBottom
	default:
		return 0
	}
}

func browserPixels(value string) float64 {
	n := js.Global().Call("parseFloat", strings.TrimSpace(strings.TrimSuffix(value, "px"))).Float()
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return n
}

func (h *browserHost) measureCharacterWidth() float64 {
	const samples = 100
	probe := h.doc.Call("createElement", "span")
	style := probe.Get("style")
	style.Set("position", "absolute")
	style.Set("visibility", "hidden")
	style.Set("whiteSpace", "pre")
	style.Set("left", "-9999px")
	style.Set("top", "0")
	style.Set("padding", "0")
	style.Set("border", "0")
	probe.Set("textContent", strings.Repeat("0", samples))
	h.surface.Call("appendChild", probe)
	width := probe.Call("getBoundingClientRect").Get("width").Float() / samples
	probe.Call("remove")
	return width
}

func (h *browserHost) browserClipboard() js.Value {
	navigator := js.Global().Get("navigator")
	if navigator.IsUndefined() || navigator.IsNull() {
		return js.Undefined()
	}
	return navigator.Get("clipboard")
}

func (h *browserHost) writeClipboard(text string) {
	defer func() { _ = recover() }() // Clipboard API permission and policy errors are optional.
	clipboard := h.browserClipboard()
	if clipboard.IsUndefined() || clipboard.IsNull() || clipboard.Get("writeText").Type() != js.TypeFunction {
		return
	}
	h.ignorePromise(clipboard.Call("writeText", text))
}

func (h *browserHost) readClipboard() {
	defer func() { _ = recover() }() // Clipboard API permission and policy errors are optional.
	clipboard := h.browserClipboard()
	if clipboard.IsUndefined() || clipboard.IsNull() || clipboard.Get("readText").Type() != js.TypeFunction {
		return
	}
	h.handlePromise(clipboard.Call("readText"), func(value js.Value) {
		h.emit(ClipboardEvent{Text: value.String()})
	})
}

func (h *browserHost) notify(n Notification) {
	defer func() { _ = recover() }() // A restricted or absent Web Notifications API is a no-op.
	if n.Title == "" && n.Body == "" {
		return
	}
	constructor := js.Global().Get("Notification")
	if constructor.Type() != js.TypeFunction || constructor.Get("permission").String() != "granted" {
		return
	}
	text := n.Title
	if text == "" {
		text = n.Body
	}
	options := map[string]any{}
	if n.Title != "" {
		options["body"] = n.Body
	}
	constructor.New(text, options)
}

func (h *browserHost) ignorePromise(promise js.Value) {
	h.handlePromise(promise, nil)
}

func (h *browserHost) handlePromise(promise js.Value, fulfilled func(js.Value)) {
	defer func() { _ = recover() }()
	if promise.IsUndefined() || promise.IsNull() || promise.Get("then").Type() != js.TypeFunction {
		return
	}
	var ok, fail js.Func
	release := func() {
		ok.Release()
		fail.Release()
	}
	ok = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if fulfilled != nil && len(args) > 0 && !h.closed {
			fulfilled(args[0])
		}
		release()
		return nil
	})
	fail = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		return nil
	})
	promise.Call("then", ok, fail)
}

func (h *browserHost) cell(ev js.Value) (int, int) {
	rect := h.surface.Call("getBoundingClientRect")
	return browserCellCoordinates(
		ev.Get("clientX").Float(), ev.Get("clientY").Float(),
		rect.Get("left").Float(), rect.Get("top").Float(),
		h.surface.Get("scrollLeft").Float(), h.surface.Get("scrollTop").Float(),
		h.columns(), h.rows(), h.cellWidth, h.lineHeight, h.padLeft, h.padTop,
	)
}

func (h *browserHost) cellAt(clientX, clientY float64) (int, int) {
	rect := h.surface.Call("getBoundingClientRect")
	return browserCellCoordinates(
		clientX, clientY, rect.Get("left").Float(), rect.Get("top").Float(),
		h.surface.Get("scrollLeft").Float(), h.surface.Get("scrollTop").Float(),
		h.columns(), h.rows(), h.cellWidth, h.lineHeight, h.padLeft, h.padTop,
	)
}

func (h *browserHost) render(frame Frame) {
	fragment := h.doc.Call("createDocumentFragment")
	lines := browserSpans(frame.Content)
	for y, line := range lines {
		for _, span := range line {
			fragment.Call("appendChild", h.spanNode(span))
		}
		if y < len(lines)-1 {
			fragment.Call("appendChild", h.doc.Call("createTextNode", "\n"))
		}
	}
	h.content.Call("replaceChildren", fragment)
	title := frame.Title
	if title == "" {
		title = h.origTitle
	}
	h.doc.Set("title", title)
	h.placeCursor(frame.Cursor)
}

func (h *browserHost) spanNode(span browserSpan) js.Value {
	tag := "span"
	if span.Link != "" {
		tag = "a"
	}
	node := h.doc.Call("createElement", tag)
	node.Set("textContent", span.Text)
	if span.Link != "" {
		node.Set("href", span.Link)
		node.Set("rel", "noopener noreferrer")
	}
	style := node.Get("style")
	fg, bg := span.Foreground, span.Background
	if span.Reverse {
		computed := js.Global().Call("getComputedStyle", h.surface)
		if fg == "" {
			fg = computed.Get("color").String()
		}
		if bg == "" {
			bg = computed.Get("backgroundColor").String()
		}
		fg, bg = bg, fg
	}
	if fg != "" {
		style.Set("color", fg)
	}
	if bg != "" {
		style.Set("backgroundColor", bg)
	}
	if span.Bold {
		style.Set("fontWeight", "bold")
	}
	if span.Faint {
		style.Set("opacity", "0.6")
	}
	if span.Italic {
		style.Set("fontStyle", "italic")
	}
	decorations := make([]string, 0, 2)
	if span.Underline != "" {
		decorations = append(decorations, "underline")
		style.Set("textDecorationStyle", span.Underline)
		if span.UnderlineColor != "" {
			style.Set("textDecorationColor", span.UnderlineColor)
		}
	}
	if span.Strikethrough {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		style.Set("textDecorationLine", strings.Join(decorations, " "))
	}
	if span.Blink {
		style.Set("animation", "flatte-blink 1s steps(1,end) infinite")
	}
	if span.Conceal {
		style.Set("visibility", "hidden")
	}
	if span.Cells > 1 && len([]rune(span.Text)) == 1 {
		style.Set("display", "inline-block")
		style.Set("width", fmt.Sprintf("%dch", span.Cells))
	}
	return node
}

func (h *browserHost) placeCursor(cursor *Cursor) {
	if cursor == nil {
		h.cursor.Get("style").Set("display", "none")
		return
	}
	lineHeight, charWidth := h.lineHeight, h.cellWidth
	left := h.padding("left") + float64(cursor.X)*charWidth
	top := h.padding("top") + float64(cursor.Y)*lineHeight
	for _, el := range []js.Value{h.cursor, h.input} {
		style := el.Get("style")
		style.Set("left", fmt.Sprintf("%.2fpx", left))
		style.Set("top", fmt.Sprintf("%.2fpx", top))
		style.Set("height", fmt.Sprintf("%.2fpx", lineHeight))
	}
	style := h.cursor.Get("style")
	style.Set("display", "block")
	style.Set("backgroundColor", browserCursorColor(cursor, js.Global().Call("getComputedStyle", h.surface).Get("color").String()))
	style.Set("animation", "")
	style.Set("width", fmt.Sprintf("%.2fpx", charWidth))
	style.Set("height", fmt.Sprintf("%.2fpx", lineHeight))
	switch cursorShape(cursor) {
	case CursorShapeBar:
		style.Set("width", "2px")
	case CursorShapeUnderline:
		style.Set("top", fmt.Sprintf("%.2fpx", top+lineHeight-2))
		style.Set("height", "2px")
	}
	if cursor.Style != nil && cursor.Style.Blink {
		style.Set("animation", "flatte-blink 1s steps(1,end) infinite")
	}
	h.input.Get("style").Set("width", fmt.Sprintf("%.2fpx", max(charWidth, 1)))
}

func cursorShape(cursor *Cursor) CursorShape {
	if cursor != nil && cursor.Style != nil {
		return cursor.Style.Shape
	}
	return CursorShapeDefault
}

func browserCursorColor(cursor *Cursor, fallback string) string {
	if cursor != nil && cursor.Style != nil {
		if color := browserColor(cursor.Style.Color); color != "" {
			return color
		}
	}
	return fallback
}
