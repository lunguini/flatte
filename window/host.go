package window

import (
	"context"
	"maps"
	"math"

	"github.com/charmbracelet/colorprofile"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// maxLayoutPasses bounds how many LayoutEvents one render delivers, as in
// the terminal host (flatte.SettleFrame).
const maxLayoutPasses = 2

// host drives an App from window events: everything about running an app in
// a window that a platform backend does not need to know. It runs on the UI
// thread only; async results arrive through post.
type host[S any] struct {
	app     flatte.App[S]
	fx      flatte.Effects[S]
	metrics layout.Metrics
	cancel  context.CancelFunc

	// stop ends the platform run loop; invalidate requests a redraw. Both
	// are called on the UI thread.
	stop       func()
	invalidate func()

	width, height float64 // view size in points
	scene         layout.Scene
	title         string
	// cursor is a string frame's Frame.Cursor, in cells; Root frames show the
	// focused Input's caret instead.
	cursor    *flatte.Cursor
	delivered map[string]layout.Rect
	quitting  bool
}

// newHost wires app to a backend. post runs a function on the UI thread
// (from any goroutine); async updates reach the app through it.
func newHost[S any](ctx context.Context, app flatte.App[S], m layout.Metrics, post func(func()), stop, invalidate func()) *host[S] {
	if app.Tracer == nil {
		app.Tracer = flatte.NoopTracer{}
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &host[S]{app: app, metrics: m, cancel: cancel, stop: stop, invalidate: invalidate}
	updates := make(chan flatte.StateUpdate[S], 256)
	h.fx = flatte.NewHarnessEffects(ctx, updates, func() { h.quitting = true }, nil, nil)
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

// close releases the host's goroutines and runs OnExit.
func (h *host[S]) close() {
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

// after reacts to a state change: stop on quit, else redraw.
func (h *host[S]) after() {
	if h.quitting {
		h.stop()
		return
	}
	h.invalidate()
}

func (h *host[S]) cols(points float64) int  { return cellsIn(points, h.metrics.Column) }
func (h *host[S]) lines(points float64) int { return cellsIn(points, h.metrics.Line) }

func cellsIn(points, unit float64) int {
	return layout.PointRect{W: points}.Cells(layout.Metrics{Column: unit}).W
}

// resize records the view size and tells the app its size in cells.
func (h *host[S]) resize(width, height float64) {
	if width == h.width && height == h.height {
		return
	}
	h.width, h.height = width, height
	h.handle(flatte.ResizeEvent{Width: h.cols(width), Height: h.lines(height)})
}

// key delivers a key event.
func (h *host[S]) key(ev flatte.KeyEvent) { h.handle(ev) }

// mouse delivers a pointer event at (x, y) points. The cell coordinates are
// chosen so that X-TargetRect.X and Y-TargetRect.Y are the exact column and
// line inside the target, even though the target is not aligned to the
// cell grid.
func (h *host[S]) mouse(x, y float64, button flatte.MouseButton, action flatte.MouseAction, mod flatte.Mod) {
	ev := flatte.MouseEvent{Button: button, Action: action, Mod: mod, X: h.cols(x), Y: h.lines(y)}
	if id, r := h.scene.HitTest(x, y); id != "" {
		ev.Target, ev.TargetRect = id, r.Cells(h.metrics)
		ev.X = ev.TargetRect.X + h.cols(x-r.X)
		ev.Y = ev.TargetRect.Y + h.lines(y-r.Y)
	}
	h.handle(ev)
}

// render produces the scene to draw, settling layout first: while the
// solved rects (in cells) differ from the last delivered, deliver them as a
// LayoutEvent and render again, at most maxLayoutPasses times.
func (h *host[S]) render() layout.Scene {
	for pass := 0; ; pass++ {
		frame := h.view()
		h.title = frame.Title
		h.cursor = nil
		root := frame.Root
		if root == nil {
			// A string frame shows as a cell grid, with its cursor.
			root = layout.Text{String: frame.Content}
			h.cursor = frame.Cursor
		}
		h.scene = layout.LayoutScene(root, h.width, h.height, h.metrics)
		if frame.Root == nil || pass == maxLayoutPasses {
			return h.scene
		}
		rects := make(map[string]layout.Rect, len(h.scene.Rects))
		for id, r := range h.scene.Rects {
			rects[id] = r.Cells(h.metrics)
		}
		if maps.Equal(rects, h.delivered) {
			return h.scene
		}
		h.delivered = rects
		h.app.Tracer.Event(flatte.LayoutEvent{Rects: maps.Clone(rects)})
		if h.app.Handle != nil {
			h.app.Handle(h.app.State, flatte.LayoutEvent{Rects: maps.Clone(rects)}, h.fx)
		}
		if h.quitting {
			h.stop()
			return h.scene
		}
	}
}

func (h *host[S]) view() flatte.Frame {
	if h.app.View == nil {
		return flatte.Frame{}
	}
	return h.app.View(h.app.State, flatte.RenderContext{Width: h.cols(h.width), ColorProfile: colorprofile.TrueColor})
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
func (h *host[S]) frameTitle() string { return h.title }

// frameCursor is the last rendered string frame's cursor, in cells.
func (h *host[S]) frameCursor() *flatte.Cursor { return h.cursor }
