// Package testkit contains Effects and Session construction helpers for
// Flatte's in-repository harnesses and samples. It is not part of the public
// module API.
package testkit

import (
	"context"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/internal/sessionaccess"
)

// NewHarnessEffects creates Effects that send updates to updates. Async work
// uses dispatch (nil means a real goroutine) and clock (nil means real time).
func NewHarnessEffects[S any](ctx context.Context, updates chan<- flatte.StateUpdate[S], quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return NewHarnessEffectsWithServices(ctx, updates, nil, quit, dispatch, clock, flatte.HostServices{})
}

// NewHarnessEffectsQueue creates Effects backed by an enqueue callback.
func NewHarnessEffectsQueue[S any](ctx context.Context, enqueue func(flatte.StateUpdate[S]) bool, quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return NewHarnessEffectsWithServices[S](ctx, nil, func(ctx context.Context, update any) bool {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return false
			default:
			}
		}
		if enqueue == nil {
			return false
		}
		return enqueue(update.(flatte.StateUpdate[S]))
	}, quit, dispatch, clock, flatte.HostServices{})
}

// NewHarnessEffectsWithServices is NewHarnessEffects with host services
// installed on the returned Effects value.
func NewHarnessEffectsWithServices[S any](ctx context.Context, updates chan<- flatte.StateUpdate[S], send func(context.Context, any) bool, quit func(), dispatch func(func()), clock flatte.Clock, services flatte.HostServices) flatte.Effects[S] {
	if ctx == nil {
		ctx = context.Background()
	}
	var effects flatte.Effects[S]
	app := flatte.App[S]{
		State: new(S),
		Init:  func(_ *S, fx flatte.Effects[S]) { effects = fx },
		View:  func(*S, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	session := flatte.NewSession(ctx, app)
	access, ok := sessionaccess.For(session)
	if !ok {
		panic("testkit: session access was not registered")
	}
	showDialog := func(dialog any, done func(any)) {
		if services.ShowDialog != nil {
			services.ShowDialog(dialog.(flatte.Dialog), func(result flatte.DialogResult) { done(result) })
		}
	}
	var dialogs func(any, func(any))
	if services.ShowDialog != nil {
		dialogs = showDialog
	}
	var notify func(any)
	if services.Notify != nil {
		notify = func(notification any) { services.Notify(notification.(flatte.Notification)) }
	}
	var desktop func(any)
	if services.Desktop != nil {
		desktop = func(command any) { services.Desktop(command.(flatte.DesktopCommand)) }
	}
	access.ConfigureTests(sessionaccess.HarnessEffects{
		Context:       ctx,
		Updates:       updates,
		SendUpdate:    send,
		Quit:          quit,
		Dispatch:      dispatch,
		Clock:         clock,
		Capabilities:  services.Capabilities,
		SetClipboard:  services.SetClipboard,
		ReadClipboard: services.ReadClipboard,
		ShowDialog:    dialogs,
		Notify:        notify,
		Desktop:       desktop,
	})
	start := flatte.ResizeEvent{}
	session.Step(flatte.StepInput{Start: &start})
	session.Cancel()
	return effects
}

// SettleFrame uses the root runtime's private cell-layout settling path.
func SettleFrame(view func() flatte.Frame, handle func(flatte.LayoutEvent), last map[string]layout.Rect, width, height int) (flatte.Frame, layout.Composition, map[string]layout.Rect) {
	frame, comp, delivered := sessionaccess.SettleFrame(view, handle, last, width, height)
	return frame.(flatte.Frame), comp, delivered
}

// WithHostMedia constructs the internal media option used by the browser
// smoke and native composite host.
func WithHostMedia(commands <-chan flatte.MediaEvent, publish func(*flatte.NowPlaying)) flatte.Option {
	return sessionaccess.MediaOption(commands, publish).(flatte.Option)
}
