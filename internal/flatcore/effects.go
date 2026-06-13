package flatcore

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Go runs work off-loop and folds its result back into state as one named
// update. It is Async spelled through Effects.
func Go[S, T any](fx Effects[S], name string, work func(context.Context) (T, error), fold func(*S, T, error)) {
	Async(fx.context(), fx.Updates, fx.dispatch, name, work, fold)
}

// Exec releases the terminal (cooked mode, main screen), runs cmd
// attached to it, restores the terminal, and applies the named fold with
// the command's error — all synchronously on the loop goroutine: the TUI
// is paused while cmd runs, exactly like shelling out to $EDITOR. cmd's
// stdin/stdout default to Run's input/output and stderr to os.Stderr,
// each only when unset. Loop-goroutine-only, like all effects; no-op on
// a zero Effects value.
func Exec[S any](fx Effects[S], name string, cmd *exec.Cmd, fold func(*S, error)) {
	if fx.enqueue == nil {
		return
	}
	fx.enqueue(action{exec: &execAction{
		cmd: cmd,
		done: func(err error) {
			update := Named(name, func(s *S) { fold(s, err) })
			select {
			case fx.Updates <- update:
			default:
				// The updates channel is full (a burst of async results
				// landed during the exec). Spill to a goroutine instead of
				// deadlocking the loop against its own channel.
				ctx := fx.context()
				go func() {
					select {
					case fx.Updates <- update:
					case <-ctx.Done():
					}
				}()
			}
		},
	}})
}

// Every sends a named update on a fixed interval until the loop context is
// cancelled. Timing comes from the Clock (real ticker by default; a fake
// clock drives it deterministically under test).
func Every[S any](fx Effects[S], name string, interval time.Duration, fold func(*S, time.Time)) {
	clk := fx.clock
	if clk == nil {
		clk = realClock{}
	}
	ctx := fx.context()
	clk.Tick(ctx, interval, func(now time.Time) {
		update := Named(name, func(s *S) { fold(s, now) })
		select {
		case fx.Updates <- update:
		case <-ctx.Done():
		}
	})
}

// Stream runs a long-lived source that emits many values over time; each
// emitted value becomes one named update. The source must return when its
// context is cancelled.
func Stream[S, T any](fx Effects[S], name string, source func(context.Context, func(T)), fold func(*S, T)) {
	ctx := fx.context()
	fx.spawn(func() {
		source(ctx, func(value T) {
			update := Named(name, func(s *S) { fold(s, value) })
			select {
			case fx.Updates <- update:
			case <-ctx.Done():
			}
		})
	})
}

// Latest is Go with supersede-by-name semantics: starting new work under a
// name cancels any in-flight work under the same name, and a superseded
// result is dropped even if it was already queued. This replaces manual
// generation counters for request/response races.
//
// On a zero Effects value (no registry) it degrades to Go.
func Latest[S, T any](fx Effects[S], name string, work func(context.Context) (T, error), fold func(*S, T, error)) {
	if fx.latest == nil {
		Go(fx, name, work, fold)
		return
	}
	ctx, entry := fx.latest.replace(name, fx.context())
	fx.spawn(func() {
		value, err := work(ctx)
		if ctx.Err() != nil {
			// Superseded, Cancelled, or parent shutdown. release is
			// identity-checked, so it only removes the entry when replace or
			// cancel has not already done so (parent shutdown).
			fx.latest.release(name, entry)
			return
		}
		// The entry must stay registered until the update is APPLIED, not
		// merely sent: as long as the update sits in the queue, a newer
		// Latest call must still find this generation to cancel so the
		// apply-time ctx.Err() guard can drop the stale result.
		update := Named(name, func(s *S) {
			defer fx.latest.release(name, entry)
			if ctx.Err() != nil {
				return // superseded or cancelled after queueing
			}
			fold(s, value, err)
		})
		select {
		case fx.Updates <- update:
		case <-ctx.Done():
			fx.latest.release(name, entry)
		}
	})
}

// Cancel stops in-flight Latest work under name, if any.
func Cancel[S any](fx Effects[S], name string) {
	if fx.latest == nil {
		return
	}
	fx.latest.cancel(name)
}

func (fx Effects[S]) context() context.Context {
	if fx.Context == nil {
		return context.Background()
	}
	return fx.Context
}

// latestEntry identifies one generation of Latest work. Its pointer identity
// is what lets release distinguish "my entry" from a newer replacement.
type latestEntry struct {
	cancel context.CancelFunc
}

// latestRegistry tracks the in-flight cancellation handle per Latest name.
//
// Lifecycle contract: replace registers a new generation, cancelling and
// superseding any previous one under the same name. An entry leaves the map
// in exactly one of three ways: replace (superseded by a newer generation),
// cancel (explicit Cancel call), or release (the owning generation's update
// was applied, or its goroutine exited on a cancelled context). An entry is
// deliberately kept while its update sits unapplied in the queue, so a newer
// Latest call can still cancel it and the apply-time guard drops the stale
// result. release is identity-checked so a slow old goroutine can never
// evict a newer generation's entry.
type latestRegistry struct {
	mu      sync.Mutex
	entries map[string]*latestEntry
}

func newLatestRegistry() *latestRegistry {
	return &latestRegistry{entries: make(map[string]*latestEntry)}
}

// replace cancels any existing work under name and returns a fresh child
// context registered under that name, plus the entry identifying this
// generation for a later release.
func (r *latestRegistry) replace(name string, parent context.Context) (context.Context, *latestEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	entry := &latestEntry{cancel: cancel}
	r.entries[name] = entry
	return ctx, entry
}

func (r *latestRegistry) cancel(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.cancel()
		delete(r.entries, name)
	}
}

// release removes the entry under name only if it is still owner's own
// generation; entries already superseded or cancelled are left alone.
func (r *latestRegistry) release(name string, owner *latestEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries[name] == owner {
		delete(r.entries, name)
	}
}
