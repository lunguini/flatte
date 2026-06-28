package layout

// Row creates a horizontal container. Children are laid out left-to-right;
// their W sizing controls main-axis distribution, their H sizing controls
// cross-axis behaviour (Fixed = pinned, Auto/Grow = stretch to fill).
func Row(children ...Node) Node {
	return Node{Dir: RowDir, Children: children}
}

// Col creates a vertical container. Children are laid out top-to-bottom;
// their H sizing controls main-axis distribution, their W sizing controls
// cross-axis behaviour.
func Col(children ...Node) Node {
	return Node{Dir: ColDir, Children: children}
}

// Box creates a leaf node — a terminal content area identified by id.
// The app matches solved rects by this id to know where to draw.
func Box(id string) Node {
	return Node{ID: id, Dir: LeafDir}
}

// ContentBox creates a leaf node carrying rendered content. When passed to
// Render, the content is composed into the solved rect automatically. When
// the main-axis size is Auto (the default), the node auto-sizes to the
// content's intrinsic dimensions — no manual Height/Fixed needed.
func ContentBox(id, content string) Node {
	return Node{ID: id, Dir: LeafDir, Content: content}
}

// Content creates an anonymous content leaf — a string at the tree's edge.
// Auto-sizes to the string's dimensions.
func Content(s string) Node {
	return Node{Dir: LeafDir, Content: s}
}

// El wraps an Element as a layout leaf node. The engine calls the element's
// Layout method during measure (to discover intrinsic size) and render
// (with solved dimensions) to obtain and render the element's subtree.
func El(e Element) Node {
	return Node{Dir: LeafDir, Element: e}
}

// Spacer returns a node that grows to fill remaining space. In a Row it
// pushes subsequent children to the right; in a Col it pushes them down.
// This is shorthand for Box("").Grow(1).
func Spacer() Node {
	return Node{Dir: LeafDir, W: Grow(1), H: Grow(1)}
}

// Slot creates a named placeholder that Inject fills before solving.
// An unfilled slot behaves as a zero-size leaf in the solver.
func Slot(id string) Node {
	return Node{ID: id, Dir: SlotDir}
}

// Width sets a fixed width constraint.
func (n Node) Width(w int) Node {
	n.W = Fixed(w)
	return n
}

// Height sets a fixed height constraint.
func (n Node) Height(h int) Node {
	n.H = Fixed(h)
	return n
}

// Grow sets both axes to grow with the given weight. Apply this to a child
// that should fill remaining space along its parent's main axis. On the
// cross-axis, Grow behaves identically to Auto (stretch to fill).
//
// To grow on one axis and fix the other, chain after Grow:
//
//	Box("toolbar").Grow(1).Height(3) // fills width, pinned to 3 rows
func (n Node) Grow(weight float64) Node {
	n.W = Size{Kind: SizeGrow, Weight: weight}
	n.H = Size{Kind: SizeGrow, Weight: weight}
	return n
}

// Border enables a 1-cell border. The border reduces the inner area
// available for children or content by 1 cell on each side.
func (n Node) Border() Node {
	n.Bordered = true
	return n
}

// Padding sets a uniform inset between the border (or edge) and children.
func (n Node) Padding(p int) Node {
	n.Pad = p
	return n
}

// Spacing sets the gap between children in a container.
func (n Node) Spacing(s int) Node {
	n.Gap = s
	return n
}

// Layer marks this node as a viewport-relative overlay. Layer nodes are
// excluded from normal flex layout and solved in a separate pass against
// the full viewport (see layers.go). The node is centered by default.
func (n Node) Layer() Node {
	n.Overlay = true
	return n
}

// Inject replaces every Slot with the given slotID inside root with the
// replacement node. If no slot matches, root is returned unchanged.
// Inject is purely structural — it walks the tree and swaps nodes by name.
// It carries no state, no conditionals, no behavior.
func Inject(root Node, slotID string, replacement Node) Node {
	for i, c := range root.Children {
		if c.Dir == SlotDir && c.ID == slotID {
			root.Children[i] = replacement
			continue
		}
		if len(c.Children) > 0 {
			root.Children[i] = Inject(c, slotID, replacement)
		}
	}
	return root
}
