package flatui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Viewport is a scrollable window over content. The app owns it (like
// TextField): no goroutines, no key policy. Feed it content with one of the
// Set* methods, size it with SetSize, scroll it with the movement methods
// (see below), and render the visible slice with View. View returns a plain
// string, so the widget works under alt-screen or inline rendering alike —
// the scroll model does not assume alt-screen.
type Viewport struct {
	lines   []string // content already split into windowable lines
	content string   // source kept for re-wrapping on resize (wrapped mode)
	wrapped bool     // whether content is soft-wrapped to width
	width   int
	height  int
	offset  int // index of the top visible line
}

// SetLines sets pre-laid-out lines verbatim: no splitting, no wrapping. Lines
// wider than the viewport are clipped at its width on render.
func (v *Viewport) SetLines(lines []string) {
	v.lines = append([]string(nil), lines...)
	v.content = ""
	v.wrapped = false
	v.clamp()
}

// SetContent splits content on newlines and windows it without wrapping; long
// lines are clipped at the viewport width on render.
func (v *Viewport) SetContent(content string) {
	v.content = content
	v.wrapped = false
	v.lines = strings.Split(content, "\n")
	v.clamp()
}

// SetWrappedContent splits content on newlines and soft-wraps each line to the
// viewport width — ANSI- and display-width-aware, so SGR styles and wide runes
// are never split. Re-wraps automatically when the width changes via SetSize.
// If the width is not yet known, the wrap is deferred until SetSize provides
// one.
func (v *Viewport) SetWrappedContent(content string) {
	v.content = content
	v.wrapped = true
	v.relayout()
}

// SetSize sets the visible window dimensions. Width drives wrapping and
// clipping; height caps the number of rows View emits.
func (v *Viewport) SetSize(width, height int) {
	v.width, v.height = width, height
	v.relayout()
}

// relayout recomputes wrapped lines when in wrapped mode, then clamps.
func (v *Viewport) relayout() {
	if v.wrapped {
		if v.width > 0 {
			v.lines = strings.Split(ansi.Hardwrap(v.content, v.width, false), "\n")
		} else {
			v.lines = nil
		}
	}
	v.clamp()
}

// View returns the visible slice joined by newlines. Lines wider than the
// width are clipped (a no-op for already-wrapped content). Lines are not
// padded to the width or the height — the caller owns the surrounding frame.
func (v Viewport) View() string {
	if v.height <= 0 || len(v.lines) == 0 {
		return ""
	}
	off := min(max(v.offset, 0), len(v.lines))
	end := min(off+v.height, len(v.lines))
	visible := v.lines[off:end]
	if v.width <= 0 {
		return strings.Join(visible, "\n")
	}
	out := make([]string, len(visible))
	for i, line := range visible {
		out[i] = ansi.Truncate(line, v.width, "")
	}
	return strings.Join(out, "\n")
}

// maxOffset is the largest valid top-line index.
func (v Viewport) maxOffset() int {
	return max(0, len(v.lines)-v.height)
}

func (v *Viewport) clamp() {
	v.offset = min(max(v.offset, 0), v.maxOffset())
}

// TotalLines is the number of content lines after ingestion/wrapping.
func (v Viewport) TotalLines() int { return len(v.lines) }

// LineDown scrolls down by n lines (clamped). LineUp scrolls up.
func (v *Viewport) LineDown(n int) { v.offset += n; v.clamp() }
func (v *Viewport) LineUp(n int)   { v.offset -= n; v.clamp() }

// HalfPageDown/Up and PageDown/Up scroll by half / a full visible window.
func (v *Viewport) HalfPageDown() { v.LineDown(v.height / 2) }
func (v *Viewport) HalfPageUp()   { v.LineUp(v.height / 2) }
func (v *Viewport) PageDown()     { v.LineDown(v.height) }
func (v *Viewport) PageUp()       { v.LineUp(v.height) }

// GotoTop/GotoBottom jump to the first/last window.
func (v *Viewport) GotoTop()    { v.offset = 0 }
func (v *Viewport) GotoBottom() { v.offset = v.maxOffset() }

// ScrollToLine positions the window so line i is at the top (clamped).
func (v *Viewport) ScrollToLine(i int) { v.offset = i; v.clamp() }

// AtTop / AtBottom report whether the window is at an extreme.
func (v Viewport) AtTop() bool    { return v.offset <= 0 }
func (v Viewport) AtBottom() bool { return v.offset >= v.maxOffset() }

// ScrollPercent is the fraction scrolled, 0.0 at top to 1.0 at bottom. When
// all content fits (nothing to scroll), it reports 1.0.
func (v Viewport) ScrollPercent() float64 {
	maxOff := v.maxOffset()
	if maxOff <= 0 {
		return 1.0
	}
	return float64(v.offset) / float64(maxOff)
}

// Offset is the current top-line index.
func (v Viewport) Offset() int { return v.offset }

// VisibleLines is how many content rows View currently emits.
func (v Viewport) VisibleLines() int {
	n := min(len(v.lines)-v.offset, v.height)
	return max(n, 0)
}
