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
// interval elapsed. Cancelled tickers are skipped and dropped — including a
// ticker that cancels itself mid-burst, which stops firing immediately.
//
// Callbacks may register new tickers (the "cancel + re-arm an Every from
// inside a fold" pattern): iteration runs over a snapshot while Tick appends
// to a fresh slice, so registrations made during an advance are kept but do
// not fire until the next advance — they were born "now" and no time has
// passed for them yet.
func (c *fakeClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
	snapshot := c.tickers
	c.tickers = nil // Tick calls from callbacks land here
	var live []*fakeTicker
	for _, t := range snapshot {
		if t.ctx.Err() != nil {
			continue
		}
		t.acc += d
		for t.interval > 0 && t.acc >= t.interval && t.ctx.Err() == nil {
			t.acc -= t.interval
			t.cb(c.now)
		}
		if t.ctx.Err() == nil {
			live = append(live, t)
		}
	}
	// Survivors keep their original order, followed by tickers registered
	// during this advance.
	c.tickers = append(live, c.tickers...)
}
