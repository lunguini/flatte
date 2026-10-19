package flatte

import (
	"image/color"

	"github.com/lunguini/flatte/flatui/layout"
)

// Frame is what View returns: rendered content plus terminal metadata.
// The zero value is a blank frame with no cursor and no title.
type Frame struct {
	// Content is the styled frame text. It is ignored when Root is set.
	Content string
	// Cursor places the hardware cursor, in frame cell coordinates
	// ((0,0) is the frame's top-left). nil hides the cursor.
	Cursor *Cursor
	// Title sets the terminal window title when non-empty. It is emitted
	// only when it changes, and reset on exit if it was ever set.
	Title string
	// Root, when set, is the frame's layout tree. The host solves and paints
	// it at the frame size, so View needs no Solve call or size bookkeeping,
	// and MouseEvent.Target names the ID'd node under the pointer. Frames
	// whose Root holds a container (Row, Col) are not comparable with ==.
	Root layout.Node
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

// ComposeFrame resolves frame.Root, when set, into frame.Content at
// width×height and returns the composition for hit-testing. A frame without
// Root is returned unchanged with an empty composition. Run calls it before
// every draw; test harnesses call it to render exactly as Run does.
func ComposeFrame(frame Frame, width, height int) (Frame, layout.Composition) {
	if frame.Root == nil {
		return frame, layout.Composition{}
	}
	c := layout.Compose(frame.Root, width, height)
	frame.Content = c.Frame
	return frame, c
}

// rootHeight is the height a Root frame is composed at. Alt-screen frames
// fill the terminal. Inline frames own only their own lines, so they take the
// root's fixed or measured height, clamped to the terminal; a root that grows
// or has no measurable height fills the terminal.
func rootHeight(root layout.Node, terminalHeight int, inline bool) int {
	if !inline {
		return terminalHeight
	}
	_, h := root.Size()
	if (h.Kind == layout.SizeFixed || h.Kind == layout.SizeContent) && h.Value < terminalHeight {
		return max(h.Value, 0)
	}
	return terminalHeight
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
