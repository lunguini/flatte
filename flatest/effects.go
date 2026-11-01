package flatest

import (
	"context"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/testkit"
)

// NewHarnessEffects builds an Effects value for callback-level tests that
// need to pass Effects directly to Handle or observe named updates. Prefer
// Start when a test should exercise the complete app loop.
func NewHarnessEffects[S any](ctx context.Context, updates chan<- flatte.StateUpdate[S], quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return testkit.NewHarnessEffects(ctx, updates, quit, dispatch, clock)
}

// NewHarnessEffectsQueue is NewHarnessEffects with a caller-owned update
// queue, useful when tests must avoid blocking on channel capacity.
func NewHarnessEffectsQueue[S any](ctx context.Context, enqueue func(flatte.StateUpdate[S]) bool, quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return testkit.NewHarnessEffectsQueue(ctx, enqueue, quit, dispatch, clock)
}
