// Package flatest is a deterministic, synchronous test harness for
// flatcore apps: it drives an App through scripted events and controlled
// time, exercising real async folds without goroutine races, real clocks,
// or a terminal.
package flatest

import (
	"context"

	"github.com/lunguini/flat/internal/flatcore"
)

// Driver runs a flatcore.App synchronously for tests. App-triggered async
// (Go/Latest/Stream) is deferred to a pending queue and only runs on
// Settle; time-based effects (Every) are driven by a fake clock advanced
// with Advance. The width is fixed so frames are deterministic.
type Driver[S any] struct {
	app     flatcore.App[S]
	width   int
	updates chan flatcore.StateUpdate[S]
	clock   *fakeClock
	pending []func()
	quit    bool
}

// Start builds a Driver, runs Init, delivers the initial ResizeEvent, and
// renders the first frame.
func Start[S any](app flatcore.App[S], width int) *Driver[S] {
	d := &Driver[S]{
		app:     app,
		width:   width,
		updates: make(chan flatcore.StateUpdate[S], 1024),
		clock:   newFakeClock(),
	}
	if app.Init != nil {
		app.Init(app.State, d.effects())
	}
	d.deliver(flatcore.ResizeEvent{Width: width, Height: 24})
	return d
}

func (d *Driver[S]) effects() flatcore.Effects[S] {
	return flatcore.NewHarnessEffects(
		context.Background(), d.updates,
		func() { d.quit = true },
		func(f func()) { d.pending = append(d.pending, f) },
		d.clock,
	)
}

// Send delivers one event, drains the synchronous updates it produced,
// and returns the rendered frame. Async results triggered by the event
// are NOT applied here — call Settle.
func (d *Driver[S]) Send(ev flatcore.Event) flatcore.Frame {
	d.deliver(ev)
	return d.Frame()
}

func (d *Driver[S]) deliver(ev flatcore.Event) {
	if d.app.Tracer != nil {
		d.app.Tracer.Event(ev)
	}
	if d.app.Handle != nil {
		d.app.Handle(d.app.State, ev, d.effects())
	}
	d.drain()
}

// drain applies every queued update to completion — the harness is
// synchronous, so there is no per-frame cap like Run's drainUpdates.
func (d *Driver[S]) drain() {
	for {
		select {
		case u := <-d.updates:
			flatcore.ApplyUpdate(d.app.State, d.app.Tracer, u)
		default:
			return
		}
	}
}

// Frame renders the current state without changing it.
func (d *Driver[S]) Frame() flatcore.Frame {
	return d.app.View(d.app.State, flatcore.RenderContext{Width: d.width})
}

// State exposes the live state for field assertions.
func (d *Driver[S]) State() *S { return d.app.State }

// Quit reports whether the app has requested quit via fx.Quit().
func (d *Driver[S]) Quit() bool { return d.quit }
