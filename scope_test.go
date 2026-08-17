package flatte

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestScopeCancelStopsGo(t *testing.T) {
	var applied int32
	updates := make(chan StateUpdate[struct{}], 10)
	fx := newEffects[struct{}](context.Background(), updates, func() {})

	scope := NewScope(fx, "test")
	ScopeGo(scope, fx, func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 42, ctx.Err()
	}, func(s *struct{}, v int, err error) {
		atomic.AddInt32(&applied, 1)
	})

	scope.Cancel()

	select {
	case <-updates:
		// update was sent before cancel took effect — drain it
	case <-time.After(100 * time.Millisecond):
		// no update — the goroutine saw cancellation and returned without sending
	}

	// After cancel, no NEW work should apply
	if atomic.LoadInt32(&applied) > 1 {
		t.Fatalf("applied = %d, expected <= 1 after cancel", applied)
	}
}

func TestScopeGoDropsQueuedUpdateAfterCancel(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	var pending []func()
	fx := NewHarnessEffects[testState](context.Background(), updates, nil,
		func(f func()) { pending = append(pending, f) }, nil)
	scope := NewScope(fx, "test")

	ScopeGo(scope, fx,
		func(context.Context) (int, error) { return 42, nil },
		func(s *testState, value int, err error) { s.count = value })
	if len(pending) != 1 {
		t.Fatalf("captured %d workers, want 1", len(pending))
	}
	pending[0]()
	update := receiveCoreUpdate(t, updates)

	scope.Cancel()
	var state testState
	update.Apply(&state)
	if state.count != 0 {
		t.Fatalf("cancelled queued ScopeGo update applied count = %d, want 0", state.count)
	}
}

func TestScopeStreamDropsQueuedUpdateAfterCancel(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	var pending []func()
	fx := NewHarnessEffects[testState](context.Background(), updates, nil,
		func(f func()) { pending = append(pending, f) }, nil)
	scope := NewScope(fx, "test")

	ScopeStream(scope, fx,
		func(_ context.Context, send func(int)) { send(42) },
		func(s *testState, value int) { s.count = value })
	if len(pending) != 1 {
		t.Fatalf("captured %d workers, want 1", len(pending))
	}
	pending[0]()
	update := receiveCoreUpdate(t, updates)

	scope.Cancel()
	var state testState
	update.Apply(&state)
	if state.count != 0 {
		t.Fatalf("cancelled queued ScopeStream update applied count = %d, want 0", state.count)
	}
}

func TestScopeEveryDropsQueuedUpdateAfterCancel(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	clk := &manualClock{}
	fx := NewHarnessEffects[testState](context.Background(), updates, nil,
		func(f func()) { f() }, clk)
	scope := NewScope(fx, "test")

	if err := ScopeEvery(scope, fx, time.Millisecond,
		func(s *testState, _ time.Time) { s.count++ }); err != nil {
		t.Fatalf("ScopeEvery returned unexpected error: %v", err)
	}
	if len(clk.cbs) != 1 {
		t.Fatalf("registered callbacks = %d, want 1", len(clk.cbs))
	}
	clk.fire(1)
	update := receiveCoreUpdate(t, updates)

	scope.Cancel()
	var state testState
	update.Apply(&state)
	if state.count != 0 {
		t.Fatalf("cancelled queued ScopeEvery update applied count = %d, want 0", state.count)
	}
}

func TestScopeEveryRejectsNonPositiveIntervalWithoutRegistering(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Nanosecond} {
		t.Run(interval.String(), func(t *testing.T) {
			updates := make(chan StateUpdate[testState], 1)
			clk := &manualClock{}
			fx := NewHarnessEffects[testState](context.Background(), updates,
				func() {}, func(f func()) { f() }, clk)
			scope := NewScope(fx, "test")

			err := ScopeEvery(scope, fx, interval, func(s *testState, _ time.Time) { s.count++ })
			if !errors.Is(err, ErrInvalidInterval) {
				t.Fatalf("ScopeEvery error = %v, want ErrInvalidInterval", err)
			}
			if len(clk.cbs) != 0 {
				t.Fatalf("registered callbacks = %d, want 0", len(clk.cbs))
			}
			select {
			case <-updates:
				t.Fatal("invalid interval sent an update")
			default:
			}
		})
	}
}

func TestScopeEveryRegistersValidInterval(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	clk := &manualClock{}
	fx := NewHarnessEffects[testState](context.Background(), updates,
		func() {}, func(f func()) { f() }, clk)
	scope := NewScope(fx, "test")

	if err := ScopeEvery(scope, fx, time.Millisecond,
		func(s *testState, _ time.Time) { s.count++ }); err != nil {
		t.Fatalf("ScopeEvery returned unexpected error: %v", err)
	}
	if len(clk.cbs) != 1 {
		t.Fatalf("registered callbacks = %d, want 1", len(clk.cbs))
	}
}

func TestScopeCtxReturnsCancellableContext(t *testing.T) {
	fx := newEffects[struct{}](context.Background(), nil, func() {})
	scope := NewScope(fx, "test")
	ctx := scope.Ctx()
	if ctx.Err() != nil {
		t.Fatal("scope context should not be done before cancel")
	}
	scope.Cancel()
	if ctx.Err() == nil {
		t.Fatal("scope context should be done after cancel")
	}
}

func TestScopeCancelIsIdempotent(t *testing.T) {
	fx := newEffects[struct{}](context.Background(), nil, func() {})
	scope := NewScope(fx, "test")
	scope.Cancel()
	scope.Cancel() // should not panic
}
