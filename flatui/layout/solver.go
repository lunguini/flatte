package layout

// Solve is the stateless entry point. Given a node tree and a viewport size,
// it returns a map of node IDs to solved rectangles. The solver holds nothing
// across frames.
//
// The algorithm is a top-down walk: Fixed children claim cells first, Grow
// children split the remainder by weight, Auto children take zero on the main
// axis and stretch on the cross axis. Remainder cells from integer division
// are distributed left-to-right (top-to-bottom) to Grow children so layouts
// don't jitter on resize.
//
// Layer-flagged nodes are excluded from normal flex layout and solved in a
// second pass against the viewport rect (see layers.go).
func Solve(root Node, width, height int) map[string]Rect {
	rects := make(map[string]Rect)
	solveNode(root, Rect{0, 0, width, height}, rects)
	solveOverlayPass(root, Rect{0, 0, width, height}, rects)
	return rects
}

// solveNode assigns the available rect to n, then distributes space to
// non-layer children. Layer children are skipped here — they get their own
// pass in solveOverlayPass.
func solveNode(n Node, avail Rect, out map[string]Rect) {
	if n.ID != "" {
		out[n.ID] = avail
	}

	if n.Dir == LeafDir || n.Dir == SlotDir {
		return
	}

	inset := n.innerInset()
	inner := avail.Inset(inset)

	children := n.nonLayerChildren()
	if len(children) == 0 {
		return
	}

	if n.Dir == RowDir {
		solveRow(children, n.Gap, inner, out)
	} else {
		solveCol(children, n.Gap, inner, out)
	}
}

// solveRow distributes children horizontally within inner.
// Main axis = width; cross axis = height (stretch unless Fixed).
func solveRow(children []Node, spacing int, inner Rect, out map[string]Rect) {
	n := len(children)
	totalSpacing := spacing * (n - 1)
	availMain := inner.W - totalSpacing
	if availMain < 0 {
		availMain = 0
	}

	// First pass: Fixed/Content children claim cells; sum Grow weights.
	fixedMain := 0
	growTotal := 0.0
	for _, c := range children {
		switch c.W.Kind {
		case SizeFixed, SizeContent:
			fixedMain += c.W.Value
		case SizeGrow:
			growTotal += c.W.Weight
		}
	}

	remaining := availMain - fixedMain
	if remaining < 0 {
		remaining = 0
	}

	// Assign main-axis sizes.
	mainSizes := make([]int, n)
	growUsed := 0
	for i, c := range children {
		switch c.W.Kind {
		case SizeFixed, SizeContent:
			mainSizes[i] = c.W.Value
		case SizeGrow:
			if growTotal > 0 {
				mainSizes[i] = int(float64(remaining) * c.W.Weight / growTotal)
			}
			growUsed += mainSizes[i]
		}
	}

	// Distribute remainder cells left-to-right to Grow children.
	remainder := remaining - growUsed
	for i, c := range children {
		if remainder <= 0 {
			break
		}
		if c.W.Kind == SizeGrow {
			mainSizes[i]++
			remainder--
		}
	}

	// Place children.
	x := inner.X
	for i, c := range children {
		crossSize := crossAxisSize(c.H, inner.H)
		placeChild(c, Rect{X: x, Y: inner.Y, W: mainSizes[i], H: crossSize}, out)
		x += mainSizes[i] + spacing
	}
}

// solveCol distributes children vertically within inner.
// Main axis = height; cross axis = width (stretch unless Fixed).
func solveCol(children []Node, spacing int, inner Rect, out map[string]Rect) {
	n := len(children)
	totalSpacing := spacing * (n - 1)
	availMain := inner.H - totalSpacing
	if availMain < 0 {
		availMain = 0
	}

	// First pass: Fixed/Content children claim cells; sum Grow weights.
	fixedMain := 0
	growTotal := 0.0
	for _, c := range children {
		switch c.H.Kind {
		case SizeFixed, SizeContent:
			fixedMain += c.H.Value
		case SizeGrow:
			growTotal += c.H.Weight
		}
	}

	remaining := availMain - fixedMain
	if remaining < 0 {
		remaining = 0
	}

	// Assign main-axis sizes.
	mainSizes := make([]int, n)
	growUsed := 0
	for i, c := range children {
		switch c.H.Kind {
		case SizeFixed, SizeContent:
			mainSizes[i] = c.H.Value
		case SizeGrow:
			if growTotal > 0 {
				mainSizes[i] = int(float64(remaining) * c.H.Weight / growTotal)
			}
			growUsed += mainSizes[i]
		}
	}

	// Distribute remainder cells top-to-bottom to Grow children.
	remainder := remaining - growUsed
	for i, c := range children {
		if remainder <= 0 {
			break
		}
		if c.H.Kind == SizeGrow {
			mainSizes[i]++
			remainder--
		}
	}

	// Place children.
	y := inner.Y
	for i, c := range children {
		crossSize := crossAxisSize(c.W, inner.W)
		placeChild(c, Rect{X: inner.X, Y: y, W: crossSize, H: mainSizes[i]}, out)
		y += mainSizes[i] + spacing
	}
}

// crossAxisSize resolves a child's cross-axis dimension. Fixed uses the
// specified value (clamped); Auto and Grow both stretch to fill the parent.
func crossAxisSize(s Size, avail int) int {
	if s.Kind == SizeFixed {
		if s.Value < avail {
			return s.Value
		}
	}
	return avail
}

// placeChild solves a child node within its assigned rect.
func placeChild(c Node, r Rect, out map[string]Rect) {
	if r.W < 0 {
		r.W = 0
	}
	if r.H < 0 {
		r.H = 0
	}
	solveNode(c, r, out)
}
