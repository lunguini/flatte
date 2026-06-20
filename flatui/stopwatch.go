package flatui

import "time"

// Stopwatch is app-owned elapsed-time state. It owns no goroutine and no key
// policy; apps decide how ticks are produced and which keys start or stop it.
type Stopwatch struct {
	elapsed time.Duration
	running bool
}

// NewStopwatch returns a stopped stopwatch at zero elapsed time.
func NewStopwatch() Stopwatch {
	return Stopwatch{}
}

// Start makes subsequent Tick calls advance elapsed time.
func (s *Stopwatch) Start() {
	s.running = true
}

// Stop pauses elapsed time.
func (s *Stopwatch) Stop() {
	s.running = false
}

// Toggle switches between running and stopped.
func (s *Stopwatch) Toggle() {
	s.running = !s.running
}

// Reset clears elapsed time and stops the stopwatch.
func (s *Stopwatch) Reset() {
	s.elapsed = 0
	s.running = false
}

// Tick advances elapsed time only while the stopwatch is running. Negative
// deltas are ignored.
func (s *Stopwatch) Tick(delta time.Duration) {
	if !s.running || delta <= 0 {
		return
	}
	s.elapsed += delta
}

// Elapsed returns elapsed stopwatch time.
func (s Stopwatch) Elapsed() time.Duration { return s.elapsed }

// Running reports whether Tick currently advances elapsed time.
func (s Stopwatch) Running() bool { return s.running }
