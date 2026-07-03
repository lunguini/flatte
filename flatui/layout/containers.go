package layout

// Row lays children out horizontally (main axis = width).
// Embed NodeBase for geometry. Children participate in distribution
// via their Size() constraints.
type Row struct {
	NodeBase
	Children []Node
}

func (r Row) Size() (Size, Size) {
	w, h := r.NodeBase.Size()
	if w.Kind == SizeAuto || h.Kind == SizeAuto {
		mainSize, crossSize, mainGrow, crossGrow := measureChildren(r.Children, true, r.Gap)
		inset := 2 * r.innerInset()
		// Row: main=width, cross=height. On an Auto axis, prefer measured
		// content; if there is none but a child grows on that axis, report
		// Grow(1) so the container does not collapse to zero.
		if w.Kind == SizeAuto {
			if mainSize > 0 {
				w = Size{Kind: SizeContent, Value: mainSize + inset}
			} else if mainGrow {
				w = Grow(1)
			}
		}
		if h.Kind == SizeAuto {
			if crossSize > 0 {
				h = Size{Kind: SizeContent, Value: crossSize + inset}
			} else if crossGrow {
				h = Grow(1)
			}
		}
	}
	return w, h
}

// Render composes the row into a self-contained block sized to rect. It
// delegates to the single walk via SolveAndCompose (using rect's dimensions at
// a local origin) so container distribution, chrome, clipping, and overlay
// compositing all follow one code path. Only the childless case takes the
// direct fillRect path — which also prevents the walk (whose leaf branch would
// otherwise Render a container) from recursing.
func (r Row) Render(rect Rect) string {
	if len(r.Children) == 0 {
		if r.Chrome != nil {
			return r.Chrome(rect)
		}
		return fillRect("", rect, r.Pad, r.Bordered)
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

func (c Col) Size() (Size, Size) {
	w, h := c.NodeBase.Size()
	if w.Kind == SizeAuto || h.Kind == SizeAuto {
		mainSize, crossSize, mainGrow, crossGrow := measureChildren(c.Children, false, c.Gap)
		inset := 2 * c.innerInset()
		// Col: main=height, cross=width. On an Auto axis, prefer measured
		// content; if there is none but a child grows on that axis, report
		// Grow(1) so the container does not collapse to zero.
		if h.Kind == SizeAuto {
			if mainSize > 0 {
				h = Size{Kind: SizeContent, Value: mainSize + inset}
			} else if mainGrow {
				h = Grow(1)
			}
		}
		if w.Kind == SizeAuto {
			if crossSize > 0 {
				w = Size{Kind: SizeContent, Value: crossSize + inset}
			} else if crossGrow {
				w = Grow(1)
			}
		}
	}
	return w, h
}

// Render composes the column into a self-contained block sized to rect,
// delegating to the single walk via SolveAndCompose. See Row.Render for the
// rationale, including why the childless case stays on the direct path.
func (c Col) Render(rect Rect) string {
	if len(c.Children) == 0 {
		if c.Chrome != nil {
			return c.Chrome(rect)
		}
		return fillRect("", rect, c.Pad, c.Bordered)
	}
	frame, _ := SolveAndCompose(c, rect.W, rect.H)
	return frame
}

func (c Col) GetChildren() []Node { return c.Children }
func (c Col) IsHorizontal() bool  { return false }
func (c Col) GetGap() int         { return c.Gap }
