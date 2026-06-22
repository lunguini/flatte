package flatest

import (
	"context"
	"time"
)

// fakeTicker is one Every registration: its callback fires once per whole
// interval of advanced time.
type fakeTicker struct {
	interval time.Duration
	cb       func(time.Time)
	ctx      context.Context
	acc      time.Duration
}

// fakeClock implements flatte.Clock for deterministic Every ticks: no
// goroutine, no real time — advance fires due callbacks synchronously.
type fakeClock struct {
	now     time.Time
	tickers []*fakeTicker
}

func newFakeClock() *fakeClock { return &fakeClock{} }

func (c *fakeClock) Tick(ctx context.Context, interval time.Duration, cb func(time.Time)) {
	c.tickers = append(c.tickers, &fakeTicker{interval: interval, cb: cb, ctx: ctx})
}

// advance moves time forward, firing each live ticker once per whole
// interval elapsed. Cancelled tickers are skipped and dropped.
func (c *fakeClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
	live := c.tickers[:0]
	for _, t := range c.tickers {
		if t.ctx.Err() != nil {
			continue
		}
		t.acc += d
		for t.interval > 0 && t.acc >= t.interval {
			t.acc -= t.interval
			t.cb(c.now)
		}
		live = append(live, t)
	}
	c.tickers = live
}
