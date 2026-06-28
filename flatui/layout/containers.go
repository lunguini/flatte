package layout

import "charm.land/lipgloss/v2"

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
		mainSize, crossSize := measureChildren(r.Children, true, r.Gap)
		inset := 2 * r.innerInset()
		// Row: main=width, cross=height
		if w.Kind == SizeAuto && mainSize > 0 {
			w = Size{Kind: SizeContent, Value: mainSize + inset}
		}
		if h.Kind == SizeAuto && crossSize > 0 {
			h = Size{Kind: SizeContent, Value: crossSize + inset}
		}
	}
	return w, h
}

func (r Row) Render(rect Rect) string {
	inset := r.innerInset()
	inner := rect.Inset(inset)
	children := nonOverlayChildren(r.Children)
	if len(children) == 0 {
		return fillRect("", rect, r.Pad, r.Bordered)
	}
	mainSizes := distributeMain(children, true, r.Gap, inner.W)
	parts := make([]string, len(children))
	x := inner.X
	for i, c := range children {
		_, ch := c.Size()
		cross := crossAxis(ch, inner.H)
		parts[i] = c.Render(Rect{x, inner.Y, mainSizes[i], cross})
		x += mainSizes[i] + r.Gap
	}
	result := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if inset > 0 {
		result = fillRect(result, rect, r.Pad, r.Bordered)
	}
	return result
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
		mainSize, crossSize := measureChildren(c.Children, false, c.Gap)
		inset := 2 * c.innerInset()
		// Col: main=height, cross=width
		if h.Kind == SizeAuto && mainSize > 0 {
			h = Size{Kind: SizeContent, Value: mainSize + inset}
		}
		if w.Kind == SizeAuto && crossSize > 0 {
			w = Size{Kind: SizeContent, Value: crossSize + inset}
		}
	}
	return w, h
}

func (c Col) Render(rect Rect) string {
	inset := c.innerInset()
	inner := rect.Inset(inset)
	children := nonOverlayChildren(c.Children)
	if len(children) == 0 {
		return fillRect("", rect, c.Pad, c.Bordered)
	}
	mainSizes := distributeMain(children, false, c.Gap, inner.H)
	parts := make([]string, len(children))
	y := inner.Y
	for i, child := range children {
		cw, _ := child.Size()
		cross := crossAxis(cw, inner.W)
		parts[i] = child.Render(Rect{inner.X, y, cross, mainSizes[i]})
		y += mainSizes[i] + c.Gap
	}
	result := lipgloss.JoinVertical(lipgloss.Top, parts...)
	if inset > 0 {
		result = fillRect(result, rect, c.Pad, c.Bordered)
	}
	return result
}

func (c Col) GetChildren() []Node { return c.Children }
func (c Col) IsHorizontal() bool  { return false }
func (c Col) GetGap() int         { return c.Gap }
