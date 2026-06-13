package flatui

import "testing"

func TestSpinnerCyclesFramesWithWrap(t *testing.T) {
	s := NewSpinner(SpinnerLine) // | / - \
	want := []string{"|", "/", "-", "\\", "|"}
	if s.View() != want[0] {
		t.Fatalf("initial View() = %q, want %q", s.View(), want[0])
	}
	for i := 1; i < len(want); i++ {
		s.Tick()
		if s.View() != want[i] {
			t.Fatalf("after %d ticks View() = %q, want %q", i, s.View(), want[i])
		}
	}
	if s.Frame() != 0 {
		t.Fatalf("after a full cycle Frame() = %d, want 0 (wrapped)", s.Frame())
	}
}

func TestSpinnerZeroValueIsSafe(t *testing.T) {
	var s Spinner // no frames
	if s.View() != "" {
		t.Fatalf("zero View() = %q, want empty", s.View())
	}
	s.Tick() // must not panic or move
	if s.View() != "" {
		t.Fatalf("zero View() after Tick = %q, want empty", s.View())
	}
}

func TestSpinnerDotsPresetIsNonEmpty(t *testing.T) {
	s := NewSpinner(SpinnerDots)
	if s.View() == "" {
		t.Fatal("SpinnerDots View() is empty")
	}
	if len(SpinnerDots) < 2 {
		t.Fatalf("SpinnerDots has %d frames, want >= 2", len(SpinnerDots))
	}
}

func TestNewSpinnerCopiesFrames(t *testing.T) {
	frames := []string{"a", "b"}
	s := NewSpinner(frames)
	frames[0] = "X" // mutate caller's slice
	if s.View() != "a" {
		t.Fatalf("View() = %q, want a — NewSpinner must copy its frames", s.View())
	}
}
