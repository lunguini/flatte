package flatte

import (
	"image/color"
	"maps"

	"github.com/charmbracelet/colorprofile"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/internal/sessionaccess"
)

// Frame is what View returns: rendered content plus terminal metadata.
// The zero value is a blank frame with no cursor and no title.
type Frame struct {
	// Content is the styled frame text. It is ignored when Root is set.
	Content string
	// Cursor places the hardware cursor, in frame cell coordinates
	// ((0,0) is the frame's top-left). nil hides the cursor — unless Root
	// holds a focused layout.Input, whose caret then places it.
	Cursor *Cursor
	// CursorStyle styles the cursor a focused layout.Input's caret places
	// (when Cursor is nil); nil is the terminal's default.
	CursorStyle *CursorStyle
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
// width×height and returns the composition for hit-testing. A nil Cursor
// is placed at the caret of the topmost focused layout.Input, if any. A
// frame without Root is returned unchanged with an empty composition. Run
// calls it before every draw; test harnesses call it to render exactly as
// Run does.
func ComposeFrame(frame Frame, width, height int) (Frame, layout.Composition) {
	return composeFrameWithColorProfile(frame, width, height, colorprofile.Unknown)
}

func composeFrameWithColorProfile(frame Frame, width, height int, profile colorprofile.Profile) (Frame, layout.Composition) {
	if frame.Root == nil {
		return frame, layout.Composition{}
	}
	c := layout.ComposeWithColorProfile(frame.Root, width, height, profile)
	frame.Content = c.Frame
	if frame.Cursor == nil {
		if x, y, ok := c.Cursor(); ok {
			frame.Cursor = &Cursor{X: x, Y: y, Style: frame.CursorStyle}
		}
	}
	return frame, c
}

// maxLayoutPasses bounds how many LayoutEvents one render delivers. Two
// covers the common chain (a pane's size sets a list's rows, which moves the
// row rects). Geometry still changing after that is painted as-is and
// re-delivered on the next render, so a non-converging app cannot stall.
const maxLayoutPasses = 2

func init() {
	sessionaccess.RegisterHelpers(sessionaccess.Helpers{
		SettleFrame: func(view, handle any, last map[string]layout.Rect, width, height int) (any, layout.Composition, map[string]layout.Rect) {
			var deliver func(LayoutEvent)
			if handle != nil {
				deliver = handle.(func(LayoutEvent))
			}
			return settleFrame(view.(func() Frame), deliver, last, width, height)
		},
		MediaOption: func(commands, publish any) any {
			return withHostMedia(commands.(<-chan MediaEvent), publish.(func(*NowPlaying)))
		},
	})
}

func settleFrame(view func() Frame, handle func(LayoutEvent), last map[string]layout.Rect, width, height int) (Frame, layout.Composition, map[string]layout.Rect) {
	return settleFrameWithHeightProfile(view, handle, last, width, func(layout.Node) int { return height }, colorprofile.Unknown)
}

func settleFrameWithProfile(view func() Frame, handle func(LayoutEvent), last map[string]layout.Rect, width, height int, profile colorprofile.Profile) (Frame, layout.Composition, map[string]layout.Rect) {
	return settleFrameWithHeightProfile(view, handle, last, width, func(layout.Node) int { return height }, profile)
}

// settleFrameWithHeight supports inline frames, whose height comes from the
// root's measured content.
func settleFrameWithHeight(view func() Frame, handle func(LayoutEvent), last map[string]layout.Rect, width int, heightFor func(layout.Node) int) (Frame, layout.Composition, map[string]layout.Rect) {
	return settleFrameWithHeightProfile(view, handle, last, width, heightFor, colorprofile.Unknown)
}

func settleFrameWithHeightProfile(view func() Frame, handle func(LayoutEvent), last map[string]layout.Rect, width int, heightFor func(layout.Node) int, profile colorprofile.Profile) (Frame, layout.Composition, map[string]layout.Rect) {
	for pass := 0; ; pass++ {
		frame := view()
		if frame.Root == nil {
			return frame, layout.Composition{}, last
		}
		frame, comp := composeFrameWithColorProfile(frame, width, heightFor(frame.Root), profile)
		if handle == nil || pass == maxLayoutPasses || maps.Equal(comp.Rects, last) {
			return frame, comp, last
		}
		last = comp.Rects
		handle(LayoutEvent{Rects: maps.Clone(last)})
	}
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
