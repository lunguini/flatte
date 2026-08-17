package flatte

import (
	"context"
	"time"
)

// Scope groups async work under a shared cancellable context. Cancel()
// cancels all in-flight work started through the scope's Go, Stream, and
// Every helpers. This closes the "Every/Stream can't be cancelled by name"
// gap that the flat-docker dogfood found (Task 4 — Logs streaming needed a
// 30-line hand-rolled goroutine because flatte.Stream runs for the app's
// entire loop lifetime).
//
// Usage:
//
//	scope := flatte.NewScope(fx, "logs")
//	scope.Stream(fx, func(ctx context.Context, send func(string)) { ... }, fold)
//	// On selection change or screen leave:
//	scope.Cancel()
//	scope = flatte.NewScope(fx, "logs")  // fresh scope for new selection
type Scope struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc
}

// NewScope creates a scope whose context is derived from fx.Ctx().
// The name prefixes the Named updates produced by the scope's helpers.
func NewScope[S any](fx Effects[S], name string) *Scope {
	ctx, cancel := context.WithCancel(fx.Ctx())
	return &Scope{name: name, ctx: ctx, cancel: cancel}
}

// Ctx returns the scope's cancellable context. Use this to derive further
// child contexts if needed.
func (s *Scope) Ctx() context.Context { return s.ctx }

// Cancel cancels all in-flight work started through this scope. The
// scope's context is cancelled, which causes every goroutine spawned by
// ScopeGo/ScopeStream/ScopeEvery to see ctx.Done() and exit. Updates that
// were already queued but have not been applied are discarded as well.
func (s *Scope) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// ScopeGo runs work off-loop using the scope's context. When the scope is
// cancelled, the context passed to work is cancelled. The fold runs on
// the loop goroutine as a named update. The update checks the scope again at
// apply time, so cancelling after enqueueing cannot run a stale fold.
func ScopeGo[S, T any](scope *Scope, fx Effects[S], work func(context.Context) (T, error), fold func(*S, T, error)) {
	fx.spawn(func() {
		value, err := work(scope.ctx)
		if scope.ctx.Err() != nil {
			return
		}
		update := Named(scope.name+":go", func(state *S) {
			if scope.ctx.Err() != nil {
				return // cancelled after queueing, before the loop applied it
			}
			fold(state, value, err)
		})
		fx.send(scope.ctx, update)
	})
}

// ScopeStream runs a long-lived source that emits many values over time.
// Each emitted value becomes one named update. When the scope is cancelled,
// both the source's context and the send-channel select see the cancellation;
// an update already queued at cancellation time is discarded on apply.
func ScopeStream[S, T any](scope *Scope, fx Effects[S], source func(context.Context, func(T)), fold func(*S, T)) {
	fx.spawn(func() {
		source(scope.ctx, func(value T) {
			update := Named(scope.name+":stream", func(state *S) {
				if scope.ctx.Err() != nil {
					return // cancelled after queueing, before the loop applied it
				}
				fold(state, value)
			})
			fx.send(scope.ctx, update)
		})
	})
}

// ScopeEvery sends a named update on a fixed interval until the scope is
// cancelled. Timing comes from the Clock (real ticker by default; fake
// clock under test). A queued tick is discarded if the scope is cancelled
// before the loop applies it. It returns ErrInvalidInterval when interval is
// zero or negative and does not register a ticker.
func ScopeEvery[S any](scope *Scope, fx Effects[S], interval time.Duration, fold func(*S, time.Time)) error {
	if interval <= 0 {
		return ErrInvalidInterval
	}
	clk := fx.clock
	if clk == nil {
		clk = realClock{}
	}
	clk.Tick(scope.ctx, interval, func(now time.Time) {
		update := Named(scope.name+":every", func(state *S) {
			if scope.ctx.Err() != nil {
				return // cancelled after queueing, before the loop applied it
			}
			fold(state, now)
		})
		fx.send(scope.ctx, update)
	})
	return nil
}
