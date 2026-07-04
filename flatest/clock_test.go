package flatest

import (
	"context"
	"testing"
	"time"
)

// A callback that registers a new ticker mid-advance must not lose that
// registration: it fires on subsequent advances. This is the "cancel + re-arm
// an Every from inside a tick fold" pattern (e.g. a game changing speed on
// level-up); the old compact-in-place advance dropped the new ticker.
func TestAdvanceKeepsTickersRegisteredDuringCallbacks(t *testing.T) {
	c := newFakeClock()
	ctx := context.Background()

	var rearmed int
	registered := false
	c.Tick(ctx, 10*time.Millisecond, func(time.Time) {
		if !registered {
			registered = true
			c.Tick(ctx, 10*time.Millisecond, func(time.Time) { rearmed++ })
		}
	})

	c.advance(10 * time.Millisecond) // outer fires, registers the inner ticker
	if rearmed != 0 {
		t.Fatalf("inner ticker fired during the advance that registered it (rearmed=%d)", rearmed)
	}
	c.advance(10 * time.Millisecond)
	if rearmed != 1 {
		t.Fatalf("inner ticker fired %d times after one interval, want 1", rearmed)
	}
	c.advance(30 * time.Millisecond)
	if rearmed != 4 {
		t.Fatalf("inner ticker fired %d times total, want 4", rearmed)
	}
}

// A ticker cancelled inside its own callback stops firing immediately, even
// when the advance spans several of its intervals.
func TestAdvanceStopsTickerCancelledInOwnCallback(t *testing.T) {
	c := newFakeClock()
	ctx, cancel := context.WithCancel(context.Background())

	fired := 0
	c.Tick(ctx, 10*time.Millisecond, func(time.Time) {
		fired++
		cancel()
	})

	c.advance(50 * time.Millisecond)
	if fired != 1 {
		t.Fatalf("cancelled ticker fired %d times, want 1", fired)
	}
	c.advance(50 * time.Millisecond)
	if fired != 1 {
		t.Fatalf("cancelled ticker fired again after cancellation (fired=%d)", fired)
	}
}
