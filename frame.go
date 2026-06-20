package flat

import "image/color"

// Frame is what View returns: rendered content plus terminal metadata.
// The zero value is a blank frame with no cursor and no title.
type Frame struct {
	// Content is the styled frame text.
	Content string
	// Cursor places the hardware cursor, in frame cell coordinates
	// ((0,0) is the frame's top-left). nil hides the cursor.
	Cursor *Cursor
	// Title sets the terminal window title when non-empty. It is emitted
	// only when it changes, and reset on exit if it was ever set.
	Title string
}

// Cursor is a hardware-cursor position in frame cell coordinates.
type Cursor struct {
	X, Y  int
	Style *CursorStyle
}

type CursorShape int

const (
	CursorShapeDefault CursorShape = iota
	CursorShapeBlock
	CursorShapeUnderline
	CursorShapeBar
)

// CursorStyle configures the terminal hardware cursor when supported. A nil
// style leaves the terminal default in place.
type CursorStyle struct {
	Shape CursorShape
	Blink bool
	Color color.Color
}

// framesEqual reports whether two frames would render identically,
// comparing the cursor by value rather than by pointer.
func framesEqual(a, b Frame) bool {
	if a.Content != b.Content || a.Title != b.Title {
		return false
	}
	return cursorsEqual(a.Cursor, b.Cursor)
}

func cursorsEqual(a, b *Cursor) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.X != b.X || a.Y != b.Y {
		return false
	}
	return cursorStylesEqual(a.Style, b.Style)
}

func cursorStylesEqual(a, b *CursorStyle) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Shape != b.Shape || a.Blink != b.Blink {
		return false
	}
	return colorsEqual(a.Color, b.Color)
}

func colorsEqual(a, b color.Color) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
