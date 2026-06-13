package flatcore

import (
	"context"
	"testing"
	"time"
)

func TestEveryTicksThroughInjectedClock(t *testing.T) {
	clk := &manualClock{}
	updates := make(chan StateUpdate[testState], 8)
	fx := NewHarnessEffects[testState](context.Background(), updates,
		func() {}, func(f func()) { f() }, clk)

	Every(fx, "tick", 10, func(s *testState, _ time.Time) { s.count++ })
	clk.fire(3) // three intervals

	var st testState
	for len(updates) > 0 {
		(<-updates).Apply(&st)
	}
	if st.count != 3 {
		t.Fatalf("count = %d, want 3 ticks", st.count)
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
