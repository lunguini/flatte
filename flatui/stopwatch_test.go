package flatui

import (
	"testing"
	"time"
)

func TestStopwatchTicksOnlyWhileRunning(t *testing.T) {
	sw := NewStopwatch()

	sw.Start()
	sw.Tick(1200 * time.Millisecond)
	if got := sw.Elapsed(); got != 1200*time.Millisecond {
		t.Fatalf("Elapsed() = %s, want 1.2s", got)
	}
	if !sw.Running() {
		t.Fatal("Running() = false, want true")
	}

	sw.Stop()
	sw.Tick(time.Second)
	if got := sw.Elapsed(); got != 1200*time.Millisecond {
		t.Fatalf("Elapsed() after stop = %s, want 1.2s", got)
	}
	if sw.Running() {
		t.Fatal("Running() after stop = true, want false")
	}

	sw.Reset()
	if got := sw.Elapsed(); got != 0 {
		t.Fatalf("Elapsed() after reset = %s, want 0", got)
	}
}

func TestStopwatchToggleAndReset(t *testing.T) {
	sw := NewStopwatch()

	sw.Toggle()
	if !sw.Running() {
		t.Fatal("Running() after toggle = false, want true")
	}
	sw.Tick(time.Second)
	sw.Toggle()
	sw.Tick(time.Second)
	if got := sw.Elapsed(); got != time.Second {
		t.Fatalf("Elapsed() after toggled stop = %s, want 1s", got)
	}
	sw.Reset()
	if sw.Running() {
		t.Fatal("Running() after reset = true, want false")
	}
}
