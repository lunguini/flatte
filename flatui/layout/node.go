// Package layout is a stateless flexbox-style layout solver and renderer
// for Flatte.
//
// Immediate mode: the solver holds nothing across frames. You feed it a node
// tree and a terminal size; Solve returns a map of node IDs to solved
// rectangles, Render returns the composed string.
//
// Nodes carry structure, geometry, and optional content. Leaf nodes with
// content auto-size to that content (no manual Height/Fixed needed). The
// app owns all state; content strings are plain rendered output.
//
// There is no escape hatch. If a layout can't be expressed, add a constraint
// type to the model; never hand-place.
package layout

// Rect is an integer-cell rectangle in terminal space.
type Rect struct {
	X, Y, W, H int
}

// Inset returns a rect shrunk by n cells on every side.
func (r Rect) Inset(n int) Rect {
	return Rect{
		X: r.X + n,
		Y: r.Y + n,
		W: r.W - 2*n,
		H: r.H - 2*n,
	}
}

// Contains reports whether (x,y) falls inside r.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// SizeKind classifies how a node's dimension along an axis is determined.
type SizeKind int

const (
	// SizeAuto is the default: stretches to fill the parent on both axes.
	SizeAuto SizeKind = iota
	// SizeFixed is an exact cell count, pinned on both axes.
	SizeFixed
	// SizeGrow fills remaining space proportionally by weight.
	SizeGrow
	// SizeContent is set by MeasurePass from content measurement.
	// Behaves as Fixed on the main axis (claims that many cells) but
	// stretches to fill the parent on the cross axis.
	SizeContent
)

// Size is a per-axis size constraint.
type Size struct {
	Kind   SizeKind
	Value  int     // for SizeFixed
	Weight float64 // for SizeGrow
}

// Fixed returns a SizeFixed constraint.
func Fixed(n int) Size { return Size{Kind: SizeFixed, Value: n} }

// Grow returns a SizeGrow constraint.
func Grow(weight float64) Size { return Size{Kind: SizeGrow, Weight: weight} }

// Auto returns a SizeAuto constraint.
func Auto() Size { return Size{Kind: SizeAuto} }

// Direction is the layout axis of a container node.
type Direction int

const (
	// LeafDir is a node with no children.
	LeafDir Direction = iota
	// RowDir lays children out horizontally (main axis = width).
	RowDir
	// ColDir lays children out vertically (main axis = height).
	ColDir
	// SlotDir is a named placeholder filled by Inject before solving.
	SlotDir
)

// Node is a single element in a layout tree. There is one type — everything
// is a Node. Containers (Row/Col) have Children. Text leaves carry Content.
// Widget leaves carry a Layout closure that produces a subtree at the
// solved rect. The engine handles all three uniformly.
type Node struct {
	ID       string
	Dir      Direction
	W        Size // width sizing
	H        Size // height sizing
	Pad      int  // uniform inset between border and children/content
	Gap      int  // gap between children (containers only)
	Bordered bool // draw a 1-cell border (reduces inner area)
	Overlay  bool // viewport-relative overlay (solved in a second pass)
	Children []Node
	Content  string            // text leaf: pre-rendered string
	Layout   func(r Rect) Node // widget leaf: produces subtree at solved rect
}

// innerInset returns the total inset (border + padding) applied to all sides
// when computing the content area for this node's children.
func (n Node) innerInset() int {
	inset := n.Pad
	if n.Bordered {
		inset++
	}
	return inset
}

// nonLayerChildren returns children that participate in normal flex layout.
// Overlay-flagged children are excluded — they solve against the viewport.
func (n Node) nonLayerChildren() []Node {
	var out []Node
	for _, c := range n.Children {
		if !c.Overlay {
			out = append(out, c)
		}
	}
	return out
}
