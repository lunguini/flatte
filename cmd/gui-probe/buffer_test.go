package main

import (
	"math"
	"testing"
)

func TestEditBufferTyping(t *testing.T) {
	var b editBuffer
	b.Insert("h", noSpan)
	b.Insert("i", noSpan)
	if got := b.Text(); got != "hi" {
		t.Fatalf("Text = %q, want %q", got, "hi")
	}
	if b.HasMarked() || !b.MarkedRange().Missing {
		t.Fatalf("unexpected composition: %+v", b.MarkedRange())
	}
	if got, want := b.SelectedRange(), (span{Loc: 2}); got != want {
		t.Fatalf("SelectedRange = %+v, want %+v", got, want)
	}
}

// TestEditBufferComposition follows a Japanese kana input-method session:
// the composition grows, then the converted text is committed in its place.
func TestEditBufferComposition(t *testing.T) {
	var b editBuffer
	b.Insert("x", noSpan)
	b.SetMarked("k", span{Loc: 1}, noSpan)
	b.SetMarked("か", span{Loc: 1}, noSpan)
	if got := b.Text(); got != "xか" {
		t.Fatalf("Text = %q, want %q", got, "xか")
	}
	if got, want := b.MarkedRange(), (span{Loc: 1, Len: 1}); got != want {
		t.Fatalf("MarkedRange = %+v, want %+v", got, want)
	}
	if got, want := b.SelectedRange(), (span{Loc: 2}); got != want {
		t.Fatalf("SelectedRange = %+v, want %+v", got, want)
	}
	b.Insert("課", noSpan)
	if got := b.Text(); got != "x課" || b.HasMarked() {
		t.Fatalf("after commit Text = %q marked = %v", got, b.HasMarked())
	}
}

func TestEditBufferMarkedSelection(t *testing.T) {
	var b editBuffer
	b.Insert("a", noSpan)
	b.SetMarked("😀bc", span{Loc: 2, Len: 1}, noSpan) // emoji is two UTF-16 units
	if got, want := b.SelectedRange(), (span{Loc: 3, Len: 1}); got != want {
		t.Fatalf("SelectedRange = %+v, want %+v", got, want)
	}
	b.SetMarked("ab", span{Loc: 5, Len: 9}, noSpan) // clamped into the composition
	if got, want := b.SelectedRange(), (span{Loc: 3}); got != want {
		t.Fatalf("clamped SelectedRange = %+v, want %+v", got, want)
	}
	b.SetMarked("ab", span{Loc: -3, Len: 1}, noSpan)
	if got, want := b.SelectedRange(), (span{Loc: 1, Len: 1}); got != want {
		t.Fatalf("negative SelectedRange = %+v, want %+v", got, want)
	}
	b.SetMarked("abc", noSpan, noSpan)
	if got, want := b.SelectedRange(), (span{Loc: 4}); got != want {
		t.Fatalf("missing-selection SelectedRange = %+v, want %+v", got, want)
	}
}

func TestEditBufferCancelAndUnmark(t *testing.T) {
	var b editBuffer
	b.SetMarked("ni", span{Loc: 2}, noSpan)
	b.SetMarked("", span{}, noSpan)
	if b.HasMarked() || b.Text() != "" {
		t.Fatalf("cancel left %q marked=%v", b.Text(), b.HasMarked())
	}
	b.SetMarked("ni", span{Loc: 2}, noSpan)
	b.Unmark()
	if b.HasMarked() || b.Text() != "ni" {
		t.Fatalf("Unmark left %q marked=%v", b.Text(), b.HasMarked())
	}
}

func TestEditBufferReplacementRanges(t *testing.T) {
	var b editBuffer
	b.Insert("hello", noSpan)
	b.Insert("J", span{Loc: 0, Len: 1})
	if got := b.Text(); got != "Jello" {
		t.Fatalf("Insert replace = %q, want %q", got, "Jello")
	}
	b.SetMarked("y", span{Loc: 1}, span{Loc: 4, Len: 1})
	if got := b.Text(); got != "Jelly" || !b.HasMarked() {
		t.Fatalf("SetMarked replace = %q marked=%v", got, b.HasMarked())
	}
	b.Insert("!", span{Loc: 3, Len: math.MaxInt}) // overflowing end clamps
	if got := b.Text(); got != "Jel!" || b.HasMarked() {
		t.Fatalf("overflow replace = %q marked=%v", got, b.HasMarked())
	}
}

func TestEditBufferDeleteBackward(t *testing.T) {
	var b editBuffer
	b.DeleteBackward() // empty is a no-op
	b.Insert("aé😀", noSpan)
	b.DeleteBackward()
	if got := b.Text(); got != "aé" {
		t.Fatalf("Text = %q, want %q", got, "aé")
	}
	b.SetMarked("k", span{Loc: 1}, noSpan)
	b.DeleteBackward() // composition belongs to the input method
	if got := b.Text(); got != "aék" {
		t.Fatalf("Text with composition = %q, want %q", got, "aék")
	}
}

func TestByteOffset(t *testing.T) {
	const s = "a😀b" // UTF-16: a=1, 😀=2, b=1
	for _, tc := range []struct{ u16, want int }{
		{-1, 0}, {0, 0}, {1, 1}, {2, 5}, {3, 5}, {4, 6}, {99, 6},
	} {
		if got := byteOffset(s, tc.u16); got != tc.want {
			t.Errorf("byteOffset(%d) = %d, want %d", tc.u16, got, tc.want)
		}
	}
	if got := utf16Len(s); got != 4 {
		t.Errorf("utf16Len = %d, want 4", got)
	}
}
