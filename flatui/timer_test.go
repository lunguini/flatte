package flatui

import (
	"testing"
	"time"
)

func TestTimerTicksTowardDone(t *testing.T) {
	timer := NewTimer(5 * time.Second)

	timer.Tick(2 * time.Second)
	if got := timer.Remaining(); got != 3*time.Second {
		t.Fatalf("Remaining() = %s, want 3s", got)
	}
	if got := timer.Percent(); got != 40 {
		t.Fatalf("Percent() = %.0f, want 40", got)
	}
	if timer.Done() {
		t.Fatal("Done() = true, want false")
	}

	timer.Tick(3 * time.Second)
	if !timer.Done() {
		t.Fatal("Done() = false, want true")
	}
	if got := timer.Remaining(); got != 0 {
		t.Fatalf("Remaining() after done = %s, want 0", got)
	}
	if got := timer.Percent(); got != 100 {
		t.Fatalf("Percent() after done = %.0f, want 100", got)
	}
}

func TestTimerResetRestartsCountdown(t *testing.T) {
	timer := NewTimer(5 * time.Second)
	timer.Tick(5 * time.Second)
	timer.Reset()

	if timer.Done() {
		t.Fatal("Done() after reset = true, want false")
	}
	if got := timer.Remaining(); got != 5*time.Second {
		t.Fatalf("Remaining() after reset = %s, want 5s", got)
	}
}

func TestTimerClampsNegativeTicks(t *testing.T) {
	timer := NewTimer(5 * time.Second)
	timer.Tick(-time.Second)

	if got := timer.Remaining(); got != 5*time.Second {
		t.Fatalf("Remaining() after negative tick = %s, want 5s", got)
	}
}
