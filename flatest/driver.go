// Package flatest is a deterministic, synchronous test harness for
// flat apps: it drives an App through scripted events and controlled
// time, exercising real async folds without goroutine races, real clocks,
// or a terminal.
package flatest

import (
	"context"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// Driver runs a flatte.App synchronously for tests. App-triggered async
// (Go/Latest/Stream) is deferred to a pending queue and only runs on
// Settle; time-based effects (Every) are driven by a fake clock advanced
// with Advance. The width is fixed so frames are deterministic.
type Driver[S any] struct {
	app    flatte.App[S]
	width  int
	height int // from the last ResizeEvent; Root frames compose at width×height
	// delivered is the rect set last delivered as a LayoutEvent; comp is the
	// composition of the last rendered frame, which mouse events hit-test
	// against — the frame "on screen", as in Run.
	delivered map[string]layout.Rect
	comp      layout.Composition
	// menus routes shortcuts and context menus, and windows draws the
	// app's secondary windows over the main frame, as Run does.
	menus   flatte.MenuHost
	windows flatte.WindowHost
	updates []flatte.StateUpdate[S]
	clock   *fakeClock
	pending []func()
	quit    bool
	fx      flatte.Effects[S]
	// dialogs are the dialogs the app asked for that the test has not
	// answered, oldest first.
	dialogs []pendingDialog
}

// Start builds a Driver, runs Init, delivers the initial ResizeEvent, and
// renders the first frame.
func Start[S any](app flatte.App[S], width int) *Driver[S] {
	d := &Driver[S]{
		app:     app,
		width:   width,
		updates: make([]flatte.StateUpdate[S], 0, 64),
		clock:   newFakeClock(),
	}
	// One Effects for the whole session: the latest registry (and thus
	// Latest's cross-event supersede) must persist across events, exactly
	// like Run's single long-lived Effects.
	d.fx = flatte.NewHarnessEffectsQueue(
		context.Background(), func(update flatte.StateUpdate[S]) bool {
			d.updates = append(d.updates, update)
			return true
		},
		func() { d.quit = true },
		func(f func()) { d.pending = append(d.pending, f) },
		d.clock,
	)
	d.fx = flatte.WithHostDialogs(d.fx, func(dg flatte.Dialog, done func(flatte.DialogResult)) {
		d.dialogs = append(d.dialogs, pendingDialog{dg, done})
	})
	if app.Init != nil {
		app.Init(app.State, d.fx)
	}
	d.deliver(flatte.ResizeEvent{Width: width, Height: 24})
	d.render()
	return d
}

// Send delivers one event, drains the synchronous updates it produced,
// and returns the rendered frame. Async results triggered by the event
// are NOT applied here — call Settle.
func (d *Driver[S]) Send(ev flatte.Event) flatte.Frame {
	d.deliver(ev)
	return d.render()
}

// render settles layout and renders, exactly as Run draws: LayoutEvents for a
// Root frame are delivered (and their updates drained) before the frame is
// returned, and the composition is kept for hit-testing the next mouse event.
func (d *Driver[S]) render() flatte.Frame {
	for _, ev := range flatte.SyncWindows(&d.windows, d.app, d.width, d.height) {
		d.handle(ev)
	}
	frame, comp, delivered := flatte.SettleFrame(d.view, d.deliverLayout, d.delivered, d.width, d.height)
	d.delivered, d.comp = delivered, comp
	frame = flatte.PaintWindows(&d.windows, d.app, frame, d.deliverLayout)
	return d.menus.Paint(frame)
}

func (d *Driver[S]) deliverLayout(ev flatte.LayoutEvent) { d.deliver(ev) }

func (d *Driver[S]) deliver(ev flatte.Event) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		d.height = e.Height
	case flatte.MouseEvent:
		e.Target, e.TargetRect = d.comp.HitTest(e.X, e.Y)
		e.Caret, e.HasCaret = d.comp.CaretAt(e.X, e.Y)
		ev = e
	}
	if _, layout := ev.(flatte.LayoutEvent); layout {
		d.handle(ev)
		return
	}
	for _, ev := range flatte.RouteInput(&d.menus, &d.windows, d.app, ev, d.width, d.height) {
		d.handle(ev)
	}
}

// handle delivers one routed event and drains the updates it produced.
func (d *Driver[S]) handle(ev flatte.Event) {
	if d.app.Tracer != nil {
		d.app.Tracer.Event(ev)
	}
	if d.app.Handle != nil {
		d.app.Handle(d.app.State, ev, d.fx)
	}
	d.drain()
}

// drain applies every queued update to completion — the harness is
// synchronous, so there is no per-frame cap like Run's drainUpdates.
func (d *Driver[S]) drain() {
	for len(d.updates) > 0 {
		u := d.updates[0]
		d.updates[0] = nil
		d.updates = d.updates[1:]
		flatte.ApplyUpdate(d.app.State, d.app.Tracer, u)
	}
}

// Settle runs every pending async body (to fixpoint — a body may dispatch
// more) and drains the updates each produces, then renders. This is where
// Go/Latest/Stream results land, under test control. drain runs after each
// body so Latest's apply-time guard sees a superseded ctx and drops the
// stale result.
func (d *Driver[S]) Settle() flatte.Frame {
	for len(d.pending) > 0 {
		batch := d.pending
		d.pending = nil
		for _, f := range batch {
			f()
			d.drain()
		}
	}
	return d.render()
}

// Advance moves the fake clock, firing due Every ticks (which queue
// updates), then settles any async those ticks triggered.
func (d *Driver[S]) Advance(by time.Duration) flatte.Frame {
	d.clock.advance(by)
	d.drain()
	return d.Settle()
}

// Frame renders the current state without changing it. A frame with a Root
// is composed at the driver size, as Run composes it at the terminal size.
func (d *Driver[S]) Frame() flatte.Frame {
	frame, _ := flatte.ComposeFrame(d.view(), d.width, d.height)
	frame = flatte.PaintWindows(&d.windows, d.app, frame, nil)
	return d.menus.Paint(frame)
}

// WindowFrame is secondary window id's own frame as last rendered (composed
// at its size, without the box the terminal draws around it); ok is false
// when the app does not declare it. Mouse events for it are sent in screen
// cells, as a terminal reports them: over its box in Frame.
func (d *Driver[S]) WindowFrame(id string) (frame flatte.Frame, ok bool) {
	return d.windows.WindowFrame(id)
}

func (d *Driver[S]) view() flatte.Frame {
	return d.app.View(d.app.State, flatte.RenderContext{Width: d.width})
}

// Frames runs the driver through a sequence of steps and returns the
// frame after each, for sequence goldens (see AssertFrames).
func Frames[S any](d *Driver[S], steps ...func(*Driver[S])) []flatte.Frame {
	frames := make([]flatte.Frame, 0, len(steps))
	for _, step := range steps {
		step(d)
		frames = append(frames, d.Frame())
	}
	return frames
}

type pendingDialog struct {
	dialog flatte.Dialog
	done   func(flatte.DialogResult)
}

// Dialog is the oldest dialog the app asked for (OpenFile, SaveFile, Alert)
// that the test has not answered; ok is false when there is none. The
// driver draws no dialogs: tests answer them with Answer.
func (d *Driver[S]) Dialog() (dialog flatte.Dialog, ok bool) {
	if len(d.dialogs) == 0 {
		return flatte.Dialog{}, false
	}
	return d.dialogs[0].dialog, true
}

// Answer answers the oldest open dialog as the user would — the chosen
// paths (none: cancelled) or button (-1: dismissed) — applies the app's
// fold, and returns the rendered frame. It panics when no dialog is open.
func (d *Driver[S]) Answer(r flatte.DialogResult) flatte.Frame {
	if len(d.dialogs) == 0 {
		panic("flatest: Answer with no dialog open")
	}
	p := d.dialogs[0]
	d.dialogs = d.dialogs[1:]
	p.done(r)
	d.drain()
	return d.render()
}

// State exposes the live state for field assertions.
func (d *Driver[S]) State() *S { return d.app.State }

// Quit reports whether the app has requested quit via fx.Quit().
func (d *Driver[S]) Quit() bool { return d.quit }

// Close calls the app's OnExit hook, mirroring what Run does on loop exit.
// Call it at the end of a test to verify state-save callbacks.
func (d *Driver[S]) Close() {
	if d.app.OnExit != nil {
		d.app.OnExit(d.app.State)
	}
}
