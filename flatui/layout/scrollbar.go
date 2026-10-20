package layout

import (
	"image/color"
	"strings"
)

// ScrollBar shows where a scrolled window is: Visible of Total rows of
// content are shown, starting at row Offset. The terminal draws a thumb of
// █ on a track of ░ in Color, one glyph per line, and blanks when the
// content fits; a GUI draws a native thin thumb and nothing when it fits.
// Nil Color is the backend's default. The app owns the numbers (a List's
// Offset, Height and Count; a Viewport's Offset, VisibleLines and
// TotalLines). On Auto axes it is one column wide and fills the height.
type ScrollBar struct {
	NodeBase
	Offset, Visible, Total int
	Color                  color.Color
}

func (b ScrollBar) Size() (Size, Size) {
	w, h := b.NodeBase.Size()
	if w.Kind == SizeAuto {
		w = Size{Kind: SizeContent, Value: satAdd(1, b.insetSides().horiz())}
	}
	if h.Kind == SizeAuto {
		h = Grow(1)
	}
	return w, h
}

func (b ScrollBar) Render(r Rect) string {
	inner := r.insetBy(b.insetSides())
	if inner.W <= 0 || inner.H <= 0 {
		return fillBox("", r, b.padSides(), b.box())
	}
	pos, size, ok := b.thumb(inner.H)
	rows := make([]string, inner.H)
	for i := range rows {
		switch {
		case !ok:
			rows[i] = " "
		case i >= pos && i < pos+size:
			rows[i] = "█"
		default:
			rows[i] = "░"
		}
	}
	bar := strings.Join(rows, "\n")
	if ok {
		bar = tinted(bar, b.Color)
	}
	return fillBox(bar, r, b.padSides(), b.box())
}

// thumb is the thumb's first line and length on a track height lines
// long, or ok false when the content fits (nothing to scroll).
func (b ScrollBar) thumb(height int) (pos, size int, ok bool) {
	if b.Total <= b.Visible || height <= 0 || b.Visible <= 0 {
		return 0, 0, false
	}
	size = min(max(1, height*b.Visible/b.Total), height)
	pos = int(float64(b.Offset) / float64(b.Total-b.Visible) * float64(height-size))
	return min(max(pos, 0), height-size), size, true
}

// fraction is the thumb as fractions of the track, its start and length,
// for a backend that draws in points; ok is false when the content fits.
func (b ScrollBar) fraction() (start, length float64, ok bool) {
	if b.Total <= b.Visible || b.Visible <= 0 {
		return 0, 0, false
	}
	offset := min(max(b.Offset, 0), b.Total-b.Visible)
	return float64(offset) / float64(b.Total), float64(b.Visible) / float64(b.Total), true
}

// OffsetAt is the offset a press or drag at line y of the bar asks for, on a
// bar height lines tall (y and height as in its rect, so y is node-local):
// the one whose thumb is centered nearest y, so the top line scrolls to the
// start and the bottom line to the end. It is Offset unchanged when there
// is nothing to scroll.
func (b ScrollBar) OffsetAt(y, height int) int {
	_, size, ok := b.thumb(height)
	if !ok {
		return b.Offset
	}
	track, scrollable := height-size, b.Total-b.Visible
	if track <= 0 {
		return min(max(b.Offset, 0), scrollable)
	}
	top := min(max(y-size/2, 0), track)
	// The thumb's line rounds down from offset/scrollable×track: start from
	// the rounded inverse and take the nearest of its neighbours.
	best, bestD := 0, track+1
	guess := (top*scrollable + track/2) / track
	for off := max(guess-1, 0); off <= min(guess+1, scrollable); off++ {
		c := b
		c.Offset = off
		pos, _, _ := c.thumb(height)
		if d := max(pos-top, top-pos); d < bestD {
			best, bestD = off, d
		}
	}
	return best
}
