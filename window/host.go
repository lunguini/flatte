package window

import (
	"context"
	"maps"
	"math"
	"reflect"
	"slices"

	"github.com/charmbracelet/colorprofile"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// maxLayoutPasses bounds how many LayoutEvents one render delivers, as in
// the terminal host (flatte.SettleFrame).
const maxLayoutPasses = 2

// host drives an App from window events: everything about running an app in
// windows that a platform backend does not need to know. It runs on the UI
// thread only; async results arrive through post. Its embedded surface is
// the main window; App.Windows declares the others.
type host[S any] struct {
	*surface[S]

	app     flatte.App[S]
	fx      flatte.Effects[S]
	metrics layout.Metrics
	cancel  context.CancelFunc

	// stop ends the platform run loop. It is called on the UI thread.
	stop     func()
	quitting bool

	post func(func())
	// text and clip are the backend's text layout and clipboard (nil: no
	// carets in events; clipboard effects do nothing).
	text textLayout
	clip clipboard
	// menus shows native menus (nil: none); bar is the menu bar it shows.
	menus menuBackend
	bar   []flatte.Menu
	// dialogs shows native dialogs (nil: every dialog answers as
	// cancelled).
	dialogs dialogBackend
	// windows opens and closes the secondary windows (nil: none open);
	// open are the ones open, by ID; focus is the focused window's ID ("" the
	// main window).
	windows windowBackend
	open    map[string]*surface[S]
	focus   string
}

// surface is one window's share of the host: its view's size, the scene
// drawn in it, and what the backend draws over it on its own.
type surface[S any] struct {
	h *host[S]
	// id is the window's ID ("" the main window); invalidate requests a
	// redraw of it, on the UI thread.
	id         string
	invalidate func()

	width, height float64 // view size in points
	scene         layout.Scene
	title         string
	// cursor is a string frame's Frame.Cursor, in cells; Root frames show the
	// focused Input's caret instead.
	cursor    *flatte.Cursor
	delivered map[string]layout.Rect
	// dirty marks the scene stale: the app changed since it was rendered,
	// so carets must be read from a fresh one.
	dirty bool

	// drag is the ID of the Input a left-button drag began on: pointer
	// events report carets for it until the button is released.
	drag string
	// goal is the x a run of vertical caret moves keeps, for the Input goalID
	// whose caret the last move left at goalCaret.
	goal      float64
	goalID    string
	goalCaret int
	// pres is what the backend draws on its own and never tells the app:
	// the control under the pointer and the one the left button went down
	// on, and text selected in a Selectable Label. selecting is a drag
	// extending the selection.
	pres      presentation
	selecting bool
}

// dialogBackend is a backend's native dialogs.
type dialogBackend interface {
	// run shows d modally and returns the user's answer.
	run(d flatte.Dialog) flatte.DialogResult
}

// menuBackend is a backend's native menus.
type menuBackend interface {
	// setBar shows menus in the menu bar.
	setBar(menus []flatte.Menu)
	// popUp shows items as a context menu at (x, y) points in window id
	// until the user chooses or dismisses it, and returns the chosen item's
	// ID ("": none).
	popUp(id string, items []flatte.MenuItem, x, y float64) string
}

// ui is what a window's view needs from the generic host: one window's
// surface.
type ui interface {
	resize(width, height float64)
	key(flatte.KeyEvent)
	paste()
	hover(x, y float64) (pointer, string)
	presentation() presentation
	drop(x, y float64, paths []string, text string)
	mouse(x, y float64, button flatte.MouseButton, action flatte.MouseAction, mod flatte.Mod)
	// hit is the ID of the node at (x, y) in a fresh scene ("": none).
	hit(x, y float64) string
	wheel(x, y float64, button flatte.MouseButton, notches int, mod flatte.Mod)
	render() layout.Scene
	frameTitle() string
	frameCursor() *flatte.Cursor
}

// windowBackend opens and closes a backend's secondary windows.
type windowBackend interface {
	// openWindow shows w, drawn from s, and returns the function that
	// redraws it.
	openWindow(w flatte.Window, s ui) (invalidate func())
	// closeWindow closes window id.
	closeWindow(id string)
}

// presentation is what a backend draws on its own.
type presentation struct {
	hovered, pressed string
	sel              textSel
}

// textSel is the bytes from..to (either order) selected in the text of the
// Selectable TextOp at index op of the scene — while that op still has the
// same text.
type textSel struct {
	op       int
	text     string
	from, to int
	ok       bool // a selection exists
}

// in is the selection's bytes in the op at index i, from < to; empty
// (0, 0) when the selection is not in it.
func (s textSel) in(i int, op layout.TextOp) (int, int) {
	if !s.ok || i != s.op || op.Text != s.text {
		return 0, 0
	}
	return min(s.from, s.to), max(s.from, s.to)
}

// pointer is the pointer shape a backend shows.
type pointer int

const (
	pointerArrow pointer = iota
	pointerHand          // over a control
	pointerText          // over a text field
)

// textLayout is where a backend puts carets in the text fields it draws.
type textLayout interface {
	// caretAt is the byte offset into op.Value nearest (x, y) points,
	// clamped into the field.
	caretAt(op layout.InputOp, x, y float64) int
	// caretX is the x of caret within its visual row, from the row's start.
	caretX(op layout.InputOp, caret int) float64
	// caretStep is the caret one visual row up (dir < 0) or down from
	// caret, nearest x within that row; caret itself on the first or last
	// row.
	caretStep(op layout.InputOp, caret, dir int, x float64) int
	// textOffset is the byte offset into op.Text nearest (x, y) points,
	// clamped into the text.
	textOffset(op layout.TextOp, x, y float64) int
}

// clipboard is the system clipboard.
type clipboard interface {
	get() string
	set(text string)
}

// newHost wires app to a backend. post runs a function on the UI thread
// (from any goroutine); async updates reach the app through it; invalidate
// redraws the main window.
func newHost[S any](ctx context.Context, app flatte.App[S], m layout.Metrics, post func(func()), stop, invalidate func()) *host[S] {
	if app.Tracer == nil {
		app.Tracer = flatte.NoopTracer{}
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &host[S]{app: app, metrics: m, cancel: cancel, stop: stop, post: post, open: map[string]*surface[S]{}}
	h.surface = &surface[S]{h: h, invalidate: invalidate}
	updates := make(chan flatte.StateUpdate[S], 256)
	h.fx = flatte.WithHostClipboard(flatte.NewHarnessEffects(ctx, updates, func() { h.quitting = true }, nil, nil),
		func(text string) {
			if h.clip != nil {
				h.clip.set(text)
			}
		},
		func() {
			if h.clip != nil {
				text := h.clip.get()
				// Answered after the handler returns, like a terminal's reply.
				h.post(func() { h.handle(flatte.ClipboardEvent{Text: text}) })
			}
		})
	// Dialogs run modally after the handler returns, never inside it.
	h.fx = flatte.WithHostDialogs(h.fx, func(d flatte.Dialog, done func(flatte.DialogResult)) {
		h.post(func() {
			if h.dialogs == nil {
				done(flatte.DialogResult{Button: -1})
				return
			}
			done(h.dialogs.run(d))
		})
	})
	go func() {
		for {
			select {
			case u := <-updates:
				post(func() { h.apply(u) })
			case <-ctx.Done():
				return
			}
		}
	}()
	return h
}

// start runs Init and delivers the first ResizeEvent.
func (h *host[S]) start(width, height float64) {
	if h.app.Init != nil {
		h.app.Init(h.app.State, h.fx)
	}
	h.after()
	h.resize(width, height)
}

// close closes the secondary windows, releases the host's goroutines, and
// runs OnExit.
func (h *host[S]) close() {
	for id := range h.open {
		if h.windows != nil {
			h.windows.closeWindow(id)
		}
		delete(h.open, id)
	}
	h.cancel()
	if h.app.OnExit != nil {
		h.app.OnExit(h.app.State)
	}
}

func (h *host[S]) apply(u flatte.StateUpdate[S]) {
	flatte.ApplyUpdate(h.app.State, h.app.Tracer, u)
	h.after()
}

// handle delivers ev to the app.
func (h *host[S]) handle(ev flatte.Event) {
	h.app.Tracer.Event(ev)
	if h.app.Handle != nil {
		h.app.Handle(h.app.State, ev, h.fx)
	}
	h.after()
}

// after reacts to a state change: stop on quit, else open and close windows
// as declared and redraw them all.
func (h *host[S]) after() {
	if h.quitting {
		h.stop()
		return
	}
	h.syncWindows()
	for _, s := range h.surfaces() {
		s.dirty = true
		s.invalidate()
	}
}

// surfaces is the main window's surface and the open windows'.
func (h *host[S]) surfaces() []*surface[S] {
	out := []*surface[S]{h.surface}
	for _, id := range slices.Sorted(maps.Keys(h.open)) {
		out = append(out, h.open[id])
	}
	return out
}

// syncWindows opens the windows the app declares that are not open, and
// closes the open ones it no longer declares.
func (h *host[S]) syncWindows() {
	if h.windows == nil {
		return
	}
	var declared []flatte.Window
	if h.app.Windows != nil {
		declared = h.app.Windows(h.app.State)
	}
	keep := map[string]bool{}
	for _, w := range declared {
		if w.ID == "" || keep[w.ID] {
			continue
		}
		keep[w.ID] = true
		if _, ok := h.open[w.ID]; ok {
			continue
		}
		s := &surface[S]{h: h, id: w.ID, dirty: true, invalidate: func() {}}
		h.open[w.ID] = s
		s.invalidate = h.windows.openWindow(w, s)
	}
	for id := range h.open {
		if !keep[id] {
			delete(h.open, id)
			h.windows.closeWindow(id)
			if h.focus == id {
				h.focus = "" // the backend reports the next key window
			}
		}
	}
}

// focused reports that window id ("" the main window) became the key
// window; the app hears of it when that changes the focus.
func (h *host[S]) focused(id string) {
	if id == h.focus {
		return
	}
	if _, ok := h.open[id]; id != "" && !ok {
		return
	}
	h.focus = id
	h.handle(flatte.WindowEvent{Window: id, Kind: flatte.WindowFocused})
}

// closeRequested reports the user asking to close window id.
func (h *host[S]) closeRequested(id string) {
	if _, ok := h.open[id]; ok {
		h.handle(flatte.WindowEvent{Window: id, Kind: flatte.WindowCloseRequest})
	}
}

// focusedSurface is the focused window's surface.
func (h *host[S]) focusedSurface() *surface[S] {
	if s, ok := h.open[h.focus]; ok {
		return s
	}
	return h.surface
}

func (h *host[S]) cols(points float64) int  { return cellsIn(points, h.metrics.Column) }
func (h *host[S]) lines(points float64) int { return cellsIn(points, h.metrics.Line) }

func cellsIn(points, unit float64) int {
	return layout.PointRect{W: points}.Cells(layout.Metrics{Column: unit}).W
}

// resize records the view size; the main window tells the app its size in
// cells (secondary windows' sizes reach it as LayoutEvents).
func (s *surface[S]) resize(width, height float64) {
	if width == s.width && height == s.height {
		return
	}
	s.width, s.height = width, height
	if s.id == "" {
		s.h.handle(flatte.ResizeEvent{Width: s.h.cols(width), Height: s.h.lines(height)})
		return
	}
	s.dirty = true
	s.invalidate()
}

// key delivers a key event. Up and Down (plain or with Shift) over a
// focused multi-line Input carry the caret one visual row away in the
// backend's layout, keeping the x a run of vertical moves started at.
func (s *surface[S]) key(ev flatte.KeyEvent) {
	h := s.h
	// ⌘C copies selected label text; with none it is the app's.
	if ev == copyKey && s.copySelection() {
		return
	}
	// A menu shortcut the menu bar did not take natively (one without ⌘,
	// or a disabled item's) is routed here, as a terminal routes it.
	if h.app.Menus != nil {
		if it, ok := flatte.MenuShortcut(h.app.Menus(h.app.State), ev); ok {
			if !it.Disabled {
				h.handle(flatte.MenuEvent{ID: it.ID})
			}
			return
		}
	}
	vertical := (ev.Key == flatte.KeyUp || ev.Key == flatte.KeyDown) && ev.Mod&^flatte.ModShift == 0
	if op, ok := s.focusedArea(); vertical && ok && h.text != nil {
		if op.ID != s.goalID || op.Caret != s.goalCaret {
			s.goal = h.text.caretX(op, op.Caret)
		}
		dir := 1
		if ev.Key == flatte.KeyUp {
			dir = -1
		}
		ev.Caret, ev.HasCaret = h.text.caretStep(op, op.Caret, dir, s.goal), true
		s.goalID, s.goalCaret = op.ID, ev.Caret
	}
	h.handle(ev)
}

// control is the control with id in the scene; ok is false when there is
// none or it is disabled.
func (s *surface[S]) control(id string) (layout.ControlOp, bool) {
	if id == "" {
		return layout.ControlOp{}, false
	}
	for i := len(s.scene.Ops) - 1; i >= 0; i-- {
		if c, ok := s.scene.Ops[i].(layout.ControlOp); ok && c.ID == id {
			return c, !c.Disabled
		}
	}
	return layout.ControlOp{}, false
}

// hover tracks the pointer at (x, y) points (outside the view: x < 0):
// it records the control under it, redrawing when that changes, and
// reports the pointer shape and tooltip to show there.
func (s *surface[S]) hover(x, y float64) (pointer, string) {
	scene := s.fresh()
	id, tooltip, shape := "", "", pointerArrow
	if x >= 0 && y >= 0 {
		id, _ = scene.HitTest(x, y)
		tooltip = scene.TooltipAt(x, y)
		if _, ok := s.input(id); ok {
			shape = pointerText
		}
	}
	if _, ok := s.control(id); ok {
		shape = pointerHand
	} else {
		id = ""
	}
	if shape == pointerArrow {
		if _, _, ok := s.selectableAt(x, y); ok {
			shape = pointerText
		}
	}
	if id != s.pres.hovered {
		s.pres.hovered = id
		s.invalidate()
	}
	return shape, tooltip
}

// syncMenus shows the app's menu bar when it changed.
func (h *host[S]) syncMenus() {
	if h.menus == nil || h.app.Menus == nil {
		return
	}
	if bar := h.app.Menus(h.app.State); !reflect.DeepEqual(bar, h.bar) {
		h.bar = bar
		h.menus.setBar(bar)
	}
}

// copyKey is ⌘C.
var copyKey = flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'c', Mod: flatte.ModSuper}

// copySelection copies the selected label text, reporting whether there
// was any.
func (s *surface[S]) copySelection() bool {
	if s.h.clip == nil {
		return false
	}
	text := s.selectedText()
	if text != "" {
		s.h.clip.set(text)
	}
	return text != ""
}

// menuChosen delivers the menu bar's choice of the item with id — except
// that the app's Copy (an item with the ⌘C shortcut) copies selected label
// text in the focused window when there is some, as Copy acts on the
// selection in native apps.
func (h *host[S]) menuChosen(id string) {
	if it, ok := findMenuItem(h.bar, id); ok && slices.Contains(it.Keys, copyKey) && h.focusedSurface().copySelection() {
		return
	}
	h.handle(flatte.MenuEvent{ID: id})
}

// findMenuItem is the command with id in menus.
func findMenuItem(menus []flatte.Menu, id string) (flatte.MenuItem, bool) {
	var find func(items []flatte.MenuItem) (flatte.MenuItem, bool)
	find = func(items []flatte.MenuItem) (flatte.MenuItem, bool) {
		for _, it := range items {
			if len(it.Items) > 0 {
				if sub, ok := find(it.Items); ok {
					return sub, true
				}
			} else if !it.Separator && it.ID == id {
				return it, true
			}
		}
		return flatte.MenuItem{}, false
	}
	for _, m := range menus {
		if it, ok := find(m.Items); ok {
			return it, true
		}
	}
	return flatte.MenuItem{}, false
}

// drop delivers files or text dropped at (x, y) points as a DropEvent,
// located as mouse events are.
func (s *surface[S]) drop(x, y float64, paths []string, text string) {
	h := s.h
	scene := s.fresh()
	ev := flatte.DropEvent{X: h.cols(x), Y: h.lines(y), Paths: paths, Text: text, Window: s.id}
	if id, r := scene.HitTest(x, y); id != "" {
		ev.Target, ev.TargetRect = id, r.Cells(h.metrics)
		ev.X = ev.TargetRect.X + h.cols(x-r.X)
		ev.Y = ev.TargetRect.Y + h.lines(y-r.Y)
	}
	h.handle(ev)
}

// presentation is what to draw over the scene.
func (s *surface[S]) presentation() presentation { return s.pres }

// selectableAt is the topmost Selectable TextOp at (x, y) points, and its
// index in the scene; none under an overlay layer above it, or over an
// Input or a control.
func (s *surface[S]) selectableAt(x, y float64) (int, layout.TextOp, bool) {
	if x < 0 || y < 0 {
		return 0, layout.TextOp{}, false
	}
	id, _ := s.scene.HitTest(x, y)
	if _, ok := s.input(id); ok {
		return 0, layout.TextOp{}, false
	}
	if _, ok := s.control(id); ok {
		return 0, layout.TextOp{}, false
	}
	in := func(r layout.PointRect) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }
	for i := len(s.scene.Ops) - 1; i >= 0; i-- {
		switch op := s.scene.Ops[i].(type) {
		case layout.TextOp:
			if op.Selectable && in(op.Rect) && in(op.Clip) {
				return i, op, true
			}
		case layout.LayerOp:
			if in(op.Rect) {
				return 0, layout.TextOp{}, false
			}
		}
	}
	return 0, layout.TextOp{}, false
}

// opAt is the scene's op at index i, if it is an O.
func opAt[O layout.Op](s layout.Scene, i int) (O, bool) {
	var zero O
	if i < 0 || i >= len(s.Ops) {
		return zero, false
	}
	op, ok := s.Ops[i].(O)
	return op, ok
}

// selectedText is the text selected in a label, or "".
func (s *surface[S]) selectedText() string {
	sel := s.pres.sel
	op, ok := opAt[layout.TextOp](s.scene, sel.op)
	if !ok {
		return ""
	}
	a, b := sel.in(sel.op, op)
	return op.Text[a:b]
}

// selectText tracks a left-button press, drag, or release at (x, y) for
// label selection: a press on selectable text starts a selection there (and
// one anywhere else clears it); a drag extends it, clamped to its label.
func (s *surface[S]) selectText(x, y float64, action flatte.MouseAction) {
	text := s.h.text
	before := s.pres.sel
	switch action {
	case flatte.MousePress:
		s.pres.sel, s.selecting = textSel{}, false
		if i, op, ok := s.selectableAt(x, y); ok && text != nil {
			at := text.textOffset(op, x, y)
			s.pres.sel = textSel{op: i, text: op.Text, from: at, to: at, ok: true}
			s.selecting = true
		}
	case flatte.MouseMotion:
		if op, ok := opAt[layout.TextOp](s.scene, s.pres.sel.op); s.selecting && ok && op.Text == s.pres.sel.text {
			s.pres.sel.to = text.textOffset(op, x, y)
		}
	case flatte.MouseRelease:
		s.selecting = false
	}
	if s.pres.sel != before {
		s.invalidate()
	}
}

// paste delivers text as a PasteEvent, as a terminal delivers a paste.
func (s *surface[S]) paste() {
	if s.h.clip != nil {
		s.h.handle(flatte.PasteEvent{Text: s.h.clip.get()})
	}
}

// fresh returns the scene as the app now stands, rendering again when the
// app changed since the last one.
func (s *surface[S]) fresh() layout.Scene {
	if s.dirty {
		s.render()
	}
	return s.scene
}

// focusedArea is the topmost focused multi-line Input of a fresh scene.
func (s *surface[S]) focusedArea() (layout.InputOp, bool) {
	ops := s.fresh().Ops
	for i := len(ops) - 1; i >= 0; i-- {
		if in, ok := ops[i].(layout.InputOp); ok && in.Focused && in.Multiline {
			return in, true
		}
	}
	return layout.InputOp{}, false
}

// input is the topmost Input with id in the scene.
func (s *surface[S]) input(id string) (layout.InputOp, bool) {
	if id == "" {
		return layout.InputOp{}, false
	}
	for i := len(s.scene.Ops) - 1; i >= 0; i-- {
		if in, ok := s.scene.Ops[i].(layout.InputOp); ok && in.ID == id {
			return in, true
		}
	}
	return layout.InputOp{}, false
}

// mouse delivers a pointer event at (x, y) points (see pointerAt).
//
// Over an ID'd Input the event carries the caret nearest the pointer; a
// left-button drag that began on one keeps targeting it, with carets
// clamped to it, until the button is released.
func (s *surface[S]) mouse(x, y float64, button flatte.MouseButton, action flatte.MouseAction, mod flatte.Mod) {
	h := s.h
	scene := s.fresh()
	id, r := scene.HitTest(x, y)
	dragging := s.drag != "" && button == flatte.MouseLeft && action != flatte.MousePress
	if dragging {
		id, r = s.drag, scene.Rects[s.drag]
	}
	ev := s.pointerAt(x, y, id, r)
	ev.Button, ev.Action, ev.Mod = button, action, mod
	// A right press (or a Control-click, as on macOS) with a context menu
	// shows it instead of being delivered; the choice arrives as a
	// MenuEvent.
	secondary := button == flatte.MouseRight || button == flatte.MouseLeft && mod.Contains(flatte.ModCtrl)
	if secondary && action == flatte.MousePress && h.app.ContextMenu != nil && h.menus != nil {
		if items := h.app.ContextMenu(h.app.State, id); len(items) > 0 {
			if chosen := h.menus.popUp(s.id, items, x, y); chosen != "" {
				h.handle(flatte.MenuEvent{ID: chosen, Target: id})
			}
			return
		}
	}
	if op, ok := s.input(id); ok && h.text != nil {
		ev.Caret, ev.HasCaret = h.text.caretAt(op, x, y), true
		if button == flatte.MouseLeft && action == flatte.MousePress {
			s.drag = id
		}
	}
	if button == flatte.MouseLeft && action == flatte.MouseRelease {
		s.drag = ""
	}
	// A control shows pressed while the left button is down on it.
	if button == flatte.MouseLeft {
		s.selectText(x, y, action)
		pressed := s.pres.pressed
		switch action {
		case flatte.MousePress:
			if _, ok := s.control(id); ok {
				pressed = id
			}
		case flatte.MouseRelease:
			pressed = ""
		}
		if pressed != s.pres.pressed {
			s.pres.pressed = pressed
			s.invalidate()
		}
	}
	h.handle(ev)
}

func (s *surface[S]) hit(x, y float64) string {
	id, _ := s.fresh().HitTest(x, y)
	return id
}

// wheel delivers notches wheel presses at (x, y), all hit-tested against
// one scene: a fast scroll brings many notches at once, and laying the
// frame out again between them made scrolling stutter.
func (s *surface[S]) wheel(x, y float64, button flatte.MouseButton, notches int, mod flatte.Mod) {
	id, r := s.fresh().HitTest(x, y)
	ev := s.pointerAt(x, y, id, r)
	ev.Button, ev.Action, ev.Mod = button, flatte.MousePress, mod
	if op, ok := s.input(id); ok && s.h.text != nil {
		ev.Caret, ev.HasCaret = s.h.text.caretAt(op, x, y), true
	}
	for range notches {
		s.h.handle(ev)
	}
}

// pointerAt is a pointer event at (x, y) points over target id with rect r
// ("": none). The cell coordinates are chosen so that X-TargetRect.X and
// Y-TargetRect.Y are the exact column and line inside the target, even
// though the target is not aligned to the cell grid.
func (s *surface[S]) pointerAt(x, y float64, id string, r layout.PointRect) flatte.MouseEvent {
	h := s.h
	ev := flatte.MouseEvent{X: h.cols(x), Y: h.lines(y), Window: s.id}
	if id != "" {
		ev.Target, ev.TargetRect = id, r.Cells(h.metrics)
		ev.X = ev.TargetRect.X + h.cols(x-r.X)
		ev.Y = ev.TargetRect.Y + h.lines(y-r.Y)
	}
	return ev
}

// render produces the scene to draw, settling layout first: while the
// solved rects (in cells) differ from the last delivered, deliver them as a
// LayoutEvent and render again, at most maxLayoutPasses times.
func (s *surface[S]) render() layout.Scene {
	h := s.h
	for pass := 0; ; pass++ {
		frame := s.view()
		s.title = frame.Title
		s.cursor = nil
		root := frame.Root
		if root == nil {
			// A string frame shows as a cell grid, with its cursor.
			root = layout.Text{String: frame.Content}
			s.cursor = frame.Cursor
		}
		s.scene = layout.LayoutScene(root, s.width, s.height, h.metrics)
		s.dirty = false
		h.syncMenus()
		// A selection lasts while its label does.
		if sel := s.pres.sel; sel.ok {
			if op, ok := opAt[layout.TextOp](s.scene, sel.op); !ok || !op.Selectable || op.Text != sel.text {
				s.pres.sel, s.selecting = textSel{}, false
			}
		}
		if frame.Root == nil || pass == maxLayoutPasses {
			return s.scene
		}
		rects := make(map[string]layout.Rect, len(s.scene.Rects))
		for id, r := range s.scene.Rects {
			rects[id] = r.Cells(h.metrics)
		}
		if maps.Equal(rects, s.delivered) {
			return s.scene
		}
		s.delivered = rects
		ev := flatte.LayoutEvent{Rects: maps.Clone(rects), Window: s.id}
		h.app.Tracer.Event(ev)
		if h.app.Handle != nil {
			h.app.Handle(h.app.State, flatte.LayoutEvent{Rects: maps.Clone(rects), Window: s.id}, h.fx)
		}
		if h.quitting {
			h.stop()
			return s.scene
		}
	}
}

func (s *surface[S]) view() flatte.Frame {
	if s.h.app.View == nil {
		return flatte.Frame{}
	}
	return s.h.app.View(s.h.app.State, flatte.RenderContext{Width: s.h.cols(s.width), ColorProfile: colorprofile.TrueColor, Window: s.id})
}

// wheelAccumulator turns precise scroll deltas (trackpads, in points) into
// whole wheel notches of one line each, keeping the remainder.
type wheelAccumulator struct{ rest float64 }

// notches adds delta points (positive scrolls content down, i.e. wheel up)
// and returns how many whole lines to report, signed.
func (w *wheelAccumulator) notches(delta, line float64) int {
	if line <= 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return 0
	}
	w.rest += delta / line
	n := math.Trunc(w.rest)
	w.rest -= n
	return int(n)
}

// frameTitle is the last rendered frame's title.
func (s *surface[S]) frameTitle() string { return s.title }

// frameCursor is the last rendered string frame's cursor, in cells.
func (s *surface[S]) frameCursor() *flatte.Cursor { return s.cursor }
