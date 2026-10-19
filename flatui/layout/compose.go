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
	frame, rects, _ := SolveAndComposeWithDiagnostics(root, width, height)
	return frame, rects
}

// DuplicateIDDiagnostic reports a repeated non-empty node ID in traversal
// order. The later node still wins in the returned rect map, preserving the
// historical behavior; diagnostics make accidental collisions observable
// without changing that behavior.
type DuplicateIDDiagnostic struct {
	ID        string
	FirstRect Rect
	Duplicate Rect
}

// SolveAndComposeWithDiagnostics is SolveAndCompose with non-breaking
// duplicate-ID diagnostics. Empty IDs remain valid and are not reported.
func SolveAndComposeWithDiagnostics(root Node, width, height int) (string, map[string]Rect, []DuplicateIDDiagnostic) {
	c := Compose(root, width, height)
	return c.Frame, c.Rects, c.Diagnostics
}

// Composition is the result of one solve+paint walk: the composed frame, the
// id→rect geometry, duplicate-ID diagnostics, and the paint order needed to
// answer which node is on top at a cell.
type Composition struct {
	Frame       string
	Rects       map[string]Rect
	Diagnostics []DuplicateIDDiagnostic
	hits        []hit
}

// hit is one hit-testable region in paint order. An empty id marks an
// overlay layer: it absorbs hits that reach it, so a modal blocks the base
// painted underneath even where it has no ID'd child.
type hit struct {
	id   string
	rect Rect
}

// HitTest returns the ID of the topmost ID'd node containing cell (x, y):
// the last one painted, so children beat parents and overlays beat the base.
// It returns "" outside the frame, over an overlay area with no ID'd node,
// or where no ID'd node is painted.
func (c Composition) HitTest(x, y int) string {
	for i := len(c.hits) - 1; i >= 0; i-- {
		if c.hits[i].rect.Contains(x, y) {
			return c.hits[i].id
		}
	}
	return ""
}

// Compose is SolveAndCompose returning a Composition, which adds hit-testing
// in paint order to the frame, rects, and diagnostics.
func Compose(root Node, width, height int) Composition {
	if width <= 0 || height <= 0 {
		return Composition{Rects: map[string]Rect{}}
	}
	buf := uv.NewScreenBuffer(width, height)
	rects := make(map[string]Rect)
	state := walkState{rects: rects, first: make(map[string]Rect)}
	// Overlay roots do not participate in the base pass. They are painted by
	// the centered overlay pass below; walking one here first would paint it at
	// the viewport origin and then paint it a second time at its centered rect.
	if !isOverlay(root) {
		walk(root, Rect{0, 0, width, height}, &buf, &state)
	}

	// Overlay pass: centered layers composed on top of the resolved base.
	// Clear the layer's rect first (matching the string compositor's FillArea)
	// so the base cannot bleed through cells the overlay does not paint, then
	// recurse through the normal walk so overlay descendants get rects recorded
	// and container chrome is painted by the compositor rather than the legacy
	// string path.
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		buf.FillArea(&uv.EmptyCell, uv.Rect(mr.X, mr.Y, mr.W, mr.H))
		walkOverlay(o, mr, &buf, &state)
	}
	return Composition{Frame: buf.Render(), Rects: rects, Diagnostics: state.diagnostics, hits: state.hits}
}

type walkState struct {
	rects       map[string]Rect
	first       map[string]Rect
	diagnostics []DuplicateIDDiagnostic
	hits        []hit
}

// walkOverlay walks an overlay layer after recording it as a hit barrier, so
// the layer's own nodes (recorded during the walk) sit above the barrier and
// the barrier sits above everything painted before it.
func walkOverlay(o Node, r Rect, buf *uv.ScreenBuffer, state *walkState) {
	state.hits = append(state.hits, hit{rect: r})
	walk(o, r, buf, state)
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
func walk(n Node, r Rect, buf *uv.ScreenBuffer, state *walkState) {
	if id := getID(n); id != "" {
		if first, ok := state.first[id]; ok {
			state.diagnostics = append(state.diagnostics, DuplicateIDDiagnostic{ID: id, FirstRect: first, Duplicate: r})
		} else {
			state.first[id] = r
		}
		state.rects[id] = r
		state.hits = append(state.hits, hit{id: id, rect: r})
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
	// paint children on top inside the inset region. A custom Chrome painter
	// replaces the default Pad/Bordered fill and is painted even with zero
	// inset; the layout inset itself still comes from Pad/Bordered only.
	if buf != nil {
		if ch := getChrome(n); ch != nil {
			drawInto(*buf, ch(r), r)
		} else if getInsetSides(n).any() {
			drawInto(*buf, fillRect("", r, getPadSides(n), isBordered(n)), r)
		}
	}

	children := nonOverlayChildren(container.GetChildren())
	if len(children) == 0 {
		return
	}

	gap := getGap(n)
	horizontal := container.IsHorizontal()
	inner := r.insetBy(getInsetSides(n))
	mainSizes := distributeMain(children, horizontal, gap, mainAvail(inner, horizontal))

	pos := inner.X
	mainOrigin := inner.X
	if !horizontal {
		pos = inner.Y
		mainOrigin = inner.Y
	}
	innerEnd := rectEnd(mainOrigin, mainAvail(inner, horizontal))
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
		walk(c, cr, buf, state)
		step := satAdd(mainSizes[i], gap)
		pos = min(innerEnd, satAdd(pos, step))
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

// getPadSides / isBordered read chrome fields off a node's NodeBase for the
// container chrome paint. They mirror the insetSides accounting.
func getPadSides(n Node) sides {
	type padder interface{ padSides() sides }
	if p, ok := n.(padder); ok {
		return p.padSides()
	}
	return sides{}
}

func isBordered(n Node) bool {
	type borderer interface{ bordered() bool }
	if b, ok := n.(borderer); ok {
		return b.bordered()
	}
	return false
}

func getChrome(n Node) func(Rect) string {
	type chromer interface{ chrome() func(Rect) string }
	if c, ok := n.(chromer); ok {
		return c.chrome()
	}
	return nil
}
