package layout

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// measureString returns the intrinsic dimensions of a rendered string:
// height = line count, width = widest line (ANSI-aware via lipgloss.Width).
func measureString(content string) (width, height int) {
	lines := strings.Split(content, "\n")
	height = len(lines)
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > width {
			width = w
		}
	}
	return
}

// MeasurePass walks the tree bottom-up and replaces Auto sizes with
// Fixed sizes derived from content. This is the "Content sizing" pass:
//
//   - Leaf with Content: W/H auto-size to the content's intrinsic dimensions
//     (plus padding/border insets).
//   - Container with Auto H: Row → max child height, Col → sum child heights.
//   - Container with Auto W: Row → sum child widths, Col → max child width.
//   - Fixed/Grow nodes are left untouched.
//
// After this pass, every Auto size that can be resolved has been converted
// to Fixed. Unresolved Autos (e.g. a container with a Grow child but no
// explicit size from the parent) stay Auto and behave as before (stretch on
// cross-axis, zero on main-axis).
//
// MeasurePass is exported so apps can pre-measure a tree in resize() to
// derive body dimensions before View() renders content.
func MeasurePass(n Node) Node {
	// Copy children slice to avoid mutating the caller's tree.
	if len(n.Children) > 0 {
		children := make([]Node, len(n.Children))
		copy(children, n.Children)
		for i := range children {
			children[i] = MeasurePass(children[i])
		}
		n.Children = children
	}

	inset := 2 * n.Pad
	if n.Bordered {
		inset += 2
	}

	if n.Content != "" && n.Dir == LeafDir {
		cw, ch := measureString(n.Content)
		if n.W.Kind == SizeAuto {
			n.W = Fixed(cw + inset)
		}
		if n.H.Kind == SizeAuto {
			n.H = Fixed(ch + inset)
		}
		return n
	}

	children := n.nonLayerChildren()
	if len(children) == 0 {
		return n
	}

	// Container auto-sizing: derive from children when main axis is Auto.
	if n.Dir == RowDir && n.W.Kind == SizeAuto {
		total := 0
		for i, c := range children {
			if c.W.Kind == SizeFixed {
				total += c.W.Value
			}
			if i > 0 {
				total += n.Gap
			}
		}
		if total > 0 {
			n.W = Fixed(total + inset)
		}
	}
	if n.Dir == RowDir && n.H.Kind == SizeAuto {
		maxH := 0
		for _, c := range children {
			if c.H.Kind == SizeFixed && c.H.Value > maxH {
				maxH = c.H.Value
			}
		}
		if maxH > 0 {
			n.H = Fixed(maxH + inset)
		}
	}
	if n.Dir == ColDir && n.H.Kind == SizeAuto {
		total := 0
		for i, c := range children {
			if c.H.Kind == SizeFixed {
				total += c.H.Value
			}
			if i > 0 {
				total += n.Gap
			}
		}
		if total > 0 {
			n.H = Fixed(total + inset)
		}
	}
	if n.Dir == ColDir && n.W.Kind == SizeAuto {
		maxW := 0
		for _, c := range children {
			if c.W.Kind == SizeFixed && c.W.Value > maxW {
				maxW = c.W.Value
			}
		}
		if maxW > 0 {
			n.W = Fixed(maxW + inset)
		}
	}

	return n
}

// Render solves the tree (including content measurement) and produces the
// composed string. This is the one-call API: tree in, frame content out.
// No manual strings.Join, no manual width math.
//
// For hit-testing, call Solve separately (Render calls it internally).
func Render(root Node, width, height int) string {
	root = MeasurePass(root)
	return renderNode(root, Rect{0, 0, width, height})
}

// renderNode produces the rendered string for a node and its subtree,
// fitting into the given rect. This is the rendering counterpart of
// solveNode — same distribution logic, but instead of writing to a map,
// it composes strings.
func renderNode(n Node, r Rect) string {
	if r.W < 0 {
		r.W = 0
	}
	if r.H < 0 {
		r.H = 0
	}

	if n.Dir == LeafDir || n.Dir == SlotDir {
		return renderLeaf(n, r)
	}

	inset := n.innerInset()
	inner := r.Inset(inset)
	children := n.nonLayerChildren()
	if len(children) == 0 {
		return fillRect("", r)
	}

	var composed string
	if n.Dir == RowDir {
		composed = renderRow(children, n.Gap, inner)
	} else {
		composed = renderCol(children, n.Gap, inner)
	}

	if inset > 0 {
		composed = applyInsets(composed, r, inset)
	}
	return composed
}

// renderLeaf renders a leaf node's content into its rect, applying
// padding and border from the node's settings.
func renderLeaf(n Node, r Rect) string {
	if n.Content == "" {
		return fillRect("", r)
	}
	style := lipgloss.NewStyle().Width(r.W).Height(r.H).MaxWidth(r.W).MaxHeight(r.H)
	if n.Pad > 0 {
		style = style.Padding(0, n.Pad, n.Pad, n.Pad)
	}
	if n.Bordered {
		style = style.Border(lipgloss.RoundedBorder())
	}
	return style.Render(n.Content)
}

// renderRow distributes children horizontally and joins their rendered output.
func renderRow(children []Node, gap int, inner Rect) string {
	n := len(children)
	totalSpacing := gap * (n - 1)
	availMain := inner.W - totalSpacing
	if availMain < 0 {
		availMain = 0
	}

	fixedMain := 0
	growTotal := 0.0
	for _, c := range children {
		switch c.W.Kind {
		case SizeFixed:
			fixedMain += c.W.Value
		case SizeGrow:
			growTotal += c.W.Weight
		}
	}

	remaining := availMain - fixedMain
	if remaining < 0 {
		remaining = 0
	}

	mainSizes := make([]int, n)
	growUsed := 0
	for i, c := range children {
		switch c.W.Kind {
		case SizeFixed:
			mainSizes[i] = c.W.Value
		case SizeGrow:
			if growTotal > 0 {
				mainSizes[i] = int(float64(remaining) * c.W.Weight / growTotal)
			}
			growUsed += mainSizes[i]
		}
	}

	// Distribute remainder left-to-right.
	rem := remaining - growUsed
	for i, c := range children {
		if rem <= 0 {
			break
		}
		if c.W.Kind == SizeGrow {
			mainSizes[i]++
			rem--
		}
	}

	parts := make([]string, n)
	for i, c := range children {
		cross := crossAxisSize(c.H, inner.H)
		parts[i] = renderNode(c, Rect{0, 0, mainSizes[i], cross})
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderCol distributes children vertically and joins their rendered output.
func renderCol(children []Node, gap int, inner Rect) string {
	n := len(children)
	totalSpacing := gap * (n - 1)
	availMain := inner.H - totalSpacing
	if availMain < 0 {
		availMain = 0
	}

	fixedMain := 0
	growTotal := 0.0
	for _, c := range children {
		switch c.H.Kind {
		case SizeFixed:
			fixedMain += c.H.Value
		case SizeGrow:
			growTotal += c.H.Weight
		}
	}

	remaining := availMain - fixedMain
	if remaining < 0 {
		remaining = 0
	}

	mainSizes := make([]int, n)
	growUsed := 0
	for i, c := range children {
		switch c.H.Kind {
		case SizeFixed:
			mainSizes[i] = c.H.Value
		case SizeGrow:
			if growTotal > 0 {
				mainSizes[i] = int(float64(remaining) * c.H.Weight / growTotal)
			}
			growUsed += mainSizes[i]
		}
	}

	rem := remaining - growUsed
	for i, c := range children {
		if rem <= 0 {
			break
		}
		if c.H.Kind == SizeGrow {
			mainSizes[i]++
			rem--
		}
	}

	parts := make([]string, n)
	for i, c := range children {
		cross := crossAxisSize(c.W, inner.W)
		parts[i] = renderNode(c, Rect{0, 0, cross, mainSizes[i]})
	}
	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// applyInsets wraps content with padding/border to fill the outer rect.
func applyInsets(content string, outer Rect, inset int) string {
	style := lipgloss.NewStyle().Width(outer.W).Height(outer.H).
		MaxWidth(outer.W).MaxHeight(outer.H)
	return style.Render(content)
}

// fillRect returns a string of spaces filling the given rect.
func fillRect(content string, r Rect) string {
	if r.W <= 0 || r.H <= 0 {
		return ""
	}
	style := lipgloss.NewStyle().Width(r.W).Height(r.H).MaxWidth(r.W).MaxHeight(r.H)
	return style.Render(content)
}
