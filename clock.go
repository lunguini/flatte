package flatte

import (
	"context"
	"time"
)

// Clock abstracts the timing source for interval effects so tests can
// drive them deterministically. The real clock uses time.Ticker; flatest
// provides a fake one.
type Clock interface {
	// Tick calls cb on each interval until ctx is cancelled. Real
	// implementations own a goroutine; fake ones fire synchronously.
	Tick(ctx context.Context, interval time.Duration, cb func(time.Time))
}

type realClock struct{}

func (realClock) Tick(ctx context.Context, interval time.Duration, cb func(time.Time)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				cb(now)
			}
		}
	}()
}
