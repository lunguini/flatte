package layout

// Row creates a horizontal container (main axis = width).
func Row(children ...Node) Node {
	return Node{Dir: RowDir, Children: children}
}

// Col creates a vertical container (main axis = height).
func Col(children ...Node) Node {
	return Node{Dir: ColDir, Children: children}
}

// Text creates a leaf carrying a pre-rendered string. Auto-sizes to the
// string's dimensions unless explicit Width/Height is set.
func Text(s string) Node {
	return Node{Dir: LeafDir, Content: s}
}

// Box creates a named geometry leaf — a Node with an ID but no content.
// Used for hit-testing (Solve returns its rect keyed by ID). Shorthand
// for Node{ID: id, Dir: LeafDir}.
func Box(id string) Node {
	return Node{ID: id, Dir: LeafDir}
}

// Spacer returns a node that grows to fill remaining space. In a Row it
// pushes subsequent children right; in a Col it pushes them down.
func Spacer() Node {
	return Node{Dir: LeafDir, W: Grow(1), H: Grow(1)}
}

// Slot creates a named placeholder that Inject fills before solving.
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

// Grow sets both axes to grow with the given weight.
func (n Node) Grow(weight float64) Node {
	n.W = Size{Kind: SizeGrow, Weight: weight}
	n.H = Size{Kind: SizeGrow, Weight: weight}
	return n
}

// Border enables a 1-cell border.
func (n Node) Border() Node {
	n.Bordered = true
	return n
}

// Padding sets a uniform inset.
func (n Node) Padding(p int) Node {
	n.Pad = p
	return n
}

// Spacing sets the gap between children.
func (n Node) Spacing(s int) Node {
	n.Gap = s
	return n
}

// Layer marks this node as a viewport-relative overlay.
func (n Node) Layer() Node {
	n.Overlay = true
	return n
}

// Inject replaces every Slot with the given slotID inside root with the
// replacement node. Purely structural — no state, no behavior.
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
