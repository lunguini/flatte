package flatui

import "testing"

func TestFocusRingCyclesAndClamps(t *testing.T) {
	var f FocusRing
	f.SetCount(3)
	if f.Index() != 0 {
		t.Fatalf("initial Index() = %d, want 0", f.Index())
	}
	f.Next()
	f.Next()
	if f.Index() != 2 {
		t.Fatalf("after two Next Index() = %d, want 2", f.Index())
	}
	f.Next()
	if f.Index() != 0 {
		t.Fatalf("wrapped Index() = %d, want 0", f.Index())
	}
	f.Prev()
	if f.Index() != 2 {
		t.Fatalf("Prev wrap Index() = %d, want 2", f.Index())
	}
	f.SetCount(2)
	if f.Index() != 1 {
		t.Fatalf("after shrink Index() = %d, want 1", f.Index())
	}
}

func TestFocusRingSelectAndEmpty(t *testing.T) {
	var f FocusRing
	f.SetCount(0)
	f.Next()
	if f.Index() != 0 {
		t.Fatalf("empty Index() = %d, want 0", f.Index())
	}
	f.SetCount(4)
	f.Select(99)
	if f.Index() != 3 {
		t.Fatalf("clamped Select Index() = %d, want 3", f.Index())
	}
	f.Select(-5)
	if f.Index() != 0 {
		t.Fatalf("negative Select Index() = %d, want 0", f.Index())
	}
}

func TestFocusRingFocused(t *testing.T) {
	var f FocusRing
	if f.Focused(0) {
		t.Fatal("empty FocusRing reports index 0 focused")
	}
	f.SetCount(2)
	if !f.Focused(0) || f.Focused(1) {
		t.Fatalf("Focused mismatch at index %d", f.Index())
	}
}
