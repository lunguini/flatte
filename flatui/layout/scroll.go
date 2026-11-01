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

// ClampScroll clamps a scroll offset to [0, content-viewport]: the offset a
// Scroll whose window is viewport rows tall shows for content rows of child.
// The walk uses it, so an app clamping its own OffsetY with it agrees with
// what is painted. Negative heights count as zero.
func ClampScroll(offset, viewport, content int) int {
	return clampScrollOf[int](cells{}, offset, viewport, content)
}

type scroller interface{ scroll() (Node, int) }
