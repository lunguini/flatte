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
	p := &cellPainter{buf: uv.NewScreenBuffer(width, height)}
	s := solveTree[int](cells{}, root, width, height, p)
	c := Composition{Rects: cellRects(s), Diagnostics: cellDiagnostics(s), caret: p.caret}
	c.hits = make([]hit, len(s.hits))
	for i, h := range s.hits {
		c.hits[i] = hit{id: h.id, rect: Rect(h.rect), area: Rect(h.area)}
	}
	c.Frame = p.buf.Render()
	return c
}

func cellRects(s *solver[int]) map[string]Rect {
	rects := make(map[string]Rect, len(s.rects))
	for id, r := range s.rects {
		rects[id] = Rect(r)
	}
	return rects
}

func cellDiagnostics(s *solver[int]) []DuplicateIDDiagnostic {
	var out []DuplicateIDDiagnostic
	for _, d := range s.dups {
		out = append(out, DuplicateIDDiagnostic{ID: d.id, FirstRect: Rect(d.first), Duplicate: Rect(d.dupe)})
	}
	return out
}

type caretNode interface {
	caret(r Rect) (x, y int, ok bool)
}

// cellPainter composes the terminal frame for the cell solver: leaves are
// rendered and drawn at their solved cells, clipped to the visible region;
// overlay layers clear their rect first so the base cannot bleed through
// cells the overlay does not paint.
type cellPainter struct {
	buf uv.ScreenBuffer
	// caret is the last visible focused Input caret in paint order.
	caret *caretPos
}

func (p *cellPainter) leaf(n Node, rr, cc rect[int]) {
	r, clip := Rect(rr), Rect(cc)
	// paintClipped bounds an over-wide or over-tall string to its rect and
	// skips a leaf scrolled out of view.
	paintClipped(p.buf, r, clip, n.Render)
	if cn, ok := n.(caretNode); ok {
		if x, y, ok := cn.caret(r); ok && clip.Contains(x, y) {
			p.caret = &caretPos{x, y}
		}
	}
}

func (p *cellPainter) chrome(n Node, r, clip rect[int]) { paintChrome(n, Rect(r), p.buf, Rect(clip)) }

func (p *cellPainter) layer(r rect[int]) {
	p.buf.FillArea(&uv.EmptyCell, uv.Rect(r.X, r.Y, r.W, r.H))
}

// paintChrome paints a container's own border/background chrome (if any)
// before its children are painted on top inside the inset region. A custom
// Chrome painter replaces the default Pad/Box fill and is painted even with
// zero inset; the layout inset itself still comes from Pad and the border.
func paintChrome(n Node, r Rect, buf uv.ScreenBuffer, clip Rect) {
	if ch := getChrome(n); ch != nil {
		paintClipped(buf, r, clip, ch)
	} else if box := getBox(n); getInsetSides(n).any() || box.hasBackground() {
		paintClipped(buf, r, clip, func(r Rect) string {
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
