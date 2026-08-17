package flatte

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEveryTicksThroughInjectedClock(t *testing.T) {
	clk := &manualClock{}
	updates := make(chan StateUpdate[testState], 8)
	fx := NewHarnessEffects[testState](context.Background(), updates,
		func() {}, func(f func()) { f() }, clk)

	if err := Every(fx, "tick", 10, func(s *testState, _ time.Time) { s.count++ }); err != nil {
		t.Fatalf("Every returned unexpected error: %v", err)
	}
	if len(clk.cbs) != 1 {
		t.Fatalf("registered callbacks = %d, want 1", len(clk.cbs))
	}
	clk.fire(3) // three intervals

	var st testState
	for len(updates) > 0 {
		(<-updates).Apply(&st)
	}
	if st.count != 3 {
		t.Fatalf("count = %d, want 3 ticks", st.count)
	}
}

func TestEveryRejectsNonPositiveIntervalWithoutRegistering(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Nanosecond} {
		t.Run(interval.String(), func(t *testing.T) {
			clk := &manualClock{}
			updates := make(chan StateUpdate[testState], 1)
			fx := NewHarnessEffects[testState](context.Background(), updates,
				func() {}, func(f func()) { f() }, clk)

			err := Every(fx, "tick", interval, func(s *testState, _ time.Time) { s.count++ })
			if !errors.Is(err, ErrInvalidInterval) {
				t.Fatalf("Every error = %v, want ErrInvalidInterval", err)
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

// manualClock fires registered callbacks on demand; no real time.
type manualClock struct{ cbs []func(time.Time) }

func (c *manualClock) Tick(ctx context.Context, _ time.Duration, cb func(time.Time)) {
	c.cbs = append(c.cbs, cb)
}

func (c *manualClock) fire(n int) {
	for i := 0; i < n; i++ {
		for _, cb := range c.cbs {
			cb(time.Time{})
		}
	}
}
