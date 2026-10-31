// Package flatest is a deterministic, synchronous test harness for
// flat apps: it drives an App through scripted events and controlled
// time, exercising real async folds without goroutine races, real clocks,
// or a terminal.
package flatest

import (
	"context"
	"time"

	"github.com/lunguini/flatte"
	flattehost "github.com/lunguini/flatte/host"
)

// Driver runs a flatte.App synchronously for tests. App-triggered async
// (Go/Latest/Stream) is deferred to a pending queue and only runs on
// Settle; time-based effects (Every) are driven by a fake clock advanced
// with Advance. The width is fixed so frames are deterministic.
type Driver[S any] struct {
	app       flatte.App[S]
	session   flattehost.Session
	hostState flattehost.State
	width     int
	height    int // from the last ResizeEvent; Root frames compose at width×height
	graphical bool
	updates   []flatte.StateUpdate[S]
	clock     *fakeClock
	pending   []func()
	quit      bool
	fx        flatte.Effects[S]
	// dialogs are the dialogs the app asked for that the test has not
	// answered, oldest first.
	dialogs []pendingDialog
	// notifications are the ones the app posted, oldest first.
	notifications []flatte.Notification
}

// Start builds a Driver, runs Init, delivers the initial ResizeEvent, and
// renders the first frame.
func Start[S any](app flatte.App[S], width int) *Driver[S] {
	return StartWith(app, Options{Width: width, Height: 24})
}

// Options configures the deterministic render context of a Driver.
type Options struct {
	Width, Height int
	Graphical     bool
	Capabilities  flatte.Capabilities
}

// StartWith builds a Driver with an explicit size and host render context.
func StartWith[S any](app flatte.App[S], opts Options) *Driver[S] {
	if opts.Width <= 0 {
		opts.Width = 72
	}
	if opts.Height <= 0 {
		opts.Height = 24
	}
	d := &Driver[S]{
		app:       app,
		width:     opts.Width,
		height:    opts.Height,
		graphical: opts.Graphical,
		updates:   make([]flatte.StateUpdate[S], 0, 64),
		clock:     newFakeClock(),
	}
	// One Effects for the whole session: the latest registry (and thus
	// Latest's cross-event supersede) must persist across events, exactly
	// like Run's single long-lived Effects.
	d.fx = flattehost.NewHarnessEffectsQueue(
		context.Background(), func(update flatte.StateUpdate[S]) bool {
			d.updates = append(d.updates, update)
			return true
		},
		func() { d.quit = true },
		func(f func()) { d.pending = append(d.pending, f) },
		d.clock,
	)
	d.fx = flattehost.WithHostDialogs(d.fx, func(dg flatte.Dialog, done func(flatte.DialogResult)) {
		d.dialogs = append(d.dialogs, pendingDialog{dg, done})
	})
	d.fx = flattehost.WithHostNotifications(d.fx, func(n flatte.Notification) {
		d.notifications = append(d.notifications, n)
	})
	d.fx = flattehost.WithHostCapabilities(d.fx, opts.Capabilities)
	sessionApp := app
	sessionApp.Init = func(s *S, _ flatte.Effects[S]) {
		if app.Init != nil {
			app.Init(s, d.fx)
		}
	}
	sessionApp.Handle = func(s *S, ev flatte.Event, _ flatte.Effects[S]) {
		if app.Handle != nil {
			app.Handle(s, ev, d.fx)
		}
		d.drain()
	}
	d.session = flattehost.NewSession(context.Background(), sessionApp)
	d.session.UseHostServices(flattehost.Services{Capabilities: opts.Capabilities, CellSurface: true})
	d.session.Step(flattehost.StepInput{Start: &flatte.ResizeEvent{Width: opts.Width, Height: opts.Height}})
	d.hostState = flattehost.Snapshot(d.session)
	d.render()
	return d
}

// Resize updates dimensions used by Handle, View, and root layout composition.
func (d *Driver[S]) Resize(width, height int) flatte.Frame {
	if width > 0 {
		d.width = width
	}
	if height > 0 {
		d.height = height
	}
	d.step(flatte.ResizeEvent{Width: d.width, Height: d.height})
	return d.render()
}

// Send delivers one event, drains the synchronous updates it produced,
// and returns the rendered frame. Async results triggered by the event
// are NOT applied here — call Settle.
func (d *Driver[S]) Send(ev flatte.Event) flatte.Frame {
	d.step(ev)
	return d.render()
}

// render asks the shared session step core to settle and paint a cell frame.
func (d *Driver[S]) render() flatte.Frame {
	result := d.session.Step(flattehost.StepInput{FrameDue: true, RenderContext: d.renderContext()})
	d.hostState = flattehost.Snapshot(d.session)
	return result.Frame
}

func (d *Driver[S]) step(ev flatte.Event) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		d.width = e.Width
		d.height = e.Height
	}
	d.session.Step(flattehost.StepInput{Events: []flatte.Event{ev}, RenderContext: d.renderContext()})
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

// Frame renders the current state through the same settled cell-frame rules
// as Send, Resize, and Run.
func (d *Driver[S]) Frame() flatte.Frame {
	return d.render()
}

// WindowFrame is secondary window id's own frame as last rendered (composed
// at its size, without the box the terminal draws around it); ok is false
// when the app does not declare it. Mouse events for it are sent in screen
// cells, as a terminal reports them: over its box in Frame.
func (d *Driver[S]) WindowFrame(id string) (frame flatte.Frame, ok bool) {
	if d.hostState.WindowFrame == nil {
		return flatte.Frame{}, false
	}
	return d.hostState.WindowFrame(id)
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

// Notifications are the notifications the app posted with Notify, oldest
// first. The driver shows none; tests read them here.
func (d *Driver[S]) Notifications() []flatte.Notification {
	return append([]flatte.Notification(nil), d.notifications...)
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
	flattehost.CloseSession(d.session)
}

// renderContext is what View sees: the driver's size and render context.
func (d *Driver[S]) renderContext() flatte.RenderContext {
	return flatte.RenderContext{Width: d.width, Height: d.height, Graphical: d.graphical, Capabilities: d.fx.Capabilities()}
}
