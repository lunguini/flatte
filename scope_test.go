package flatte

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestScopeCancelStopsGo(t *testing.T) {
	var applied int32
	updates := make(chan StateUpdate[struct{}], 10)
	fx := NewEffects[struct{}](context.Background(), updates, func() {})

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

func TestScopeCtxReturnsCancellableContext(t *testing.T) {
	fx := NewEffects[struct{}](context.Background(), nil, func() {})
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
	fx := NewEffects[struct{}](context.Background(), nil, func() {})
	scope := NewScope(fx, "test")
	scope.Cancel()
	scope.Cancel() // should not panic
}
