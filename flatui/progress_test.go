package flatui

import "testing"

func TestProgressRendersFilledCellsAndPercent(t *testing.T) {
	p := NewProgress(10)
	p.SetPercent(42)

	if got, want := p.View(), "████░░░░░░   42%"; got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestProgressClampsPercent(t *testing.T) {
	p := NewProgress(5)

	p.SetPercent(-20)
	if got, want := p.View(), "░░░░░    0%"; got != want {
		t.Fatalf("negative percent View() = %q, want %q", got, want)
	}

	p.SetPercent(140)
	if got, want := p.View(), "█████  100%"; got != want {
		t.Fatalf("overfull percent View() = %q, want %q", got, want)
	}
}

func TestProgressWidthCanBeChanged(t *testing.T) {
	p := NewProgress(4)
	p.SetPercent(50)
	p.SetWidth(8)

	if got, want := p.View(), "████░░░░   50%"; got != want {
		t.Fatalf("View() after SetWidth = %q, want %q", got, want)
	}
}

func TestProgressZeroValueIsSafe(t *testing.T) {
	var p Progress
	p.SetPercent(75)

	if got, want := p.View(), " 75%"; got != want {
		t.Fatalf("zero-width View() = %q, want %q", got, want)
	}
}
