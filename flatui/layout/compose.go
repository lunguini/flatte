package layout

import (
	"image"
	"image/color"

	"github.com/charmbracelet/colorprofile"
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
	// TerminalImages are image sources whose content boxes are wholly visible
	// in this composition and are not covered by a later layout overlay.
	// Terminal hosts can draw these over Frame with a pixel-image protocol;
	// Frame itself always retains its half-block/text fallback.
	TerminalImages []TerminalImagePlacement
	hits           []hit
	caret          *caretPos
	inputs         map[string]placedInput
}

// TerminalImagePlacement is an Image source that is fully visible in the
// terminal cell composition. Rect is the clipped content box in frame cells;
// Source and Fit are the original node values for the terminal renderer.
type TerminalImagePlacement struct {
	Rect   Rect
	Source image.Image
	Fit    ImageFit
}

// placedInput is an ID'd Input as painted, for caret hit-testing.
type placedInput struct {
	in Input
	r  Rect
}

// CaretAt reports, for cell (x, y) over an ID'd Input (the HitTest target),
// the byte offset into its Value of the caret position at that cell: the
// boundary before the grapheme there, or the end of the row past its text.
func (c Composition) CaretAt(x, y int) (offset int, ok bool) {
	id, _ := c.HitTest(x, y)
	pi, ok := c.inputs[id]
	if id == "" || !ok {
		return 0, false
	}
	return pi.in.caretAt(pi.r, x, y), true
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
	return ComposeWithColorProfile(root, width, height, colorprofile.Unknown)
}

// ComposeWithColorProfile composes a terminal frame using the output's color
// profile. Colored profiles keep image RGB values in the styled cell buffer
// for the terminal renderer to downsample; ASCII and NoTTY images use Text.
// Unknown leaves RGB styles intact, which is useful for composition before a
// concrete output terminal is selected.
func ComposeWithColorProfile(root Node, width, height int, profile colorprofile.Profile) Composition {
	if width <= 0 || height <= 0 {
		return Composition{Rects: map[string]Rect{}}
	}
	p := &cellPainter{buf: uv.NewScreenBuffer(width, height), profile: profile}
	s := solveTree[int](cells{}, root, width, height, p)
	c := Composition{
		Rects: cellRects(s), Diagnostics: cellDiagnostics(s),
		TerminalImages: p.visibleTerminalImages(), caret: p.caret, inputs: p.inputs,
	}
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
	// profile is the output terminal's color depth. Image nodes use it only to
	// select the Text fallback for terminals without color.
	profile colorprofile.Profile
	// caret is the last visible focused Input caret in paint order.
	caret *caretPos
	// inputs are the ID'd Inputs painted, by ID (the last one wins).
	inputs map[string]placedInput
	images []terminalImageCandidate
	layers []orderedRect
	order  int
}

type terminalImageCandidate struct {
	placement TerminalImagePlacement
	order     int
}

type orderedRect struct {
	rect  Rect
	order int
}

func (p *cellPainter) leaf(n Node, rr, cc rect[int]) {
	r, clip := Rect(rr), Rect(cc)
	p.order++
	// A Box's background is behind its content: leaf cells with no
	// background of their own keep the one painted underneath.
	vis := r.Intersect(clip)
	under := backgrounds(p.buf, vis)
	// paintClipped bounds an over-wide or over-tall string to its rect and
	// skips a leaf scrolled out of view.
	render := n.Render
	if image, ok := n.(interface {
		renderWithColorProfile(Rect, colorprofile.Profile) string
	}); ok {
		render = func(rect Rect) string { return image.renderWithColorProfile(rect, p.profile) }
	}
	paintClipped(p.buf, r, clip, render)
	if placement, ok := terminalImagePlacement(n, r); ok {
		visible := placement.Rect.Intersect(clip).Intersect(Rect{W: p.buf.Width(), H: p.buf.Height()})
		if placement.Rect.W > 0 && placement.Rect.H > 0 && visible == placement.Rect {
			p.images = append(p.images, terminalImageCandidate{placement: placement, order: p.order})
		}
	}
	if under != nil {
		keepBackgrounds(p.buf, vis, under)
	}
	if in, ok := n.(Input); ok && in.ID != "" {
		if p.inputs == nil {
			p.inputs = map[string]placedInput{}
		}
		p.inputs[in.ID] = placedInput{in, r}
	}
	if cn, ok := n.(caretNode); ok {
		if x, y, ok := cn.caret(r); ok && clip.Contains(x, y) {
			p.caret = &caretPos{x, y}
		}
	}
}

// backgrounds returns the background of each cell of r in row order, or nil
// when none has one (the common case, which then costs one scan).
func backgrounds(buf uv.ScreenBuffer, r Rect) []color.Color {
	if r.W <= 0 || r.H <= 0 {
		return nil
	}
	var bgs []color.Color
	for y := range r.H {
		for x := range r.W {
			c := buf.CellAt(r.X+x, r.Y+y)
			if c == nil || c.Style.Bg == nil {
				continue
			}
			if bgs == nil {
				bgs = make([]color.Color, r.W*r.H)
			}
			bgs[y*r.W+x] = c.Style.Bg
		}
	}
	return bgs
}

// keepBackgrounds gives each cell of r that has no background the one it
// had before (see backgrounds).
func keepBackgrounds(buf uv.ScreenBuffer, r Rect, under []color.Color) {
	for y := range r.H {
		for x := 0; x < r.W; {
			c := buf.CellAt(r.X+x, r.Y+y)
			if c == nil {
				x++
				continue
			}
			if bg := under[y*r.W+x]; c.Style.Bg == nil && bg != nil && c.Width > 0 {
				kept := *c
				kept.Style.Bg = bg
				buf.SetCell(r.X+x, r.Y+y, &kept)
			}
			x += max(c.Width, 1)
		}
	}
}

func (p *cellPainter) chrome(n Node, r, clip rect[int]) {
	p.order++
	paintChrome(n, Rect(r), p.buf, Rect(clip))
}

func (p *cellPainter) layer(r rect[int]) {
	p.order++
	visible := Rect(r).Intersect(Rect{W: p.buf.Width(), H: p.buf.Height()})
	if visible.W > 0 && visible.H > 0 {
		p.layers = append(p.layers, orderedRect{rect: visible, order: p.order})
	}
	p.buf.FillArea(&uv.EmptyCell, uv.Rect(r.X, r.Y, r.W, r.H))
}

// terminalImagePlacement is the picture a leaf in r asks a terminal that
// shows images to draw: an Image's source, or an icon button's shape.
func terminalImagePlacement(n Node, r Rect) (TerminalImagePlacement, bool) {
	if img, ok := terminalImageNode(n); ok && !isNilImage(img.Source) {
		return TerminalImagePlacement{Rect: r.insetBy(img.insetSides()), Source: img.Source, Fit: img.Fit}, true
	}
	var b Button
	switch button := n.(type) {
	case Button:
		b = button
	case *Button:
		if button == nil {
			return TerminalImagePlacement{}, false
		}
		b = *button
	default:
		return TerminalImagePlacement{}, false
	}
	if icon, content, ok := b.terminalIcon(r); ok {
		return TerminalImagePlacement{Rect: content, Source: icon, Fit: ImageContain}, true
	}
	return TerminalImagePlacement{}, false
}

func terminalImageNode(n Node) (Image, bool) {
	switch image := n.(type) {
	case Image:
		return image, true
	case *Image:
		if image != nil {
			return *image, true
		}
	}
	return Image{}, false
}

func (p *cellPainter) visibleTerminalImages() []TerminalImagePlacement {
	if len(p.images) == 0 {
		return nil
	}
	out := make([]TerminalImagePlacement, 0, len(p.images))
	for _, image := range p.images {
		covered := false
		for _, layer := range p.layers {
			if layer.order > image.order && rectsOverlap(layer.rect, image.placement.Rect) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, image.placement)
		}
	}
	return out
}

func rectsOverlap(a, b Rect) bool {
	intersection := a.Intersect(b)
	return intersection.W > 0 && intersection.H > 0
}

// paintChrome paints a container's own border/background chrome (if any)
// before its children are painted on top inside the inset region. A custom
// Chrome painter replaces the default Pad/Box fill and is painted even with
// zero inset; the layout inset itself still comes from Pad and the border.
func paintChrome(n Node, r Rect, buf uv.ScreenBuffer, clip Rect) {
	if ch := getChrome(n); ch != nil {
		paintClipped(buf, r, clip, ch)
	} else if box := getBox(n); getInsetSides(n).any() || box.hasBackground() {
		// Padding without a background of its own must not blank the one
		// painted underneath (a padded row inside a card).
		vis := r.Intersect(clip)
		under := backgrounds(buf, vis)
		paintClipped(buf, r, clip, func(r Rect) string {
			return fillBox("", r, getPadSides(n), box)
		})
		if under != nil {
			keepBackgrounds(buf, vis, under)
		}
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
