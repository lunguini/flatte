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

func tenLineViewport() Viewport {
	var v Viewport
	v.SetLines([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"})
	v.SetSize(10, 3)
	return v
}

func TestViewportLineScroll(t *testing.T) {
	v := tenLineViewport()
	v.LineDown(2)
	if got := v.View(); got != "2\n3\n4" {
		t.Fatalf("after LineDown(2) View() = %q, want 2\\n3\\n4", got)
	}
	v.LineUp(1)
	if got := v.View(); got != "1\n2\n3" {
		t.Fatalf("after LineUp(1) View() = %q, want 1\\n2\\n3", got)
	}
}

func TestViewportPaging(t *testing.T) {
	v := tenLineViewport() // height 3
	v.HalfPageDown()       // 3/2 = 1
	if v.Offset() != 1 {
		t.Fatalf("after HalfPageDown Offset() = %d, want 1", v.Offset())
	}
	v.PageDown() // +3 -> 4
	if v.Offset() != 4 {
		t.Fatalf("after PageDown Offset() = %d, want 4", v.Offset())
	}
}

func TestViewportGotoTopBottom(t *testing.T) {
	v := tenLineViewport()
	v.GotoBottom()
	if !v.AtBottom() || v.Offset() != 7 { // maxOffset = 10 - 3 = 7
		t.Fatalf("GotoBottom: offset=%d atBottom=%v, want 7/true", v.Offset(), v.AtBottom())
	}
	v.GotoTop()
	if !v.AtTop() || v.Offset() != 0 {
		t.Fatalf("GotoTop: offset=%d atTop=%v, want 0/true", v.Offset(), v.AtTop())
	}
}

func TestViewportClampsAtBounds(t *testing.T) {
	v := tenLineViewport()
	v.LineDown(1000)
	if v.Offset() != 7 || !v.AtBottom() {
		t.Fatalf("over-scroll down: offset=%d, want clamped to 7", v.Offset())
	}
	v.LineUp(1000)
	if v.Offset() != 0 || !v.AtTop() {
		t.Fatalf("over-scroll up: offset=%d, want clamped to 0", v.Offset())
	}
}

func TestViewportClampOnContentShrink(t *testing.T) {
	v := tenLineViewport()
	v.GotoBottom() // offset 7
	v.SetLines([]string{"x", "y"})
	if v.Offset() != 0 || !v.AtBottom() {
		t.Fatalf("after shrink: offset=%d atBottom=%v, want 0/true", v.Offset(), v.AtBottom())
	}
}

func TestViewportScrollPercent(t *testing.T) {
	v := tenLineViewport()
	if v.ScrollPercent() != 0.0 {
		t.Fatalf("at top ScrollPercent = %v, want 0", v.ScrollPercent())
	}
	v.GotoBottom()
	if v.ScrollPercent() != 1.0 {
		t.Fatalf("at bottom ScrollPercent = %v, want 1", v.ScrollPercent())
	}
	var fits Viewport
	fits.SetLines([]string{"a", "b"})
	fits.SetSize(10, 5) // everything visible
	if fits.ScrollPercent() != 1.0 {
		t.Fatalf("content fits ScrollPercent = %v, want 1", fits.ScrollPercent())
	}
}

func TestViewportVisibleLines(t *testing.T) {
	v := tenLineViewport()
	if v.VisibleLines() != 3 {
		t.Fatalf("VisibleLines = %d, want 3", v.VisibleLines())
	}
	v.GotoBottom()
	if v.VisibleLines() != 3 {
		t.Fatalf("VisibleLines at bottom = %d, want 3", v.VisibleLines())
	}
	var short Viewport
	short.SetLines([]string{"a"})
	short.SetSize(10, 4)
	if short.VisibleLines() != 1 {
		t.Fatalf("VisibleLines short = %d, want 1", short.VisibleLines())
	}
}
