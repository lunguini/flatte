package layout

// Scroll shows Child through a window: the child is laid out at its natural
// height and the Scroll's inner box (inside Pad/Bordered) shows the rows
// starting at OffsetY. Painting and hit-testing are clipped to that window, so
// scrolled-out nodes are neither drawn nor hittable.
//
// The offset is app state: the app owns OffsetY, changes it from wheel and key
// events, and keeps whatever must stay visible in view. The walk clamps it to
// [0, content-viewport] with ClampScroll; apps clamp their own copy the same
// way so that scrolling back from an overshoot responds immediately.
//
// Geometry needs no extra API. Rects of nodes inside a Scroll are reported in
// frame coordinates shifted by the effective offset (rows above the window
// have negative Y), and the content size is the child's own rect: give the
// child an ID to read it. Overlays inside a Scroll stay centered on the frame.
//
// The child's height comes from its Size: Fixed is honored exactly, measured
// Content fills at least the window (so wrapping a layout that fits in a
// Scroll changes nothing visible), and Grow/Auto is the window height. Width
// follows the usual cross-axis rule: Fixed pins, everything else stretches.
//
// Leaves inside a Scroll render at their full solved size and are then
// clipped; leaves entirely outside the window are not rendered at all.
type Scroll struct {
	NodeBase
	OffsetY int
	// OffsetPoints scrolls a window this many points further, on top of
	// OffsetY's lines, for smooth scrolling (the sum is clamped like
	// OffsetY). A terminal shows whole rows and ignores it.
	OffsetPoints float64
	// ScrollX also lets the child be wider than the window, laid out at its
	// natural width (Fixed exactly, Content at least the window, Grow/Auto
	// the window, as for its height) and shown from column OffsetX. Without
	// it the child gets the window's width and OffsetX is ignored.
	ScrollX bool
	OffsetX int
	Child   Node
}

// Size measures the child's width on an Auto width. An Auto height reports
// Grow(1): a Scroll exists to be smaller than its content, so measuring the
// content would defeat it.
func (s Scroll) Size() (Size, Size) {
	w, h := intrinsic[int](cells{}, s)
	return fromSized(w), fromSized(h)
}

// Render composes the Scroll into a block sized to rect through the shared
// walk, like Row and Col.
func (s Scroll) Render(rect Rect) string {
	if s.Child == nil {
		if s.Chrome != nil {
			return s.Chrome(rect)
		}
		return fillBox("", rect, s.padSides(), s.box())
	}
	frame, _ := SolveAndCompose(s, rect.W, rect.H)
	return frame
}

// GetChildren exposes the child to the overlay pass.
func (s Scroll) GetChildren() []Node {
	if s.Child == nil {
		return nil
	}
	return []Node{s.Child}
}

func (s Scroll) IsHorizontal() bool { return false }

func (s Scroll) scroll() (Node, int) { return s.Child, s.OffsetY }

func (s Scroll) scrollPoints() float64 { return s.OffsetPoints }

func (s Scroll) across() (int, bool) { return s.OffsetX, s.ScrollX }

// ClampScroll clamps a scroll offset to [0, content-viewport]: the offset a
// Scroll whose window is viewport rows tall shows for content rows of child.
// The walk uses it, so an app clamping its own OffsetY with it agrees with
// what is painted. Negative heights count as zero.
func ClampScroll(offset, viewport, content int) int {
	return clampScrollOf[int](cells{}, offset, viewport, content)
}

// ScrollToShow is the offset, as near to offset as possible, whose window of
// viewport shows the content from start to end (its start when it is larger
// than the window). Either axis; the caller still clamps (ClampScroll).
func ScrollToShow(offset, viewport, start, end int) int {
	if end > offset+viewport {
		offset = end - viewport
	}
	return min(offset, start)
}

type scroller interface {
	scroll() (Node, int)
	scrollPoints() float64 // more of the offset, in points (a window's only)
	across() (offset int, ok bool)
}
