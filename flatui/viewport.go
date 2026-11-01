package flatui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
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
	// id and hostLines come from Adopt: the ID the app gave the viewport's
	// space, and the content's line count as the host laid it out (0: the
	// viewport's own).
	id        string
	hostLines int
	// pinned records that the view was sent to the bottom (GotoBottom, or a
	// scroll that ended there): Adopt keeps it there as the line count
	// changes.
	pinned bool
}

// contentSuffix names the content node of an adopted viewport's Layout:
// its ID is the adopted ID plus this.
const contentSuffix = ":content"

// Adopt sizes the viewport from rects[id] — the space the app gave it, an
// ID'd node holding Layout() — and scrolls over as many lines as the host
// laid the content out in, which a window that wraps in its font can make
// more or fewer than the viewport's own: the offset range, TotalLines,
// AtBottom, and ScrollPercent follow the host. Call it with every
// LayoutEvent's rects instead of SetSize. A viewport sent to the bottom
// (GotoBottom, or scrolled there) stays at the bottom as lines come and go.
func (v *Viewport) Adopt(rects map[string]layout.Rect, id string) {
	v.id = id
	r, ok := rects[id]
	if !ok {
		// Not laid out (say, on a hidden tab): no host count to follow.
		v.hostLines = 0
		v.clamp()
		return
	}
	v.SetSize(max(r.W, 1), max(r.H, 1))
	v.hostLines = 0
	if c, ok := rects[id+contentSuffix]; ok {
		v.hostLines = max(c.H, 0)
	}
	if v.pinned {
		v.offset = v.maxOffset()
	} else {
		v.clamp()
	}
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

// SetWrappedContent splits content on newlines and wraps each line by words
// to the viewport width (a word wider than the width breaks) — ANSI- and
// display-width-aware, so SGR styles and wide runes are never split. Re-wraps automatically when the width changes via SetSize.
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
	v.width, v.height = clampWidgetDimension(width), clampWidgetDimension(height)
	v.relayout()
}

// relayout recomputes wrapped lines when in wrapped mode, then clamps.
func (v *Viewport) relayout() {
	if v.wrapped {
		if v.width > 0 {
			v.lines = strings.Split(ansi.Wrap(v.content, v.width, ""), "\n")
		} else {
			v.lines = nil
		}
	}
	v.clamp()
}

// Layout returns the viewport as a layout.Scroll over all its lines, offset
// to the current top line and as tall as the visible slice (and as wide as
// the viewport's width, when set): View's text in the terminal, scrolled in
// points in a window. Plain lines are a Label, drawn natively in a window —
// for wrapped content, a wrapping Label of the raw text, so a window wraps it
// in its font; lines with escape sequences are a Text of cells.
func (v Viewport) Layout() layout.Node {
	if v.VisibleLines() == 0 {
		return layout.Text{}
	}
	content := strings.Join(v.lines, "\n")
	var child layout.Node = layout.Label{Text: content}
	switch {
	case strings.Contains(content, "\x1b"):
		child = layout.Text{String: content}
	case v.wrapped && v.width > 0:
		child = layout.Label{Text: v.content, Wrap: true}
	}
	w := layout.Auto()
	if v.width > 0 {
		w = layout.Fixed(v.width)
	}
	if v.id != "" {
		child = withID(child, v.id+contentSuffix)
	}
	return layout.Scroll{
		NodeBase: layout.NodeBase{W: w, H: layout.Fixed(v.VisibleLines())},
		OffsetY:  v.offset,
		Child:    child,
	}
}

// withID is n (a Label or Text) with ID id.
func withID(n layout.Node, id string) layout.Node {
	switch n := n.(type) {
	case layout.Label:
		n.ID = id
		return n
	case layout.Text:
		n.ID = id
		return n
	}
	return n
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
	return max(0, v.TotalLines()-v.height)
}

func (v *Viewport) clamp() {
	v.offset = min(max(v.offset, 0), v.maxOffset())
}

// TotalLines is the number of content lines after ingestion/wrapping — as
// the host laid them out, once adopted (see Adopt).
func (v Viewport) TotalLines() int {
	if v.hostLines > 0 {
		return v.hostLines
	}
	return len(v.lines)
}

// LineDown scrolls down by n lines (clamped). LineUp scrolls up.
func (v *Viewport) LineDown(n int) { v.scrollTo(v.offset + n) }
func (v *Viewport) LineUp(n int)   { v.scrollTo(v.offset - n) }

// scrollTo moves the top line to i (clamped), pinning the view to the bottom
// when it lands there with something to scroll.
func (v *Viewport) scrollTo(i int) {
	v.offset = i
	v.clamp()
	v.pinned = v.maxOffset() > 0 && v.offset == v.maxOffset()
}

// HalfPageDown/Up and PageDown/Up scroll by half / a full visible window.
func (v *Viewport) HalfPageDown() { v.LineDown(v.height / 2) }
func (v *Viewport) HalfPageUp()   { v.LineUp(v.height / 2) }
func (v *Viewport) PageDown()     { v.LineDown(v.height) }
func (v *Viewport) PageUp()       { v.LineUp(v.height) }

// GotoTop/GotoBottom jump to the first/last window.
func (v *Viewport) GotoTop()    { v.offset, v.pinned = 0, false }
func (v *Viewport) GotoBottom() { v.offset, v.pinned = v.maxOffset(), true }

// ScrollToLine positions the window so line i is at the top (clamped).
func (v *Viewport) ScrollToLine(i int) { v.scrollTo(i) }

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
	n := min(v.TotalLines()-v.offset, v.height)
	return max(n, 0)
}
