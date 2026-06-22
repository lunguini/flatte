package flatui

import "time"

// Timer is app-owned countdown state. It owns no goroutine and no key policy;
// apps advance it from flatte.Every, tests, or any other loop-owned tick source.
type Timer struct {
	duration time.Duration
	elapsed  time.Duration
}

// NewTimer returns a countdown timer with the given duration. Non-positive
// durations are valid and start already done.
func NewTimer(duration time.Duration) Timer {
	return Timer{duration: maxDuration(duration, 0)}
}

// Tick advances elapsed countdown time. Negative deltas are ignored.
func (t *Timer) Tick(delta time.Duration) {
	if delta <= 0 || t.Done() {
		return
	}
	t.elapsed += delta
	if t.elapsed > t.duration {
		t.elapsed = t.duration
	}
}

// Reset restarts the countdown with its existing duration.
func (t *Timer) Reset() {
	t.elapsed = 0
}

// Duration returns the configured countdown duration.
func (t Timer) Duration() time.Duration { return t.duration }

// Elapsed returns the elapsed countdown time, clamped to Duration.
func (t Timer) Elapsed() time.Duration {
	return minDuration(maxDuration(t.elapsed, 0), t.duration)
}

// Remaining returns the time left until Done.
func (t Timer) Remaining() time.Duration {
	return maxDuration(t.duration-t.Elapsed(), 0)
}

// Percent returns elapsed countdown progress from 0 to 100.
func (t Timer) Percent() float64 {
	if t.duration <= 0 {
		return 100
	}
	return float64(t.Elapsed()) / float64(t.duration) * 100
}

// Done reports whether the countdown has reached its duration.
func (t Timer) Done() bool {
	return t.elapsed >= t.duration
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
