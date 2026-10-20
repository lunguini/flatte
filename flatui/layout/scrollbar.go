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
