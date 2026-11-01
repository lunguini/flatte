package flatte

import (
	"context"
)

func newHarnessEffectsForTest[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func(), dispatch func(func()), clock Clock) Effects[S] {
	fx := newEffects[S](ctx, updates, quit)
	fx.dispatch, fx.clock = dispatch, clock
	return fx
}

func newHarnessEffectsQueueForTest[S any](ctx context.Context, enqueue func(StateUpdate[S]) bool, quit func(), dispatch func(func()), clock Clock) Effects[S] {
	fx := newEffects[S](ctx, nil, quit)
	fx.sendUpdate = func(sendCtx context.Context, update StateUpdate[S]) bool {
		if sendCtx == nil {
			sendCtx = context.Background()
		}
		select {
		case <-sendCtx.Done():
			return false
		default:
		}
		if enqueue == nil {
			return false
		}
		return enqueue(update)
	}
	fx.dispatch, fx.clock = dispatch, clock
	return fx
}
