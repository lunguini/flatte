package layout

import (
	uv "github.com/charmbracelet/ultraviolet"
)

// SolveAndCompose walks the tree exactly once. It distributes rects top-down
// (the same math as Solve) and, in the same pass, paints each leaf's
// Render(rect) into a cell buffer at its absolute position, recording the rect
// of every node that carries an ID. It returns the composed frame string and
// the id→rect geometry map.
//
// This is the single-pass alternative to calling Solve and Render separately:
// geometry and pixels come from one traversal and one distribution computation,
// so they cannot drift. Because leaves are painted at their solved coordinates
// and clipped to the buffer, a leaf that overflows its rect cannot corrupt the
// frame, and a leaf that under-fills simply reveals whatever a parent painted
// underneath — there is no "every leaf must fill its rect exactly" invariant.
func SolveAndCompose(root Node, width, height int) (string, map[string]Rect) {
	if width <= 0 || height <= 0 {
		return "", map[string]Rect{}
	}
	buf := uv.NewScreenBuffer(width, height)
	rects := make(map[string]Rect)
	composeNode(root, Rect{0, 0, width, height}, buf, rects)

	// Overlay pass: centered layers painted on top of the resolved base.
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		if id := getID(o); id != "" {
			rects[id] = mr
		}
		drawInto(buf, o.Render(mr), mr)
	}
	return buf.Render(), rects
}

// composeNode is the shared solve+paint recursion. It mirrors solveNode's
// distribution so the two stay in lockstep, and additionally paints.
func composeNode(n Node, r Rect, buf uv.ScreenBuffer, out map[string]Rect) {
	if id := getID(n); id != "" {
		out[id] = r
	}
	children := nonOverlayChildren(getChildren(n))
	if len(children) == 0 {
		// Leaf: paint its render at r. drawInto clips to r, so an over-wide or
		// over-tall string is bounded to the rect it was given.
		drawInto(buf, n.Render(r), r)
		return
	}

	// Container: paint its own border/background chrome first (if any), then
	// paint children on top inside the inset region.
	if inset := getInset(n); inset > 0 {
		drawInto(buf, fillRect("", r, getPad(n), isBordered(n)), r)
	}

	container := n.(childContainer)
	gap := getGap(n)
	horizontal := container.IsHorizontal()
	inner := r.Inset(getInset(n))
	mainSizes := distributeMain(children, horizontal, gap, mainAvail(inner, horizontal))

	pos := inner.X
	if !horizontal {
		pos = inner.Y
	}
	for i, c := range children {
		cw, ch := c.Size()
		var cr Rect
		if horizontal {
			cross := crossAxis(ch, inner.H)
			cr = Rect{pos, inner.Y, mainSizes[i], cross}
		} else {
			cross := crossAxis(cw, inner.W)
			cr = Rect{inner.X, pos, cross, mainSizes[i]}
		}
		composeNode(c, cr, buf, out)
		pos += mainSizes[i] + gap
	}
}

// drawInto paints a styled string into buf, clipped to r. Cells the string does
// not cover are left untouched (revealing whatever was painted underneath).
func drawInto(buf uv.ScreenBuffer, s string, r Rect) {
	if r.W <= 0 || r.H <= 0 || s == "" {
		return
	}
	uv.NewStyledString(s).Draw(buf, uv.Rect(r.X, r.Y, r.W, r.H))
}

// getPad / isBordered read chrome fields off a node's NodeBase for the
// container chrome paint. They mirror the innerInset accounting.
func getPad(n Node) int {
	type padder interface{ pad() int }
	if p, ok := n.(padder); ok {
		return p.pad()
	}
	return 0
}

func isBordered(n Node) bool {
	type borderer interface{ bordered() bool }
	if b, ok := n.(borderer); ok {
		return b.bordered()
	}
	return false
}
