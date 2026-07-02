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
	walk(root, Rect{0, 0, width, height}, &buf, rects)

	// Overlay pass: centered layers composed on top of the resolved base.
	// Clear the layer's rect first (matching the string compositor's FillArea)
	// so the base cannot bleed through cells the overlay does not paint, then
	// recurse through the normal walk so overlay descendants get rects recorded
	// and container chrome is painted by the compositor rather than the legacy
	// string path.
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		buf.FillArea(&uv.EmptyCell, uv.Rect(mr.X, mr.Y, mr.W, mr.H))
		walk(o, mr, &buf, rects)
	}
	return buf.Render(), rects
}

// walk is the single solve+paint traversal shared by Solve and SolveAndCompose.
// It distributes rects top-down and records every ID'd node's rect. When buf is
// non-nil it also paints each leaf's Render into the buffer at its solved
// position (composition); with buf nil it is geometry-only (hit-testing). One
// distribution computation feeds both, so painted geometry and hit-test
// geometry cannot drift.
//
// Only nodes that are not childContainers (Text, Spacer, widgets) have their
// Render called here. Containers are distributed structurally and never
// Render'd by the walk, so Row/Col may delegate their own Render back to
// SolveAndCompose without risking infinite recursion.
func walk(n Node, r Rect, buf *uv.ScreenBuffer, out map[string]Rect) {
	if id := getID(n); id != "" {
		out[id] = r
	}

	container, isContainer := n.(childContainer)
	if !isContainer {
		// Leaf: paint its render at r. drawInto clips to r, so an over-wide or
		// over-tall string is bounded to the rect it was given.
		if buf != nil {
			drawInto(*buf, n.Render(r), r)
		}
		return
	}

	// Container: paint its own border/background chrome first (if any), then
	// paint children on top inside the inset region.
	if buf != nil {
		if inset := getInset(n); inset > 0 {
			drawInto(*buf, fillRect("", r, getPad(n), isBordered(n)), r)
		}
	}

	children := nonOverlayChildren(container.GetChildren())
	if len(children) == 0 {
		return
	}

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
		// Clamp to the parent's inner box: Fixed/Content children are not shrunk
		// by distribution, so an overflowing row/col would otherwise place a
		// child rect past the frame edge.
		cr = cr.Intersect(inner)
		walk(c, cr, buf, out)
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
