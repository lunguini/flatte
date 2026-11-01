//go:build js && wasm

package window

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"strconv"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf16"

	"github.com/lunguini/flatte"
)

// Options selects the canvas host in a browser page.
func (o Options) ConfigureRun(config *flatte.RunConfig) {
	config.UseHost(canvasWindowHost{options: o})
}

func Supported() bool { return true }

type canvasWindowHost struct{ options Options }

func (h canvasWindowHost) Run(ctx context.Context, session *flatte.Session) error {
	return runCanvas(ctx, session, h.options)
}

func snapshot[S any](flatte.App[S], Options, float64) (image.Image, error) {
	return nil, ErrUnsupported
}

type canvasListener struct {
	target js.Value
	event  string
	fn     js.Func
}

type canvasPage struct {
	doc, mount, canvas, input, ctx, win js.Value
	host                                *host
	painter                             *canvasPainter
	options                             Options
	listeners                           []canvasListener
	observer                            js.Value
	observerFn                          js.Func
	mediaQuery                          js.Value
	mediaFn                             js.Func
	frameID                             js.Value
	frameFn                             js.Func
	frameCancel                         string
	framePending                        bool
	closed                              bool
	composing                           bool
	marked                              string
	caretOn                             bool
	blinkID                             js.Value
	blinkFn                             js.Func
	blinkPending                        bool
	wheel                               wheelAccumulator
	originalTitle                       string
	title                               string
	style                               js.Value
	styles                              map[string]string
	stop                                func()
}

func runCanvas(ctx context.Context, session *flatte.Session, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return errors.New("window: browser document is unavailable")
	}
	mount := doc.Call("querySelector", "[data-flatte]")
	if mount.IsUndefined() || mount.IsNull() {
		return errors.New("window: browser mount element [data-flatte] not found")
	}
	win := js.Global().Get("window")
	if win.IsUndefined() || win.IsNull() {
		win = js.Global()
	}
	th := themeFor(opts.Theme, browserDarkMode(win))
	if err := loadCanvasFonts(doc, th, opts.Fonts); err != nil {
		return err
	}

	canvas := doc.Call("createElement", "canvas")
	canvas.Set("ariaLabel", "Flatte graphical interface")
	canvas.Set("tabIndex", 0)
	canvas.Set("data-flatte-canvas", "")
	canvasStyle := canvas.Get("style")
	canvasStyle.Set("display", "block")
	canvasStyle.Set("width", "100%")
	canvasStyle.Set("height", "100%")
	canvasStyle.Set("touchAction", "none")
	input := doc.Call("createElement", "textarea")
	input.Set("ariaLabel", "Flatte application input")
	input.Set("rows", 1)
	input.Set("spellcheck", false)
	inputStyle := input.Get("style")
	for key, value := range map[string]string{
		"position": "absolute", "left": "-10000px", "top": "0", "width": "1px", "height": "1px",
		"padding": "0", "border": "0", "margin": "0", "resize": "none", "overflow": "hidden",
		"background": "transparent", "color": "transparent", "caretColor": "transparent", "opacity": "0",
	} {
		inputStyle.Set(key, value)
	}
	mountStyle := mount.Get("style")
	page := &canvasPage{
		doc: doc, mount: mount, canvas: canvas, input: input, win: win,
		options: opts, originalTitle: doc.Get("title").String(), title: opts.Title,
		caretOn: true, style: mountStyle, styles: map[string]string{},
	}
	for _, key := range []string{"position", "padding", "overflow", "whiteSpace", "background"} {
		page.styles[key] = mountStyle.Get(key).String()
	}
	if js.Global().Call("getComputedStyle", mount).Get("position").String() == "static" {
		mountStyle.Set("position", "relative")
	}
	mountStyle.Set("padding", "0")
	mountStyle.Set("overflow", "hidden")
	mountStyle.Set("whiteSpace", "normal")
	mountStyle.Set("background", "transparent")
	mount.Set("tabIndex", 0)
	mount.Call("replaceChildren", canvas, input)

	ctx2d := canvas.Call("getContext", "2d")
	if ctx2d.IsNull() || ctx2d.IsUndefined() {
		page.restore()
		return errors.New("window: browser canvas 2D context is unavailable")
	}
	th = themeFor(opts.Theme, browserDarkMode(win))
	page.ctx = ctx2d
	painter := newCanvasPainter(ctx2d, th)
	page.painter = painter
	painter.invalidate = page.invalidate
	page.originalTitle = doc.Get("title").String()
	page.title = opts.Title

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	var stopOnce bool
	page.stop = func() {
		if stopOnce {
			return
		}
		stopOnce = true
		close(done)
	}
	post := func(f func()) {
		var fn js.Func
		fn = js.FuncOf(func(js.Value, []js.Value) any {
			fn.Release()
			if !page.closed {
				f()
			}
			return nil
		})
		js.Global().Call("setTimeout", fn, 0)
	}
	h := newSessionHost(runCtx, session, painter.metrics, post, page.stop, page.invalidate)
	h.page = true
	h.text = canvasTextLayout{ctx: ctx2d, theme: th}
	h.clip = canvasClipboard{win: win}
	h.notes = canvasNotifier{win: win}
	h.dialogs = canvasDialogs{win: win}
	page.host = h
	h.surface.clock = time.Now
	page.bind()
	page.observeSize()
	page.setSize(false)
	h.start(page.width(), page.height())
	page.input.Call("focus", map[string]any{"preventScroll": true})
	page.invalidate()

	select {
	case <-runCtx.Done():
	case <-done:
	}
	page.close()
	h.close()
	return nil
}

func browserDarkMode(win js.Value) bool {
	match := win.Get("matchMedia")
	if match.Type() != js.TypeFunction {
		return true
	}
	return win.Call("matchMedia", "(prefers-color-scheme: dark)").Get("matches").Bool()
}

func (p *canvasPage) width() float64 {
	if w := p.canvas.Get("clientWidth").Float(); w > 0 {
		return w
	}
	if w := p.mount.Get("clientWidth").Float(); w > 0 {
		return w
	}
	return 640
}

func (p *canvasPage) height() float64 {
	if h := p.canvas.Get("clientHeight").Float(); h > 0 {
		return h
	}
	if h := p.mount.Get("clientHeight").Float(); h > 0 {
		return h
	}
	return 400
}

func (p *canvasPage) listen(target js.Value, event string, fn func(js.Value)) {
	cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if !p.closed && len(args) > 0 {
			fn(args[0])
		}
		return nil
	})
	target.Call("addEventListener", event, cb)
	p.listeners = append(p.listeners, canvasListener{target: target, event: event, fn: cb})
}

func (p *canvasPage) bind() {
	p.listen(p.canvas, "pointerdown", func(ev js.Value) {
		p.input.Call("focus", map[string]any{"preventScroll": true})
		if p.canvas.Get("setPointerCapture").Type() == js.TypeFunction {
			p.canvas.Call("setPointerCapture", ev.Get("pointerId"))
		}
		x, y := p.point(ev)
		p.host.surface.mouse(x, y, canvasMouseButton(ev.Get("button").Int()), flatte.MousePress, canvasModifiers(ev))
		ev.Call("preventDefault")
	})
	p.listen(p.canvas, "pointerup", func(ev js.Value) {
		x, y := p.point(ev)
		p.host.surface.mouse(x, y, canvasMouseButton(ev.Get("button").Int()), flatte.MouseRelease, canvasModifiers(ev))
	})
	p.listen(p.canvas, "pointermove", func(ev js.Value) {
		x, y := p.point(ev)
		button := canvasMouseButton(ev.Get("button").Int())
		if ev.Get("buttons").Int() == 0 {
			button = flatte.MouseNone
		}
		p.host.surface.mouse(x, y, button, flatte.MouseMotion, canvasModifiers(ev))
		pointer, tip := p.host.surface.hover(x, y)
		p.canvas.Get("style").Set("cursor", canvasCursor(pointer))
		p.canvas.Set("title", tip)
	})
	p.listen(p.canvas, "pointerleave", func(ev js.Value) {
		p.canvas.Get("style").Set("cursor", "default")
		p.canvas.Set("title", "")
		p.host.surface.hover(-1, -1)
	})
	p.listen(p.canvas, "wheel", func(ev js.Value) {
		ev.Call("preventDefault")
		delta := ev.Get("deltaY").Float()
		switch ev.Get("deltaMode").Int() {
		case 1:
			delta *= p.host.metrics.Line
		case 2:
			delta *= p.height()
		}
		n := p.wheel.notches(-delta, p.host.metrics.Line)
		if n == 0 {
			return
		}
		button := flatte.MouseWheelUp
		if n < 0 {
			button, n = flatte.MouseWheelDown, -n
		}
		x, y := p.point(ev)
		p.host.surface.wheel(x, y, button, n, canvasModifiers(ev))
	})
	p.listen(p.canvas, "dragover", func(ev js.Value) { ev.Call("preventDefault") })
	p.listen(p.canvas, "drop", func(ev js.Value) {
		ev.Call("preventDefault")
		dt := ev.Get("dataTransfer")
		files := dt.Get("files")
		paths := make([]string, 0, files.Get("length").Int())
		for i := 0; i < files.Get("length").Int(); i++ {
			paths = append(paths, files.Index(i).Get("name").String())
		}
		x, y := p.point(ev)
		p.host.surface.drop(x, y, paths, dt.Call("getData", "text/plain").String())
	})
	p.listen(p.input, "keydown", p.keyDown)
	p.listen(p.input, "input", func(js.Value) {
		if p.composing {
			return
		}
		p.sendText(p.input.Get("value").String())
		p.input.Set("value", "")
	})
	p.listen(p.input, "compositionstart", func(js.Value) { p.composing = true })
	p.listen(p.input, "compositionupdate", func(ev js.Value) {
		p.marked = ev.Get("data").String()
		p.invalidate()
	})
	p.listen(p.input, "compositionend", func(ev js.Value) {
		p.composing, p.marked = false, ""
		text := ev.Get("data").String()
		if text == "" {
			text = p.input.Get("value").String()
		}
		p.sendText(text)
		p.input.Set("value", "")
		p.invalidate()
	})
	p.listen(p.input, "paste", func(ev js.Value) {
		data := ev.Get("clipboardData")
		if data.IsNull() || data.IsUndefined() {
			return
		}
		text := data.Call("getData", "text").String()
		if text != "" {
			ev.Call("preventDefault")
			p.host.handle(flatte.PasteEvent{Text: text})
			p.input.Set("value", "")
		}
	})
	p.listen(p.input, "focus", func(js.Value) {
		p.host.activated(true)
		p.caretOn = true
		p.scheduleBlink()
	})
	p.listen(p.input, "blur", func(js.Value) {
		p.host.activated(false)
		p.cancelBlink()
		p.invalidate()
	})
	p.listen(p.mount, "focus", func(js.Value) { p.input.Call("focus", map[string]any{"preventScroll": true}) })
	p.listen(p.win, "resize", func(js.Value) { p.setSize(true) })
	if p.win.Get("matchMedia").Type() == js.TypeFunction && p.options.Theme == nil {
		p.mediaQuery = p.win.Call("matchMedia", "(prefers-color-scheme: dark)")
		p.mediaFn = js.FuncOf(func(js.Value, []js.Value) any {
			p.refreshTheme()
			return nil
		})
		p.mediaQuery.Call("addEventListener", "change", p.mediaFn)
	}
}

func (p *canvasPage) keyDown(ev js.Value) {
	if p.composing || ev.Get("isComposing").Truthy() || ev.Get("keyCode").Int() == 229 {
		return
	}
	name := ev.Get("key").String()
	mods := canvasModifiers(ev)
	if canvasPasteGesture(name, mods, ev) {
		return
	}
	if key, ok := canvasKeyEvent(name, mods, ev); ok {
		ev.Call("preventDefault")
		p.host.surface.key(key)
	}
}

func (p *canvasPage) sendText(text string) {
	if text == "" {
		return
	}
	events := make([]flatte.Event, 0, len([]rune(text)))
	runes := []rune(text)
	for i, r := range runes {
		if r == '\r' || r == '\n' {
			events = append(events, flatte.KeyEvent{Key: flatte.KeyEnter})
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				runes[i+1] = 0
			}
			continue
		}
		if r != 0 {
			events = append(events, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r})
		}
	}
	if len(events) > 0 {
		for _, event := range events {
			p.host.handle(event)
		}
	}
}

func (p *canvasPage) point(ev js.Value) (float64, float64) {
	r := p.canvas.Call("getBoundingClientRect")
	return ev.Get("clientX").Float() - r.Get("left").Float(), ev.Get("clientY").Float() - r.Get("top").Float()
}

func (p *canvasPage) observeSize() {
	ctor := js.Global().Get("ResizeObserver")
	if ctor.Type() != js.TypeFunction {
		return
	}
	p.observerFn = js.FuncOf(func(js.Value, []js.Value) any {
		p.setSize(true)
		return nil
	})
	p.observer = ctor.New(p.observerFn)
	p.observer.Call("observe", p.mount)
}

func (p *canvasPage) setSize(notify bool) {
	if p.closed || p.canvas.IsUndefined() || p.canvas.IsNull() {
		return
	}
	w, h := p.mount.Get("clientWidth").Float(), p.mount.Get("clientHeight").Float()
	if w <= 0 || h <= 0 {
		r := p.mount.Call("getBoundingClientRect")
		w, h = r.Get("width").Float(), r.Get("height").Float()
	}
	if w <= 0 {
		w = 640
	}
	if h <= 0 {
		h = 400
	}
	dpr := p.win.Get("devicePixelRatio").Float()
	if dpr < 1 {
		dpr = 1
	}
	p.canvas.Set("width", int(w*dpr+0.5))
	p.canvas.Set("height", int(h*dpr+0.5))
	p.canvas.Get("style").Set("width", fmt.Sprintf("%.2fpx", w))
	p.canvas.Get("style").Set("height", fmt.Sprintf("%.2fpx", h))
	p.ctx.Call("setTransform", dpr, 0, 0, dpr, 0, 0)
	p.painter.scale = dpr
	if notify && p.host != nil {
		p.host.surface.resize(w, h)
	}
	p.invalidate()
}

func (p *canvasPage) refreshTheme() {
	if p.closed {
		return
	}
	th := themeFor(p.options.Theme, browserDarkMode(p.win))
	ctx := p.ctx
	p.painter.close()
	p.painter = newCanvasPainter(ctx, th)
	p.painter.invalidate = p.invalidate
	p.host.metrics = p.painter.metrics
	p.host.text = canvasTextLayout{ctx: ctx, theme: th}
	p.host.surface.delivered = nil
	p.host.surface.deliveredPoints = nil
	p.host.handle(flatte.ResizeEvent{Width: p.host.cols(p.width()), Height: p.host.lines(p.height())})
}

func (p *canvasPage) invalidate() {
	if p.closed || p.framePending {
		return
	}
	request := p.win.Get("requestAnimationFrame")
	if request.Type() != js.TypeFunction {
		request, p.frameCancel = p.win.Get("setTimeout"), "clearTimeout"
		p.framePending = true
		p.frameFn = js.FuncOf(func(js.Value, []js.Value) any {
			p.framePending = false
			p.frameFn.Release()
			p.draw()
			return nil
		})
		p.frameID = request.Invoke(p.frameFn, 16)
		return
	}
	p.frameCancel = "cancelAnimationFrame"
	p.framePending = true
	p.frameFn = js.FuncOf(func(js.Value, []js.Value) any {
		p.framePending = false
		p.frameFn.Release()
		p.draw()
		return nil
	})
	p.frameID = request.Invoke(p.frameFn)
}

func (p *canvasPage) draw() {
	if p.closed || p.host == nil {
		return
	}
	scene := p.host.surface.render()
	p.painter.marked = p.marked
	p.painter.caretOn = p.caretOn
	p.painter.draw(scene, p.width(), p.height(), p.host.surface.presentation())
	if cursor := p.host.surface.frameCursor(); cursor != nil {
		p.painter.drawCursor(*cursor)
	}
	title := p.host.surface.frameTitle()
	if title == "" {
		title = p.title
	}
	if title == "" {
		title = p.originalTitle
	}
	p.doc.Set("title", title)
	if p.painter.animated {
		p.painter.animated = false
		var fn js.Func
		fn = js.FuncOf(func(js.Value, []js.Value) any { fn.Release(); p.invalidate(); return nil })
		p.win.Call("setTimeout", fn, 80)
	}
}

func (p *canvasPage) scheduleBlink() {
	if p.closed || p.blinkPending {
		return
	}
	p.blinkPending = true
	p.blinkFn = js.FuncOf(func(js.Value, []js.Value) any {
		p.blinkPending = false
		p.blinkFn.Release()
		if p.closed {
			return nil
		}
		p.caretOn = !p.caretOn
		p.invalidate()
		p.scheduleBlink()
		return nil
	})
	p.blinkID = p.win.Call("setTimeout", p.blinkFn, 500)
}

func (p *canvasPage) cancelBlink() {
	if !p.blinkPending {
		return
	}
	if p.win.Get("clearTimeout").Type() == js.TypeFunction {
		p.win.Call("clearTimeout", p.blinkID)
	}
	p.blinkFn.Release()
	p.blinkPending = false
}

func (p *canvasPage) close() {
	if p.closed {
		return
	}
	p.closed = true
	if p.framePending {
		if cancel := p.win.Get(p.frameCancel); cancel.Type() == js.TypeFunction {
			cancel.Invoke(p.frameID)
		}
		p.frameFn.Release()
	}
	p.cancelBlink()
	if !p.observer.IsUndefined() && !p.observer.IsNull() {
		p.observer.Call("disconnect")
		p.observerFn.Release()
	}
	if !p.mediaQuery.IsUndefined() && !p.mediaQuery.IsNull() && p.mediaFn.Value.Type() == js.TypeFunction {
		p.mediaQuery.Call("removeEventListener", "change", p.mediaFn)
		p.mediaFn.Release()
	}
	for i := len(p.listeners) - 1; i >= 0; i-- {
		l := p.listeners[i]
		l.target.Call("removeEventListener", l.event, l.fn)
		l.fn.Release()
	}
	if p.painter != nil {
		p.painter.close()
	}
	p.canvas.Call("remove")
	p.input.Call("remove")
	p.restore()
	if p.doc.Get("title").String() != p.originalTitle {
		p.doc.Set("title", p.originalTitle)
	}
}

func (p *canvasPage) restore() {
	for key, value := range p.styles {
		p.style.Set(key, value)
	}
}

func canvasModifiers(ev js.Value) flatte.Mod {
	var mod flatte.Mod
	if ev.Get("shiftKey").Bool() {
		mod |= flatte.ModShift
	}
	if ev.Get("altKey").Bool() {
		mod |= flatte.ModAlt
	}
	if ev.Get("ctrlKey").Bool() {
		mod |= flatte.ModCtrl
	}
	if ev.Get("metaKey").Bool() {
		mod |= flatte.ModSuper
	}
	return mod
}

func canvasMouseButton(button int) flatte.MouseButton {
	switch button {
	case 0:
		return flatte.MouseLeft
	case 1:
		return flatte.MouseMiddle
	case 2:
		return flatte.MouseRight
	default:
		return flatte.MouseNone
	}
}

func canvasCursor(p pointer) string {
	switch p {
	case pointerHand:
		return "pointer"
	case pointerText:
		return "text"
	default:
		return "default"
	}
}

func canvasPasteGesture(name string, mod flatte.Mod, ev js.Value) bool {
	if ev.Get("getModifierState").Type() == js.TypeFunction && ev.Call("getModifierState", "AltGraph").Truthy() {
		return false
	}
	if mod&flatte.ModAlt != 0 || (name != "v" && name != "V") {
		return false
	}
	mods := mod &^ flatte.ModShift
	return mods == flatte.ModCtrl || mods == flatte.ModSuper
}

func canvasKeyEvent(name string, mod flatte.Mod, ev js.Value) (flatte.KeyEvent, bool) {
	var key flatte.Key
	switch name {
	case "ArrowUp":
		key = flatte.KeyUp
	case "ArrowDown":
		key = flatte.KeyDown
	case "ArrowLeft":
		key = flatte.KeyLeft
	case "ArrowRight":
		key = flatte.KeyRight
	case "Enter":
		key = flatte.KeyEnter
	case "Escape":
		key = flatte.KeyEscape
	case "Backspace":
		key = flatte.KeyBackspace
	case "Delete":
		key = flatte.KeyDelete
	case "Tab":
		key = flatte.KeyTab
	case "Home":
		key = flatte.KeyHome
	case "End":
		key = flatte.KeyEnd
	case "PageUp":
		key = flatte.KeyPageUp
	case "PageDown":
		key = flatte.KeyPageDown
	default:
		if len(name) >= 2 && name[0] == 'F' {
			n := 0
			for i := 1; i < len(name); i++ {
				if name[i] < '0' || name[i] > '9' {
					n = 0
					break
				}
				n = n*10 + int(name[i]-'0')
			}
			if n >= 1 && n <= 12 {
				key = flatte.Key(int(flatte.KeyF1) + n - 1)
			}
		}
	}
	if key != flatte.KeyUnknown {
		return flatte.KeyEvent{Key: key, Mod: mod}, true
	}
	runes := []rune(name)
	if len(runes) != 1 || mod&(flatte.ModCtrl|flatte.ModAlt|flatte.ModSuper) == 0 ||
		(ev.Get("getModifierState").Type() == js.TypeFunction && ev.Call("getModifierState", "AltGraph").Truthy()) {
		return flatte.KeyEvent{}, false
	}
	if mod&flatte.ModCtrl != 0 && mod&flatte.ModAlt == 0 && (runes[0] == 'c' || runes[0] == 'C') {
		return flatte.KeyEvent{Key: flatte.KeyCtrlC, Mod: mod}, true
	}
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: runes[0], Mod: mod}, true
}

type canvasClipboard struct{ win js.Value }

func (c canvasClipboard) set(text string) {
	clipboard := c.win.Get("navigator").Get("clipboard")
	if clipboard.IsUndefined() || clipboard.IsNull() || clipboard.Get("writeText").Type() != js.TypeFunction {
		return
	}
	promise := clipboard.Call("writeText", text)
	ignoreCanvasPromise(promise)
}

func (c canvasClipboard) get() string { return "" }

func (c canvasClipboard) getAsync(done func(string)) {
	clipboard := c.win.Get("navigator").Get("clipboard")
	if clipboard.IsUndefined() || clipboard.IsNull() || clipboard.Get("readText").Type() != js.TypeFunction {
		done("")
		return
	}
	var success, failure js.Func
	settled := false
	release := func() {
		if settled {
			return
		}
		settled = true
		success.Release()
		failure.Release()
	}
	success = js.FuncOf(func(_ js.Value, args []js.Value) any {
		text := ""
		if len(args) > 0 {
			text = args[0].String()
		}
		release()
		done(text)
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any { release(); done(""); return nil })
	clipboard.Call("readText").Call("then", success, failure)
}

type canvasNotifier struct{ win js.Value }

func (n canvasNotifier) notify(notification flatte.Notification) {
	ctor := n.win.Get("Notification")
	if ctor.IsUndefined() || ctor.IsNull() || ctor.Get("permission").String() != "granted" {
		return
	}
	ctor.New(notification.Title, map[string]any{"body": notification.Body})
}

type canvasDialogs struct{ win js.Value }

func (d canvasDialogs) run(dialog flatte.Dialog) flatte.DialogResult {
	if dialog.Alert == nil {
		return flatte.DialogResult{Err: flatte.ErrNoDialogs}
	}
	alert := *dialog.Alert
	buttons := alert.Buttons
	if len(buttons) == 0 {
		buttons = []string{"OK"}
	}
	message := alert.Message
	if alert.Title != "" {
		message = alert.Title + "\n\n" + message
	}
	if len(buttons) <= 1 {
		d.win.Call("alert", message)
		return flatte.DialogResult{Button: 0}
	}
	if len(buttons) == 2 {
		if d.win.Call("confirm", message+"\n\n"+buttons[0]+" / "+buttons[1]).Bool() {
			return flatte.DialogResult{Button: 0}
		}
		return flatte.DialogResult{Button: 1}
	}
	if d.win.Get("prompt").Type() == js.TypeFunction {
		choices := make([]string, len(buttons))
		for i, button := range buttons {
			choices[i] = fmt.Sprintf("%d: %s", i, button)
		}
		answer := d.win.Call("prompt", message+"\n\n"+strings.Join(choices, "\n"), "0")
		if answer.IsNull() || answer.IsUndefined() {
			return flatte.DialogResult{Button: -1}
		}
		button, err := strconv.Atoi(answer.String())
		if err != nil || button < 0 || button >= len(buttons) {
			return flatte.DialogResult{Button: -1}
		}
		return flatte.DialogResult{Button: button}
	}
	d.win.Call("alert", message)
	return flatte.DialogResult{Button: 0}
}

func ignoreCanvasPromise(promise js.Value) {
	if promise.IsUndefined() || promise.IsNull() || promise.Get("then").Type() != js.TypeFunction {
		return
	}
	var success, failure js.Func
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		success.Release()
		failure.Release()
	}
	success = js.FuncOf(func(js.Value, []js.Value) any { release(); return nil })
	failure = js.FuncOf(func(js.Value, []js.Value) any { release(); return nil })
	promise.Call("then", success, failure)
}

func loadCanvasFonts(doc js.Value, th Theme, files [][]byte) error {
	if len(files) == 0 {
		return nil
	}
	ctor := js.Global().Get("FontFace")
	if ctor.Type() != js.TypeFunction {
		return errors.New("window: FontFace is unavailable")
	}
	fonts := doc.Get("fonts")
	for i, data := range files {
		family, weight, style := canvasFontInfo(data)
		if family == "" {
			family = th.Font
		}
		if family == "" {
			family = fmt.Sprintf("Flatte Font %d", i)
		}
		if weight == "" {
			if i == 1 {
				weight = "700"
			} else {
				weight = "400"
			}
		}
		array := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(array, data)
		face := ctor.New(family, array, map[string]any{"weight": weight, "style": style})
		loaded, err := awaitCanvasPromise(face.Call("load"))
		if err != nil {
			return fmt.Errorf("window: load font %q: %w", family, err)
		}
		fonts.Call("add", loaded)
	}
	return nil
}

func awaitCanvasPromise(promise js.Value) (js.Value, error) {
	type result struct {
		value js.Value
		err   error
	}
	done := make(chan result, 1)
	var success, failure js.Func
	settled := false
	release := func() {
		if settled {
			return
		}
		settled = true
		success.Release()
		failure.Release()
	}
	success = js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		release()
		done <- result{value: value}
		return nil
	})
	failure = js.FuncOf(func(_ js.Value, args []js.Value) any {
		err := errors.New("font loading failed")
		if len(args) > 0 {
			err = errors.New(args[0].String())
		}
		release()
		done <- result{err: err}
		return nil
	})
	promise.Call("then", success, failure)
	r := <-done
	return r.value, r.err
}

func canvasFontInfo(data []byte) (family, weight, style string) {
	if len(data) < 12 {
		return
	}
	tables := int(binary.BigEndian.Uint16(data[4:6]))
	var name []byte
	for i := 0; i < tables; i++ {
		at := 12 + i*16
		if at+16 > len(data) {
			return
		}
		if string(data[at:at+4]) == "name" {
			offset := int(binary.BigEndian.Uint32(data[at+8 : at+12]))
			length := int(binary.BigEndian.Uint32(data[at+12 : at+16]))
			if offset >= 0 && length >= 6 && offset+length <= len(data) {
				name = data[offset : offset+length]
			}
			break
		}
	}
	if len(name) < 6 {
		return
	}
	count, stringsAt := int(binary.BigEndian.Uint16(name[2:4])), int(binary.BigEndian.Uint16(name[4:6]))
	var familyFallback, subFallback string
	for i := 0; i < count; i++ {
		at := 6 + i*12
		if at+12 > len(name) {
			break
		}
		platform, nameID := binary.BigEndian.Uint16(name[at:at+2]), binary.BigEndian.Uint16(name[at+6:at+8])
		length, offset := int(binary.BigEndian.Uint16(name[at+8:at+10])), int(binary.BigEndian.Uint16(name[at+10:at+12]))
		if nameID != 1 && nameID != 2 && nameID != 16 && nameID != 17 {
			continue
		}
		start, end := stringsAt+offset, stringsAt+offset+length
		if start < 0 || end > len(name) {
			continue
		}
		value := string(name[start:end])
		if platform == 0 || platform == 3 {
			units := make([]uint16, length/2)
			for j := range units {
				units[j] = binary.BigEndian.Uint16(name[start+j*2 : start+j*2+2])
			}
			value = string(utf16.Decode(units))
		}
		value = strings.TrimSpace(value)
		switch nameID {
		case 16:
			family = value
		case 1:
			familyFallback = value
		case 17:
			subFallback = value
		case 2:
			if subFallback == "" {
				subFallback = value
			}
		}
	}
	if family == "" {
		family = familyFallback
	}
	lower := strings.ToLower(subFallback)
	if strings.Contains(lower, "bold") || strings.Contains(lower, "semibold") {
		weight = "700"
	}
	if strings.Contains(lower, "italic") || strings.Contains(lower, "oblique") {
		style = "italic"
	} else {
		style = "normal"
	}
	return
}
