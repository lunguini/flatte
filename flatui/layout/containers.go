package layout

// Row lays children out horizontally (main axis = width).
// Embed NodeBase for geometry. Children participate in distribution
// via their Size() constraints.
type Row struct {
	NodeBase
	Children []Node
}

// Size declares the row's claims: explicit W/H as set, and on an Auto axis
// the measured content of its children (or Grow(1) when the only claim on
// that axis is a growing child).
func (r Row) Size() (Size, Size) {
	w, h := intrinsic[int](cells{}, r)
	return fromSized(w), fromSized(h)
}

// Render composes the row into a self-contained block sized to rect. It
// delegates to the single walk via SolveAndCompose (using rect's dimensions at
// a local origin) so container distribution, chrome, clipping, and overlay
// compositing all follow one code path. Only the childless case takes the
// direct fillBox path — which also prevents the walk (whose leaf branch would
// otherwise Render a container) from recursing.
func (r Row) Render(rect Rect) string {
	if len(r.Children) == 0 {
		if r.Chrome != nil {
			return r.Chrome(rect)
		}
		return fillBox("", rect, r.padSides(), r.box())
	}
	frame, _ := SolveAndCompose(r, rect.W, rect.H)
	return frame
}

// Introspection for Solve and overlay pass.
func (r Row) GetChildren() []Node { return r.Children }
func (r Row) IsHorizontal() bool  { return true }
func (r Row) GetGap() int         { return r.Gap }

// Col lays children out vertically (main axis = height).
type Col struct {
	NodeBase
	Children []Node
}

// Size declares the column's claims; see Row.Size.
func (c Col) Size() (Size, Size) {
	w, h := intrinsic[int](cells{}, c)
	return fromSized(w), fromSized(h)
}

// Render composes the column into a self-contained block sized to rect,
// delegating to the single walk via SolveAndCompose. See Row.Render for the
// rationale, including why the childless case stays on the direct path.
func (c Col) Render(rect Rect) string {
	if len(c.Children) == 0 {
		if c.Chrome != nil {
			return c.Chrome(rect)
		}
		return fillBox("", rect, c.padSides(), c.box())
	}
	frame, _ := SolveAndCompose(c, rect.W, rect.H)
	return frame
}

func (c Col) GetChildren() []Node { return c.Children }
func (c Col) IsHorizontal() bool  { return false }
func (c Col) GetGap() int         { return c.Gap }
