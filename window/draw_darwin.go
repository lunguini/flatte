package window

import (
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte/flatui/layout"
)

type rgba struct{ R, G, B, A float64 }

func rgbaOf(c color.Color) rgba {
	r, g, b, a := c.RGBA()
	return rgba{float64(r) / 0xffff, float64(g) / 0xffff, float64(b) / 0xffff, float64(a) / 0xffff}
}

// theme is the window's look. Roles and Boxes map onto it; explicit colors
// in a TextStyle or Box override it.
type theme struct {
	// surface is the raised fill of an overlay's box.
	bg, surface, text, subtle, title, border, field, caret, marked rgba
}

var darkTheme = theme{
	bg:      rgba{0.118, 0.122, 0.133, 1},
	surface: rgba{0.161, 0.165, 0.180, 1},
	text:    rgba{0.871, 0.878, 0.898, 1},
	subtle:  rgba{0.553, 0.569, 0.608, 1},
	title:   rgba{0.949, 0.953, 0.961, 1},
	border:  rgba{0.298, 0.306, 0.333, 1},
	field:   rgba{0.090, 0.094, 0.106, 1},
	caret:   rgba{0.400, 0.655, 1.000, 1},
	marked:  rgba{0.400, 0.655, 1.000, 1},
}

// Type sizes and spacing in points. Body text sets the line metric; the
// cell grid uses the user's fixed-pitch font at the same size.
const (
	bodySize  = 13.0
	titleSize = 15.0
	// lineSpacing is a line's height over its font's, the room text needs
	// between lines (a CSS line-height); text is centered in the line.
	lineSpacing = 1.25
	stepPoints  = 8.0
	// borderBand is the space a border takes on each side, as a border
	// takes a cell in the terminal; its stroke runs down the middle.
	borderBand  = 12.0
	strokeWidth = 1.0
	fieldInset  = 4.0
	// NSFontWeightSemibold and NSFontWeightBold.
	nsFontWeightSemibold = 0.3
	nsFontWeightBold     = 0.4
)

// radii of the border kinds: Normal is a slightly rounded hairline,
// Rounded is visibly round.
func radiusOf(k layout.BorderKind) float64 {
	if k == layout.BorderNormal {
		return 4
	}
	return 8
}

type fontKey struct {
	role               layout.TextRole
	bold, italic, mono bool
}

type font struct {
	ref                      uintptr
	ascent, descent, leading float64
}

func (f font) lineHeight() float64 { return f.ascent + f.descent + f.leading }

// pitch is the height of one line of f's text, on whole points.
func (f font) pitch() float64 { return math.Round(f.lineHeight() * lineSpacing) }

var (
	fontsMu sync.Mutex
	fonts   = map[fontKey]font{}
)

// fontFor returns the CoreText font for a role and emphasis; fonts are
// created once and live for the process.
func fontFor(k fontKey) font {
	fontsMu.Lock()
	defer fontsMu.Unlock()
	if f, ok := fonts[k]; ok {
		return f
	}
	var ref uintptr
	bold := k.bold // the bold trait, unless the weight already is bold
	switch {
	case k.mono:
		ref = ctFontCreateUIFontForLanguage(ctFontUserFixedPitch, bodySize, 0)
	case k.role == layout.TextTitle:
		// Titles are semibold, bold ones bold. Neither weight is a
		// CTFontUIFontType (and the bold trait does not apply over
		// semibold), so they come from NSFont, which is toll-free bridged.
		weight := nsFontWeightSemibold
		if k.bold {
			weight, bold = nsFontWeightBold, false
		}
		withPool(func() {
			f := objc.Send[objc.ID](class("NSFont"), sel("systemFontOfSize:weight:"), titleSize, weight)
			ref = uintptr(f.Send(sel("retain")))
		})
	default:
		ref = ctFontCreateUIFontForLanguage(ctFontSystem, bodySize, 0)
	}
	var traits uint32
	if bold {
		traits |= ctTraitBold
	}
	if k.italic {
		traits |= ctTraitItalic
	}
	if traits != 0 {
		if styled := ctFontCreateCopyWithSymbolicTr(ref, 0, nil, traits, traits); styled != 0 {
			cfRelease(ref)
			ref = styled
		}
	}
	f := font{ref: ref, ascent: ctFontGetAscent(ref), descent: ctFontGetDescent(ref), leading: ctFontGetLeading(ref)}
	fonts[k] = f
	return f
}

func styleFont(s layout.TextStyle) font {
	return fontFor(fontKey{role: s.Role, bold: s.Bold, italic: s.Italic})
}

// textLine is a shaped CoreText line. Call release.
type textLine struct {
	ref   uintptr
	width float64
}

// shape lays out text in f; color comes from the context fill at draw time.
func shape(text string, f font) textLine {
	var l textLine
	withPool(func() {
		attrs := class("NSMutableDictionary").Send(sel("dictionary"))
		attrs.Send(sel("setObject:forKey:"), objc.ID(f.ref), kCTFontAttributeName)
		attrs.Send(sel("setObject:forKey:"), class("NSNumber").Send(sel("numberWithBool:"), true), kCTForegroundFromCtx)
		attr := alloc("NSAttributedString").Send(sel("initWithString:attributes:"), nsString(text), attrs)
		l.ref = ctLineCreateWithAttributedString(attr)
		attr.Send(sel("release"))
	})
	l.width = ctLineGetTypographicBounds(l.ref, nil, nil, nil)
	return l
}

func (l textLine) release() { cfRelease(l.ref) }

// offset is the x offset of the byte index i of text in l.
func (l textLine) offset(text string, i int) float64 {
	i = min(max(i, 0), len(text))
	return ctLineGetOffsetForStringIndex(l.ref, utf16Len(text[:i]), nil)
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// metrics is the layout metric for this machine's fonts: columns are the
// fixed-pitch advance, lines the body font's pitch, a step 8pt, a border a
// band of borderBand.
func metrics() layout.Metrics {
	mono := fontFor(fontKey{mono: true})
	zero := shape("0", mono)
	defer zero.release()
	return layout.Metrics{
		Column: zero.width,
		Line:   fontFor(fontKey{}).pitch(),
		Step:   stepPoints,
		Border: borderBand,
		MeasureText: func(text string, style layout.TextStyle) (float64, float64) {
			f := styleFont(style)
			lines := strings.Split(text, "\n")
			w := 0.0
			for _, s := range lines {
				l := shape(s, f)
				w = max(w, l.width)
				l.release()
			}
			return w, float64(len(lines)) * f.pitch()
		},
	}
}

// painter draws a scene into a CoreGraphics context whose user space is
// top-left origin points (a flipped view, or a flipped bitmap).
type painter struct {
	ctx uintptr
	m   layout.Metrics
	th  theme
	// scale is device pixels per point, for snapping the cell grid.
	scale float64
	// marked is the input method's uncommitted text, shown at the focused
	// input's caret.
	marked string
	// caret is where the focused input's caret was drawn (for the input
	// method's candidate window); ok reports whether one was.
	caret   CGRect
	caretOK bool
	// layer is the rect of the overlay layer just started: its box, the
	// next op, is a raised surface.
	layer   layout.PointRect
	inLayer bool
}

func (p *painter) fill(c rgba)   { cgContextSetRGBFillColor(p.ctx, c.R, c.G, c.B, c.A) }
func (p *painter) stroke(c rgba) { cgContextSetRGBStrokeColor(p.ctx, c.R, c.G, c.B, c.A) }

func rectOf(r layout.PointRect) CGRect { return cgRect(r.X, r.Y, r.W, r.H) }

func intersect(a, b layout.PointRect) layout.PointRect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	return layout.PointRect{X: x0, Y: y0, W: max(x1-x0, 0), H: max(y1-y0, 0)}
}

// clipped runs f with drawing clipped to r.
func (p *painter) clipped(r layout.PointRect, f func()) {
	cgContextSaveGState(p.ctx)
	cgContextClipToRect(p.ctx, rectOf(r))
	f()
	cgContextRestoreGState(p.ctx)
}

func (p *painter) draw(s layout.Scene, width, height float64) {
	p.fill(p.th.bg)
	cgContextFillRect(p.ctx, cgRect(0, 0, width, height))
	// The space is flipped; flip glyphs back.
	cgContextSetTextMatrix(p.ctx, CGAffineTransform{A: 1, D: -1})
	for _, op := range s.Ops {
		_, layer := op.(layout.LayerOp)
		_, box := op.(layout.BoxOp)
		if !layer && !box {
			p.inLayer = false
		}
		switch op := op.(type) {
		case layout.LayerOp:
			p.fill(p.th.bg)
			cgContextFillRect(p.ctx, rectOf(op.Rect))
			p.layer, p.inLayer = op.Rect, true
		case layout.BoxOp:
			p.clipped(op.Clip, func() { p.box(op) })
		case layout.TextOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.text(op) })
		case layout.InputOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.input(op) })
		case layout.CellsOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.cells(op) })
		}
	}
}

func (p *painter) roundedPath(r CGRect, rad float64, f func()) {
	path := cgPathCreateWithRoundedRect(r, rad, rad, nil)
	cgContextAddPath(p.ctx, path)
	f()
	cgPathRelease(path)
}

// box draws a Box. A border is a rounded stroke down the middle of its
// band, with the background filling inside it; the first box of an overlay
// layer is a raised surface.
func (p *painter) box(op layout.BoxOp) {
	raised := p.inLayer && op.Rect == p.layer
	p.inLayer = false
	bg, hasBg := rgba{}, false
	if op.Box.Background != nil && op.Box.Background != color.Transparent {
		bg, hasBg = rgbaOf(op.Box.Background), true
	} else if raised {
		bg, hasBg = p.th.surface, true
	}
	if op.Box.Border == layout.BorderNone {
		if hasBg {
			p.fill(bg)
			cgContextFillRect(p.ctx, rectOf(op.Rect))
		}
		return
	}
	in := strokeRect(op.Rect, p.m.Border)
	rad := min(radiusOf(op.Box.Border), in.Size.Width/2, in.Size.Height/2)
	if hasBg {
		p.fill(bg)
		p.roundedPath(in, rad, func() { cgContextFillPath(p.ctx) })
	}
	c := p.th.border
	if op.Box.BorderColor != nil {
		c = rgbaOf(op.Box.BorderColor)
	}
	p.stroke(c)
	cgContextSetLineWidth(p.ctx, strokeWidth)
	p.roundedPath(in, rad, func() { cgContextStrokePath(p.ctx) })
}

// strokeRect is the centerline of a border stroke: half a band inside r,
// on half points so a 1pt stroke covers whole pixels at 1× and 2×.
func strokeRect(r layout.PointRect, band float64) CGRect {
	x0, y0 := math.Floor(r.X+band/2)+0.5, math.Floor(r.Y+band/2)+0.5
	x1, y1 := math.Ceil(r.X+r.W-band/2)-0.5, math.Ceil(r.Y+r.H-band/2)-0.5
	if x1 <= x0 || y1 <= y0 {
		return cgRect(r.X+0.5, r.Y+0.5, max(r.W-1, 0), max(r.H-1, 0))
	}
	return cgRect(x0, y0, x1-x0, y1-y0)
}

func (p *painter) roleColor(s layout.TextStyle) rgba {
	if s.Fg != nil {
		return rgbaOf(s.Fg)
	}
	switch s.Role {
	case layout.TextTitle:
		return p.th.title
	case layout.TextSubtle:
		return p.th.subtle
	}
	return p.th.text
}

// line draws one line of text from x in the row of height h at y, centered
// vertically; a background fills the row behind it.
func (p *painter) line(text string, f font, x, y, h float64, fg rgba, bg color.Color, underline, strike bool) float64 {
	l := shape(text, f)
	defer l.release()
	if bg != nil {
		p.fill(rgbaOf(bg))
		cgContextFillRect(p.ctx, cgRect(x, y, l.width, h))
	}
	p.fill(fg)
	base := y + (h-f.lineHeight())/2 + f.ascent
	cgContextSetTextPosition(p.ctx, x, base)
	ctLineDraw(l.ref, p.ctx)
	if underline {
		t := max(ctFontGetUnderlineThickness(f.ref), 1)
		cgContextFillRect(p.ctx, cgRect(x, base-ctFontGetUnderlinePosition(f.ref), l.width, t))
	}
	if strike {
		t := max(ctFontGetUnderlineThickness(f.ref), 1)
		cgContextFillRect(p.ctx, cgRect(x, base-ctFontGetXHeight(f.ref)/2-t/2, l.width, t))
	}
	return l.width
}

func (p *painter) text(op layout.TextOp) {
	f := styleFont(op.Style)
	h := f.pitch()
	for i, s := range strings.Split(op.Text, "\n") {
		p.line(s, f, op.Rect.X, op.Rect.Y+float64(i)*h, h, p.roleColor(op.Style), op.Style.Bg, op.Style.Underline, op.Style.Strikethrough)
	}
}

// input draws a text field: an outlined, filled rounded rect (outlined in
// the focus color when focused) with the value (or the placeholder),
// scrolled so the caret stays visible, and the caret and any input-method
// text when focused.
func (p *painter) input(op layout.InputOp) {
	r := op.Rect
	field := cgRect(r.X+0.5, r.Y+0.5, max(r.W-1, 0), max(r.H-1, 0))
	rad := min(4, field.Size.Height/2)
	p.fill(p.th.field)
	p.roundedPath(field, rad, func() { cgContextFillPath(p.ctx) })
	outline := p.th.border
	if op.Focused {
		outline = p.th.caret
	}
	p.stroke(outline)
	cgContextSetLineWidth(p.ctx, strokeWidth)
	p.roundedPath(field, rad, func() { cgContextStrokePath(p.ctx) })
	f := styleFont(op.Style)
	y := r.Y + (r.H-f.lineHeight())/2
	x0 := r.X + fieldInset
	avail := max(r.W-2*fieldInset, 1)
	if op.Value == "" && (!op.Focused || p.marked == "") {
		p.line(op.Placeholder, f, x0, r.Y, r.H, p.th.subtle, nil, false, false)
		if op.Focused {
			p.caretAt(x0, r.Y, r.H, f)
		}
		return
	}
	caret := min(max(op.Caret, 0), len(op.Value))
	for caret > 0 && caret < len(op.Value) && !runeStart(op.Value, caret) {
		caret--
	}
	text, markStart, markEnd := op.Value, caret, caret
	if op.Focused && p.marked != "" {
		text = op.Value[:caret] + p.marked + op.Value[caret:]
		markEnd = caret + len(p.marked)
	}
	l := shape(text, f)
	caretX := l.offset(text, markEnd)
	l.release()
	scroll := 0.0
	if op.Focused && caretX > avail-2 {
		scroll = caretX - avail + 2
	}
	p.line(text, f, x0-scroll, r.Y, r.H, p.roleColor(op.Style), nil, op.Style.Underline, op.Style.Strikethrough)
	if markEnd > markStart {
		ml := shape(text, f)
		a, b := ml.offset(text, markStart), ml.offset(text, markEnd)
		ml.release()
		p.fill(p.th.marked)
		cgContextFillRect(p.ctx, cgRect(x0-scroll+a, y+f.lineHeight()-1.5, b-a, 1.5))
	}
	if op.Focused {
		p.caretAt(x0-scroll+caretX, r.Y, r.H, f)
	}
}

// runeStart reports whether byte i of s starts a rune (or is the end).
func runeStart(s string, i int) bool { return i == len(s) || s[i]&0xC0 != 0x80 }

// caretAt draws the caret at x in the row of height h at y, as tall as f's
// text and centered like it.
func (p *painter) caretAt(x, y, h float64, f font) {
	p.caret = cgRect(x, y+(h-f.lineHeight())/2, 1.5, f.lineHeight())
	p.caretOK = true
	p.fill(p.th.caret)
	cgContextFillRect(p.ctx, p.caret)
}

// cells draws terminal content as a monospace grid: runs of same-styled
// cells as one shaped line each, wide cells on their own so the grid stays
// aligned whatever the font's advance for them.
func (p *painter) cells(op layout.CellsOp) {
	buf := uv.NewScreenBuffer(op.Cols, op.Lines)
	uv.NewStyledString(op.Content).Draw(buf, buf.Bounds())
	for y := range op.Lines {
		top := op.Rect.Y + float64(y)*p.m.Line
		x := 0
		for x < op.Cols {
			c := buf.CellAt(x, y)
			if c == nil || c.Width == 0 {
				x++
				continue
			}
			cx := op.Rect.X + float64(x)*p.m.Column
			if g, ok := gridGlyph(c, cx, top, float64(max(c.Width, 1))*p.m.Column, p.m.Line); ok {
				p.cellShape(g, c.Style, cx, top, float64(max(c.Width, 1))*p.m.Column)
				x += max(c.Width, 1)
				continue
			}
			start, style := x, c.Style
			var run strings.Builder
			for x < op.Cols {
				cc := buf.CellAt(x, y)
				if cc == nil || !cc.Style.Equal(&style) || (cc.Width > 1 && x > start) {
					break
				}
				if _, ok := gridGlyph(cc, 0, 0, 1, 1); ok {
					break
				}
				if cc.Width == 0 {
					x++
					continue
				}
				run.WriteString(cellText(cc))
				x += max(cc.Width, 1)
				if cc.Width > 1 {
					break
				}
			}
			p.cellRun(run.String(), style, op.Rect.X+float64(start)*p.m.Column, top, float64(x-start)*p.m.Column)
		}
	}
}

func cellText(c *uv.Cell) string {
	if c.Content == "" {
		return " "
	}
	return c.Content
}

// cellColors resolves a cell style's colors over the theme.
func (p *painter) cellColors(st uv.Style) (fg, bg rgba, hasBg bool) {
	fg = p.th.text
	if st.Fg != nil {
		fg = rgbaOf(st.Fg)
	}
	if st.Bg != nil {
		bg, hasBg = rgbaOf(st.Bg), true
	}
	if st.Attrs&uv.AttrReverse != 0 {
		if !hasBg {
			bg = p.th.bg
		}
		fg, bg, hasBg = bg, fg, true
	}
	if st.Attrs&uv.AttrFaint != 0 {
		fg.A *= 0.6
	}
	return fg, bg, hasBg
}

// cellFill fills a cell-grid rect with its edges snapped to device pixels.
// Columns are fractional, so unsnapped fills would leave each shared edge
// pixel half-covered twice: a visible seam between neighboring cells.
func (p *painter) cellFill(c rgba, x, y, w, h float64) {
	x0, y0, x1, y1 := p.snap(x), p.snap(y), p.snap(x+w), p.snap(y+h)
	p.fill(c)
	cgContextFillRect(p.ctx, cgRect(x0, y0, x1-x0, y1-y0))
}

// snap rounds v (points) to the nearest device pixel.
func (p *painter) snap(v float64) float64 {
	s := p.scale
	if !(s > 0) {
		s = 1
	}
	return math.Round(v*s) / s
}

func (p *painter) cellRun(text string, st uv.Style, x, y, w float64) {
	fg, bg, hasBg := p.cellColors(st)
	if hasBg {
		p.cellFill(bg, x, y, w, p.m.Line)
	}
	if strings.TrimSpace(text) == "" || st.Attrs&uv.AttrConceal != 0 {
		return
	}
	f := fontFor(fontKey{mono: true, bold: st.Attrs&uv.AttrBold != 0, italic: st.Attrs&uv.AttrItalic != 0})
	p.line(text, f, x, y, p.m.Line, fg, nil, st.Underline != uv.UnderlineNone, st.Attrs&uv.AttrStrikethrough != 0)
}

// gridGlyph is the shape of a grid cell drawn as one (see cellGlyph).
func gridGlyph(c *uv.Cell, x, y, w, h float64) (cellShape, bool) {
	r, size := utf8.DecodeRuneInString(c.Content)
	if size == 0 || size != len(c.Content) {
		return cellShape{}, false
	}
	return cellGlyph(r, x, y, w, h)
}

// cellShape draws a line-drawing or block cell in its colors.
func (p *painter) cellShape(g cellShape, st uv.Style, x, y, w float64) {
	fg, bg, hasBg := p.cellColors(st)
	if hasBg {
		p.cellFill(bg, x, y, w, p.m.Line)
	}
	if st.Attrs&uv.AttrConceal != 0 {
		return
	}
	for _, r := range g.rects {
		c := fg
		c.A *= r.Alpha
		p.cellFill(c, r.X, r.Y, r.W, r.H)
	}
	p.fill(fg)
	p.stroke(fg)
	cgContextSetLineWidth(p.ctx, strokeWidth)
	for _, a := range g.arcs {
		cgContextMoveToPoint(p.ctx, a.from.X, a.from.Y)
		cgContextAddArcToPoint(p.ctx, a.corner.X, a.corner.Y, a.to.X, a.to.Y, a.r)
		cgContextAddLineToPoint(p.ctx, a.to.X, a.to.Y)
		cgContextStrokePath(p.ctx)
	}
	for _, pl := range g.polys {
		cgContextMoveToPoint(p.ctx, pl.pts[0].X, pl.pts[0].Y)
		for _, pt := range pl.pts[1:] {
			cgContextAddLineToPoint(p.ctx, pt.X, pt.Y)
		}
		if pl.fill {
			cgContextClosePath(p.ctx)
			cgContextFillPath(p.ctx)
		} else {
			cgContextStrokePath(p.ctx)
		}
	}
}
