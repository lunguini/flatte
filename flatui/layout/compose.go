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
	caret       *caretPos
}

// caretPos is a hardware-cursor cell.
type caretPos struct{ x, y int }

// Cursor reports where the hardware cursor belongs: at the caret of the
// topmost focused Input whose caret cell is visible. ok is false when no
// focused Input is painted.
func (c Composition) Cursor() (x, y int, ok bool) {
	if c.caret == nil {
		return 0, 0, false
	}
	return c.caret.x, c.caret.y, true
}

// hit is one hit-testable region in paint order. An empty id marks an
// overlay layer: it absorbs hits that reach it, so a modal blocks the base
// painted underneath even where it has no ID'd child. area is the part of
// rect that is visible (rect clipped by enclosing Scrolls); only it is hittable.
type hit struct {
	id   string
	rect Rect
	area Rect
}

// HitTest returns the ID and solved rect of the topmost ID'd node containing
// cell (x, y): the last one painted, so children beat parents and overlays
// beat the base. The rect is that node's own (not the last-wins entry in
// Rects when IDs repeat), and whole even where a Scroll clips it, so
// x-rect.X and y-rect.Y are node-local coordinates. It returns "" and a zero
// Rect outside the frame, over an overlay area with no ID'd node, or where no
// ID'd node is painted.
func (c Composition) HitTest(x, y int) (string, Rect) {
	for i := len(c.hits) - 1; i >= 0; i-- {
		if h := c.hits[i]; h.area.Contains(x, y) {
			if h.id == "" {
				return "", Rect{}
			}
			return h.id, h.rect
		}
	}
	return "", Rect{}
}

// Compose is SolveAndCompose returning a Composition, which adds hit-testing
// in paint order to the frame, rects, and diagnostics.
func Compose(root Node, width, height int) Composition {
	if width <= 0 || height <= 0 {
		return Composition{Rects: map[string]Rect{}}
	}
	buf := uv.NewScreenBuffer(width, height)
	rects := make(map[string]Rect)
	state := walkState{rects: rects, first: make(map[string]Rect), clip: Rect{0, 0, width, height}}
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
	return Composition{Frame: buf.Render(), Rects: rects, Diagnostics: state.diagnostics, hits: state.hits, caret: state.caret}
}

type walkState struct {
	rects       map[string]Rect
	first       map[string]Rect
	diagnostics []DuplicateIDDiagnostic
	hits        []hit
	// clip is the visible region: the frame, narrowed by each enclosing
	// Scroll's window. Painting and hit areas are confined to it.
	clip Rect
	// caret is the last visible focused Input caret in paint order.
	caret *caretPos
}

type caretNode interface {
	caret(r Rect) (x, y int, ok bool)
}

// walkOverlay walks an overlay layer after recording it as a hit barrier, so
// the layer's own nodes (recorded during the walk) sit above the barrier and
// the barrier sits above everything painted before it.
func walkOverlay(o Node, r Rect, buf *uv.ScreenBuffer, state *walkState) {
	state.hits = append(state.hits, hit{rect: r, area: r.Intersect(state.clip)})
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
		state.hits = append(state.hits, hit{id: id, rect: r, area: r.Intersect(state.clip)})
	}

	if s, ok := n.(scroller); ok {
		walkScroll(s, r, buf, state)
		return
	}

	container, isContainer := n.(childContainer)
	if !isContainer {
		// Leaf: paint its render at r. paintClipped clips to r, so an over-wide
		// or over-tall string is bounded to the rect it was given, and to the
		// visible region, so a leaf scrolled out of view is not rendered.
		if buf != nil {
			paintClipped(*buf, r, state.clip, n.Render)
		}
		if cn, ok := n.(caretNode); ok {
			if x, y, ok := cn.caret(r); ok && state.clip.Contains(x, y) {
				state.caret = &caretPos{x, y}
			}
		}
		return
	}

	paintChrome(n, r, buf, state)

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

// paintChrome paints a container's own border/background chrome (if any)
// before its children are painted on top inside the inset region. A custom
// Chrome painter replaces the default Pad/Box fill and is painted even with
// zero inset; the layout inset itself still comes from Pad and the border.
func paintChrome(n Node, r Rect, buf *uv.ScreenBuffer, state *walkState) {
	if buf == nil {
		return
	}
	if ch := getChrome(n); ch != nil {
		paintClipped(*buf, r, state.clip, ch)
	} else if box := getBox(n); getInsetSides(n).any() || box.hasBackground() {
		paintClipped(*buf, r, state.clip, func(r Rect) string {
			return fillBox("", r, getPadSides(n), box)
		})
	}
}

// paintClipped paints render(r) at r, confined to clip. A rect wholly inside
// clip takes the direct path; one wholly outside is not rendered; one cut by
// the clip edge is drawn into a scratch buffer whose visible window is copied
// over, so styling that starts above the window still applies inside it.
func paintClipped(buf uv.ScreenBuffer, r, clip Rect, render func(Rect) string) {
	vis := r.Intersect(clip)
	if vis.W <= 0 || vis.H <= 0 {
		return
	}
	if vis == r {
		drawInto(buf, render(r), r)
		return
	}
	s := render(r)
	if s == "" {
		return
	}
	// Rows below the window are never needed; rows above it are, since a
	// style can open there and carry on into the window.
	dx, dy := vis.X-r.X, vis.Y-r.Y
	scratch := uv.NewScreenBuffer(r.W, dy+vis.H)
	uv.NewStyledString(s).Draw(scratch, scratch.Bounds())
	for y := 0; y < vis.H; y++ {
		for x := 0; x < vis.W; x++ {
			buf.SetCell(vis.X+x, vis.Y+y, nil)
		}
	}
	for y := 0; y < vis.H; y++ {
		for x := 0; x < vis.W; {
			c := scratch.CellAt(dx+x, dy+y)
			if c == nil || c.IsZero() {
				x++
				continue
			}
			buf.SetCell(vis.X+x, vis.Y+y, c)
			x += max(c.Width, 1)
		}
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

// getPadSides / getBox read chrome fields off a node's NodeBase for the
// container chrome paint. They mirror the insetSides accounting.
func getPadSides(n Node) sides {
	type padder interface{ padSides() sides }
	if p, ok := n.(padder); ok {
		return p.padSides()
	}
	return sides{}
}

func getBox(n Node) Box {
	type boxer interface{ box() Box }
	if b, ok := n.(boxer); ok {
		return b.box()
	}
	return Box{}
}

func getChrome(n Node) func(Rect) string {
	type chromer interface{ chrome() func(Rect) string }
	if c, ok := n.(chromer); ok {
		return c.chrome()
	}
	return nil
}
