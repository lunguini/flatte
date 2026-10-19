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
	updates   []flatte.StateUpdate[S]
	clock     *fakeClock
	pending   []func()
	quit      bool
	fx        flatte.Effects[S]
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
	frame, comp, delivered := flatte.SettleFrame(d.view, d.deliverLayout, d.delivered, d.width, d.height)
	d.delivered, d.comp = delivered, comp
	return frame
}

func (d *Driver[S]) deliverLayout(ev flatte.LayoutEvent) { d.deliver(ev) }

func (d *Driver[S]) deliver(ev flatte.Event) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		d.height = e.Height
	case flatte.MouseEvent:
		e.Target, e.TargetRect = d.comp.HitTest(e.X, e.Y)
		ev = e
	}
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
	return frame
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
