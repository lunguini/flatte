package flatui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestViewportSetLinesWindowsVertically(t *testing.T) {
	var v Viewport
	v.SetLines([]string{"a", "b", "c", "d", "e"})
	v.SetSize(10, 3)
	if got := v.View(); got != "a\nb\nc" {
		t.Fatalf("View() = %q, want first 3 lines", got)
	}
}

func TestViewportSetContentSplitsOnNewlines(t *testing.T) {
	var v Viewport
	v.SetContent("one\ntwo\nthree")
	v.SetSize(10, 2)
	if got := v.View(); got != "one\ntwo" {
		t.Fatalf("View() = %q, want first 2 lines", got)
	}
}

func TestViewportSetWrappedContentWrapsToWidth(t *testing.T) {
	var v Viewport
	v.SetSize(10, 5)
	v.SetWrappedContent("abcdefghijABCDEFGHIJ") // 20 cells, width 10
	for _, line := range strings.Split(v.View(), "\n") {
		if w := ansi.StringWidth(line); w > 10 {
			t.Fatalf("wrapped line %q width %d exceeds 10", line, w)
		}
	}
	if v.TotalLines() != 2 {
		t.Fatalf("TotalLines() = %d, want 2 wrapped lines", v.TotalLines())
	}
}

func TestViewportDeferredWrapUntilSize(t *testing.T) {
	var v Viewport
	v.SetWrappedContent("abcdefghijABCDEFGHIJ") // no size yet
	if v.View() != "" {
		t.Fatalf("View() before SetSize = %q, want empty (wrap deferred)", v.View())
	}
	v.SetSize(10, 5)
	if v.TotalLines() != 2 {
		t.Fatalf("TotalLines() after SetSize = %d, want 2", v.TotalLines())
	}
}

func TestViewportClipsUnwrappedLinesAtWidth(t *testing.T) {
	var v Viewport
	v.SetLines([]string{"abcdefghijklmnop"}) // 16 cells
	v.SetSize(5, 1)
	if got := v.View(); ansi.StringWidth(got) > 5 {
		t.Fatalf("View() = %q width %d, want clipped to 5", got, ansi.StringWidth(got))
	}
}

func TestViewportReWrapsOnWidthChange(t *testing.T) {
	var v Viewport
	v.SetSize(20, 5)
	v.SetWrappedContent("abcdefghijABCDEFGHIJ") // fits on one 20-wide line
	if v.TotalLines() != 1 {
		t.Fatalf("TotalLines() at width 20 = %d, want 1", v.TotalLines())
	}
	v.SetSize(10, 5) // narrower → must re-wrap to 2 lines
	if v.TotalLines() != 2 {
		t.Fatalf("TotalLines() at width 10 = %d, want 2", v.TotalLines())
	}
}
