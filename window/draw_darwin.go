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

// rgbaOf is c with straight alpha, as Core Graphics colors take it
// (color.Color's RGBA is premultiplied).
func rgbaOf(c color.Color) rgba {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return rgba{}
	}
	return rgba{float64(r) / float64(a), float64(g) / float64(a), float64(b) / float64(a), float64(a) / 0xffff}
}

// theme is a Theme in drawing terms. Roles and Boxes map onto it; explicit
// colors in a TextStyle or Box override it.
type theme struct {
	// surface is the raised fill of an overlay's box; selection highlights
	// a Selected box; track is a Meter's unfilled bar (its fill defaults to
	// the caret's accent).
	bg, surface, text, subtle, title, border, field, caret, marked, selection, track rgba
	// family and mono are font families ("": the system's); body and
	// titleSize are point sizes; spacing is a line's height over its font's
	// (a CSS line-height: text is centered in the line); radius is a plain
	// border's corner radius (negative: square).
	family, mono                     string
	body, titleSize, spacing, radius float64
}

func newTheme(t Theme) theme {
	t = t.over(Dark())
	return theme{
		bg: rgbaOf(t.Background), surface: rgbaOf(t.Surface),
		text: rgbaOf(t.Text), subtle: rgbaOf(t.Subtle), title: rgbaOf(t.Title),
		border: rgbaOf(t.Border), field: rgbaOf(t.Field),
		caret: rgbaOf(t.Accent), marked: rgbaOf(t.Accent), selection: rgbaOf(t.Selection),
		track:  rgbaOf(t.Track),
		family: t.Font, mono: t.MonoFont,
		body: t.FontSize, titleSize: t.TitleSize, spacing: t.LineSpacing, radius: t.Radius,
	}
}

// darkTheme is the default theme, and the one tests draw with.
var darkTheme = newTheme(Dark())

// Spacing and stroke sizes in points.
const (
	stepPoints = 8.0
	// borderBand is the space a border takes on each side, as a border
	// takes a cell in the terminal; its stroke runs down the middle.
	borderBand  = 12.0
	strokeWidth = 1.0
	fieldInset  = 4.0
	// maxFontSize caps TextStyle.Size; sizes round to a quarter point so
	// the font cache stays small.
	maxFontSize = 512.0
	// CGLineCap and CGLineJoin values.
	cgLineCapButt   = 0
	cgLineCapRound  = 1
	cgLineJoinMiter = 0
	cgLineJoinRound = 1
	// NSFontWeightSemibold and NSFontWeightBold.
	nsFontWeightSemibold = 0.3
	nsFontWeightBold     = 0.4
)

// radiusOf is a border kind's corner radius: Normal is slightly rounded,
// Rounded twice as round.
func (th theme) radiusOf(k layout.BorderKind) float64 {
	switch {
	case th.radius < 0:
		return 0
	case k == layout.BorderNormal:
		return th.radius
	}
	return 2 * th.radius
}

type fontKey struct {
	role               layout.TextRole
	bold, italic, mono bool
	// size overrides the role's point size when positive.
	size float64
}

type font struct {
	ref                               uintptr
	ascent, descent, leading, spacing float64
}

func (f font) lineHeight() float64 { return f.ascent + f.descent + f.leading }

// pitch is the height of one line of f's text, on whole points.
func (f font) pitch() float64 { return math.Round(f.lineHeight() * f.spacing) }

// width is the advance width of s set in f.
func (f font) width(s string) float64 {
	l := shape(s, f)
	defer l.release()
	return l.width
}

// fontID is a font as a theme asks for it.
type fontID struct {
	fontKey
	family        string
	size, spacing float64
}

var (
	fontsMu sync.Mutex
	fonts   = map[fontID]font{}
)

// fontFor is darkTheme's font for a role and emphasis.
func fontFor(k fontKey) font { return darkTheme.font(k) }

// font returns the CoreText font for a role and emphasis in th's type;
// fonts are created once and live for the process.
func (th theme) font(k fontKey) font {
	id := fontID{fontKey: k, family: th.family, size: th.body, spacing: th.spacing}
	if k.mono {
		id.family = th.mono
	} else if k.role == layout.TextTitle {
		id.size = th.titleSize
	}
	// The size lives in id.size alone, so a bad one (NaN, ≤ 0, +Inf) never
	// reaches the cache key.
	if k.size > 0 && !math.IsInf(k.size, 1) {
		id.size = math.Round(min(k.size, maxFontSize)*4) / 4
	}
	id.fontKey.size = 0
	fontsMu.Lock()
	defer fontsMu.Unlock()
	if f, ok := fonts[id]; ok {
		return f
	}
	var ref uintptr
	bold := k.bold // the bold trait, unless the weight already is bold
	switch {
	case id.family != "":
		// A named family; titles are its bold face.
		bold = bold || (!k.mono && k.role == layout.TextTitle)
		withPool(func() { ref = ctFontCreateWithName(uintptr(nsString(id.family)), id.size, nil) })
	case k.mono:
		ref = ctFontCreateUIFontForLanguage(ctFontUserFixedPitch, id.size, 0)
	case k.role == layout.TextTitle:
		// System titles are semibold, bold ones bold. Neither weight is a
		// CTFontUIFontType (and the bold trait does not apply over
		// semibold), so they come from NSFont, which is toll-free bridged.
		weight := nsFontWeightSemibold
		if k.bold {
			weight, bold = nsFontWeightBold, false
		}
		withPool(func() {
			f := objc.Send[objc.ID](class("NSFont"), sel("systemFontOfSize:weight:"), id.size, weight)
			ref = uintptr(f.Send(sel("retain")))
		})
	default:
		ref = ctFontCreateUIFontForLanguage(ctFontSystem, id.size, 0)
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
	f := font{ref: ref, ascent: ctFontGetAscent(ref), descent: ctFontGetDescent(ref), leading: ctFontGetLeading(ref), spacing: id.spacing}
	fonts[id] = f
	return f
}

func (th theme) styleFont(s layout.TextStyle) font {
	return th.font(fontKey{role: s.Role, bold: s.Bold, italic: s.Italic, size: s.Size})
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

// metrics is the layout metric for darkTheme's type.
func metrics() layout.Metrics { return darkTheme.metrics() }

// metrics is the layout metric for th's fonts on this machine: columns are
// the fixed-pitch advance, lines the body font's pitch, a step 8pt, a border
// a band of borderBand.
func (th theme) metrics() layout.Metrics {
	zero := shape("0", th.font(fontKey{mono: true}))
	defer zero.release()
	return layout.Metrics{
		Column: zero.width,
		Line:   th.font(fontKey{}).pitch(),
		Step:   stepPoints,
		Border: borderBand,
		MeasureText: func(text string, style layout.TextStyle, maxWidth float64) (float64, float64) {
			f := th.styleFont(style)
			lines := wrapWords(text, maxWidth, f.width)
			w := 0.0
			for _, s := range lines {
				w = max(w, f.width(s))
			}
			return w, float64(len(lines)) * f.pitch()
		},
		MeasureRuns: func(runs []layout.Run, maxWidth float64) (float64, float64) {
			rt := th.richText(runs)
			w, h := 0.0, 0.0
			for _, sp := range rt.lines(maxWidth) {
				w = max(w, rt.width(sp.start, sp.end))
				h += rt.metrics(sp).pitch
			}
			return w, h
		},
	}
}

// richText is a rich-text Label's runs in th's fonts, as one text.
type richText struct {
	text  string
	runs  []layout.Run
	fonts []font
	// ends[i] is where run i ends in text.
	ends []int
}

func (th theme) richText(runs []layout.Run) richText {
	rt := richText{runs: runs, fonts: make([]font, len(runs)), ends: make([]int, len(runs))}
	var b strings.Builder
	for i, r := range runs {
		b.WriteString(r.Text)
		rt.fonts[i], rt.ends[i] = th.styleFont(r.Style), b.Len()
	}
	rt.text = b.String()
	return rt
}

// pieces calls f for each run's part of text[a:b], in order.
func (rt richText) pieces(a, b int, f func(run, a, b int)) {
	start := 0
	for i, end := range rt.ends {
		if lo, hi := max(a, start), min(b, end); lo < hi {
			f(i, lo, hi)
		}
		start = end
	}
}

// width is the width of text[a:b], each run in its font.
func (rt richText) width(a, b int) float64 {
	w := 0.0
	rt.pieces(a, b, func(i, a, b int) { w += rt.fonts[i].width(rt.text[a:b]) })
	return w
}

// lines wraps the text at maxWidth (only at "\n" when maxWidth <= 0).
func (rt richText) lines(maxWidth float64) []span {
	return wrapSpans(rt.text, maxWidth, rt.width)
}

// lineMetrics is a line's height and where its baseline goes: the tallest
// font on it sets both, so mixed sizes share a baseline.
type lineMetrics struct{ pitch, ascent, height float64 }

// metrics is sp's line metrics: of the fonts of the runs on it, or — on an
// empty line — of the run it sits in.
func (rt richText) metrics(sp span) lineMetrics {
	var m lineMetrics
	add := func(f font) {
		m.pitch, m.ascent, m.height = max(m.pitch, f.pitch()), max(m.ascent, f.ascent), max(m.height, f.lineHeight())
	}
	rt.pieces(sp.start, sp.end, func(i, _, _ int) { add(rt.fonts[i]) })
	if m.pitch == 0 && len(rt.fonts) > 0 {
		i := 0
		for i < len(rt.ends)-1 && rt.ends[i] < sp.start {
			i++
		}
		add(rt.fonts[i])
	}
	return m
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
	// caret is where the focused input's caret goes (for the input
	// method's candidate window, and paintCaret); ok reports whether there
	// is one; caretClip is the field it must stay inside.
	caret     CGRect
	caretOK   bool
	caretClip layout.PointRect
	// clip is the clip of the op being drawn.
	clip layout.PointRect
	// presentation is what the host draws itself: hovered and pressed
	// controls, selected label text.
	presentation
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
	p.clip = layout.PointRect{W: width, H: height}
	p.fill(p.th.bg)
	cgContextFillRect(p.ctx, cgRect(0, 0, width, height))
	// The space is flipped; flip glyphs back.
	cgContextSetTextMatrix(p.ctx, CGAffineTransform{A: 1, D: -1})
	beginImages()
	defer endImages()
	for i, op := range s.Ops {
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
			from, to := p.sel.in(i, op)
			p.clipped(intersect(op.Rect, op.Clip), func() { p.text(op, from, to) })
		case layout.InputOp:
			p.clip = intersect(op.Rect, op.Clip)
			p.clipped(p.clip, func() { p.input(op) })
		case layout.IconOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.icon(op) })
		case layout.ImageOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.image(op) })
		case layout.MeterOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.meter(op) })
		case layout.ControlOp:
			// Not clipped to its rect: a focus ring draws just outside it.
			p.clip = op.Clip
			p.clipped(op.Clip, func() { p.control(op) })
		case layout.ScrollBarOp:
			p.clipped(intersect(op.Rect, op.Clip), func() { p.scrollBar(op) })
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
// layer is a raised surface; a selected box is highlighted first.
func (p *painter) box(op layout.BoxOp) {
	raised := p.inLayer && op.Rect == p.layer
	p.inLayer = false
	if op.Box.Selected {
		r := rectOf(op.Rect)
		p.fill(p.th.selection)
		p.roundedPath(r, min(p.th.radiusOf(layout.BorderNormal), r.Size.Width/2, r.Size.Height/2), func() { cgContextFillPath(p.ctx) })
	}
	bg, hasBg := rgba{}, false
	if op.Box.Background != nil && op.Box.Background != color.Transparent {
		bg, hasBg = rgbaOf(op.Box.Background), true
	} else if raised {
		bg, hasBg = p.th.surface, true
	}
	if op.Box.Border == layout.BorderNone {
		switch {
		case hasBg && op.Box.Round:
			r := rectOf(op.Rect)
			p.fill(bg)
			p.roundedPath(r, min(r.Size.Height/2, r.Size.Width/2, 10), func() { cgContextFillPath(p.ctx) })
		case hasBg:
			// Snapped like cell fills: adjacent filled boxes meet on whole
			// pixels, with no seam at fractional edges.
			p.cellFill(bg, op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H)
		}
		return
	}
	in := strokeRect(op.Rect, p.m.Border)
	rad := min(p.th.radiusOf(op.Box.Border), in.Size.Width/2, in.Size.Height/2)
	if hasBg {
		p.fill(bg)
		p.roundedPath(in, rad, func() { cgContextFillPath(p.ctx) })
	}
	c := p.th.border
	if op.Box.BorderColor != nil {
		c = rgbaOf(op.Box.BorderColor)
	}
	stroke := func() {
		p.stroke(c)
		cgContextSetLineWidth(p.ctx, strokeWidth)
		p.roundedPath(in, rad, func() { cgContextStrokePath(p.ctx) })
	}
	if op.Box.Title == "" {
		stroke()
		return
	}
	p.legend(op, in, rad, stroke)
}

// titleGap is the space between a box title and the stroke broken around it.
const titleGap = 4.0

// legend draws a titled border: the title on the top stroke, past the
// corner, with the stroke broken around it (cut to fit the box).
func (p *painter) legend(op layout.BoxOp, in CGRect, rad float64, stroke func()) {
	f := p.th.styleFont(op.Box.TitleStyle)
	pitch := f.pitch()
	right := in.Origin.X + in.Size.Width - rad - titleGap
	gap0 := in.Origin.X + max(rad, titleGap) + titleGap
	gap1 := min(gap0+titleGap+f.width(op.Box.Title)+titleGap, right)
	if gap1 <= gap0+2*titleGap {
		stroke()
		return
	}
	r := op.Rect
	keep := [3]CGRect{
		cgRect(r.X, r.Y, gap0-r.X, r.H),
		cgRect(gap1, r.Y, r.X+r.W-gap1, r.H),
		cgRect(r.X, in.Origin.Y+strokeWidth, r.W, r.H),
	}
	cgContextSaveGState(p.ctx)
	cgContextClipToRects(p.ctx, &keep[0], uintptr(len(keep)))
	stroke()
	cgContextRestoreGState(p.ctx)
	p.clipped(layout.PointRect{X: gap0, Y: in.Origin.Y - pitch/2, W: gap1 - gap0, H: pitch}, func() {
		st := op.Box.TitleStyle
		p.line(op.Box.Title, f, gap0+titleGap, in.Origin.Y-pitch/2, pitch, p.roleColor(st), nil, st.Underline, st.Strikethrough)
	})
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
	return p.lineOn(text, f, x, y+(h-f.lineHeight())/2+f.ascent, y, h, fg, bg, underline, strike)
}

// lineOn draws one line of text from x on the baseline base, in the row of
// height h at y (which a background fills).
func (p *painter) lineOn(text string, f font, x, base, y, h float64, fg rgba, bg color.Color, underline, strike bool) float64 {
	l := shape(text, f)
	defer l.release()
	if bg != nil {
		p.fill(rgbaOf(bg))
		cgContextFillRect(p.ctx, cgRect(x, y, l.width, h))
	}
	p.fill(fg)
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

// text draws a Label's text — plain, or its runs — wrapped as MeasureText
// and MeasureRuns wrap it; each line's runs share the baseline of its
// tallest font. sel (from < to) is the selected bytes, highlighted behind.
func (p *painter) text(op layout.TextOp, from, to int) {
	rt := p.th.opText(op)
	for _, ln := range rt.layout(op) {
		if a, b := max(from, ln.start), min(to, ln.end); a < b || (from <= ln.end && to > ln.end && ln.end < len(rt.text)) {
			x0, x1 := rt.width(ln.start, a), rt.width(ln.start, b)
			if b < to { // the selected line break
				x1 += rt.fonts[0].width(" ")
			}
			p.fill(p.th.selection)
			cgContextFillRect(p.ctx, cgRect(op.Rect.X+x0, ln.y, x1-x0, ln.pitch))
		}
		base := ln.y + (ln.pitch-ln.height)/2 + ln.ascent
		x := op.Rect.X
		rt.pieces(ln.start, ln.end, func(i, a, b int) {
			st := rt.runs[i].Style
			x += p.lineOn(rt.text[a:b], rt.fonts[i], x, base, ln.y, ln.pitch, p.roleColor(st), st.Bg, st.Underline, st.Strikethrough)
		})
	}
}

// laidLine is one laid-out line of a TextOp: its bytes, top, and metrics.
type laidLine struct {
	span
	y float64
	lineMetrics
}

// opText is op's text in th's fonts: its runs, or its text as one run.
func (th theme) opText(op layout.TextOp) richText {
	if len(op.Runs) > 0 {
		return th.richText(op.Runs)
	}
	return th.richText([]layout.Run{{Text: op.Text, Style: op.Style}})
}

// layout is rt laid out as op draws it: wrapped at its width when it wraps,
// lines stacked from its top.
func (rt richText) layout(op layout.TextOp) []laidLine {
	wrap := 0.0
	if op.Wrap {
		wrap = op.Rect.W
	}
	spans := rt.lines(wrap)
	lines := make([]laidLine, len(spans))
	y := op.Rect.Y
	for i, sp := range spans {
		lines[i] = laidLine{span: sp, y: y, lineMetrics: rt.metrics(sp)}
		y += lines[i].pitch
	}
	return lines
}

// nearest is the byte offset in line sp nearest x points from its start.
func (rt richText) nearest(sp span, x float64) int {
	prev := 0.0
	for i := sp.start; i < sp.end; {
		_, n := utf8.DecodeRuneInString(rt.text[i:sp.end])
		next := rt.width(sp.start, i+n)
		if x < (prev+next)/2 {
			return i
		}
		prev, i = next, i+n
	}
	return sp.end
}

// input draws a text field: an outlined, filled rounded rect (outlined in
// the focus color when focused) with its text laid out by field — rows,
// scroll, selection, input-method text — and the caret when focused.
func (p *painter) input(op layout.InputOp) {
	r := op.Rect
	field := cgRect(r.X+0.5, r.Y+0.5, max(r.W-1, 0), max(r.H-1, 0))
	rad := min(p.th.radiusOf(layout.BorderNormal), field.Size.Height/2)
	p.fill(p.th.field)
	p.roundedPath(field, rad, func() { cgContextFillPath(p.ctx) })
	outline := p.th.border
	if op.Focused {
		outline = p.th.caret
	}
	p.stroke(outline)
	cgContextSetLineWidth(p.ctx, strokeWidth)
	p.roundedPath(field, rad, func() { cgContextStrokePath(p.ctx) })

	fl := p.th.layoutField(op, p.marked)
	if fl.text == "" {
		p.line(op.Placeholder, fl.f, fl.x0, fl.y0, fl.pitch, p.th.subtle, nil, false, false)
		if op.Focused {
			p.caretAt(fl.x0, fl.y0, fl.pitch, fl.f)
		}
		return
	}
	selStart, selEnd := fl.shift(min(op.SelStart, op.SelEnd)), fl.shift(max(op.SelStart, op.SelEnd))
	space := fl.f.width(" ")
	cr := caretRow(fl.rows, fl.at)
	for i := fl.first; i < len(fl.rows) && float64(i-fl.first)*fl.pitch < r.H; i++ {
		row, y := fl.rows[i], fl.y0+float64(i-fl.first)*fl.pitch
		s := fl.text[row.start:row.end]
		l := shape(s, fl.f)
		x := func(b int) float64 { return fl.x0 - fl.scroll + l.offset(s, b-row.start) }
		if a, b := max(selStart, row.start), min(selEnd, row.end); a < b || (row.lineEnd && selStart <= row.end && selEnd > row.end) {
			w := x(b) - x(a)
			if row.lineEnd && selEnd > row.end {
				w += space // the selected line break
			}
			p.fill(p.th.selection)
			cgContextFillRect(p.ctx, cgRect(x(a), y, w, fl.pitch))
		}
		p.line(s, fl.f, fl.x0-fl.scroll, y, fl.pitch, p.roleColor(op.Style), nil, op.Style.Underline, op.Style.Strikethrough)
		if a, b := max(fl.caret, row.start), min(fl.at, row.end); fl.mark > 0 && a < b {
			p.fill(p.th.marked)
			cgContextFillRect(p.ctx, cgRect(x(a), y+(fl.pitch+fl.f.lineHeight())/2-1.5, x(b)-x(a), 1.5))
		}
		if op.Focused && i == cr {
			p.caretAt(x(fl.at), y, fl.pitch, fl.f)
		}
		l.release()
	}
}

// fieldText is a field's text as the window lays it out — the one layout
// painting and caret hit-testing share. Rows are visual rows of text (one
// for a single-line field); rows from first show from (x0, y0), pitch
// apart, shifted left by scroll.
type fieldText struct {
	f           font
	pitch       float64
	text        string // the value, with any input-method text at the caret
	rows        []areaRow
	first       int
	scroll      float64
	x0, y0      float64
	caret, mark int // the caret in the value; the input-method text's length
	at          int // the caret in text: after the input-method text
	single      bool
}

// shift maps a byte offset into the value to one into text.
func (fl fieldText) shift(i int) int {
	if i > fl.caret {
		return i + fl.mark
	}
	return i
}

// x is the x of byte b of text in row i, from the row's start.
func (fl fieldText) x(i, b int) float64 {
	row := fl.rows[i]
	s := fl.text[row.start:row.end]
	l := shape(s, fl.f)
	defer l.release()
	return l.offset(s, b-row.start)
}

// index is the byte offset into text of the caret position in row i
// nearest x (from the row's start).
func (fl fieldText) index(i int, x float64) int {
	row := fl.rows[i]
	s := fl.text[row.start:row.end]
	l := shape(s, fl.f)
	defer l.release()
	return row.start + byteOfUTF16(s, ctLineGetStringIndexForPosition(l.ref, CGPoint{X: x}))
}

// byteOfUTF16 is the byte offset in s of UTF-16 index u, clamped into s.
func byteOfUTF16(s string, u int) int {
	n := 0
	for i, r := range s {
		if n >= u {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

// layoutField lays out op's text in th's type, with marked input-method
// text at a focused field's caret.
func (th theme) layoutField(op layout.InputOp, marked string) fieldText {
	r := op.Rect
	fl := fieldText{f: th.styleFont(op.Style), x0: r.X + fieldInset, y0: r.Y, single: !op.Multiline}
	avail := max(r.W-2*fieldInset, 1)
	caret := min(max(op.Caret, 0), len(op.Value))
	for caret > 0 && caret < len(op.Value) && !runeStart(op.Value, caret) {
		caret--
	}
	fl.text, fl.caret, fl.at = op.Value, caret, caret
	if op.Focused && marked != "" {
		fl.text, fl.mark = op.Value[:caret]+marked+op.Value[caret:], len(marked)
		fl.at = caret + fl.mark
	}
	if fl.single {
		fl.pitch = r.H
		fl.rows = []areaRow{{start: 0, end: len(fl.text), lineEnd: true}}
	} else {
		fl.pitch = fl.f.pitch()
		fl.rows = areaRows(fl.text, avail, op.Wrap, fl.f.width)
		fl.first = firstRow(op.OffsetY, caretRow(fl.rows, fl.at), int(r.H/fl.pitch), len(fl.rows))
	}
	// Unwrapped text scrolls sideways to keep a focused caret visible.
	if op.Focused && (fl.single || !op.Wrap) {
		if x := fl.x(caretRow(fl.rows, fl.at), fl.at); x > avail-2 {
			fl.scroll = x - avail + 2
		}
	}
	return fl
}

// windowText is the window's textLayout: carets in fields as the painter
// lays them out, in the window's current theme.
type windowText struct{ th func() theme }

func (w windowText) caretAt(op layout.InputOp, x, y float64) int {
	fl := w.th().layoutField(op, "")
	i := 0
	if !fl.single {
		i = min(max(fl.first+int(math.Floor((y-fl.y0)/fl.pitch)), 0), len(fl.rows)-1)
	}
	return fl.index(i, x-fl.x0+fl.scroll)
}

func (w windowText) textOffset(op layout.TextOp, x, y float64) int {
	rt := w.th().opText(op)
	lines := rt.layout(op)
	for i, ln := range lines {
		if y < ln.y+ln.pitch || i == len(lines)-1 {
			return rt.nearest(ln.span, x-op.Rect.X)
		}
	}
	return 0
}

func (w windowText) caretX(op layout.InputOp, caret int) float64 {
	fl := w.th().layoutField(op, "")
	return fl.x(caretRow(fl.rows, caret), caret)
}

func (w windowText) caretStep(op layout.InputOp, caret, dir int, x float64) int {
	fl := w.th().layoutField(op, "")
	from := caretRow(fl.rows, caret)
	to := min(max(from+dir, 0), len(fl.rows)-1)
	if to == from {
		return caret
	}
	return fl.index(to, x)
}

// runeStart reports whether byte i of s starts a rune (or is the end).
func runeStart(s string, i int) bool { return i == len(s) || s[i]&0xC0 != 0x80 }

// caretAt draws the caret at x in the row of height h at y, as tall as f's
// text and centered like it.
func (p *painter) caretAt(x, y, h float64, f font) {
	p.caret = cgRect(x, y+(h-f.lineHeight())/2, 1.5, f.lineHeight())
	p.caretOK, p.caretClip = true, p.clip
}

// paintCaret draws the caret placed while drawing, clipped to its field. It
// is separate so a window can blink it.
func (p *painter) paintCaret() {
	if !p.caretOK {
		return
	}
	p.clipped(p.caretClip, func() {
		p.fill(p.th.caret)
		cgContextFillRect(p.ctx, p.caret)
	})
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
	f := p.th.font(fontKey{mono: true, bold: st.Attrs&uv.AttrBold != 0, italic: st.Attrs&uv.AttrItalic != 0})
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
	p.shape(g, fg)
}

// meterThickness is a Meter bar's height in points.
const meterThickness = 6.0

// meter draws a Meter: a thin rounded track across the rect, centered on the
// line, with the filled part over it.
func (p *painter) meter(op layout.MeterOp) {
	r := op.Rect
	h := min(meterThickness, r.H)
	bar := cgRect(r.X, r.Y+(r.H-h)/2, r.W, h)
	track, fill := p.th.track, p.th.caret
	if op.Track != nil {
		track = rgbaOf(op.Track)
	}
	if op.Fill != nil {
		fill = rgbaOf(op.Fill)
	}
	p.fill(track)
	p.roundedPath(bar, h/2, func() { cgContextFillPath(p.ctx) })
	if w := r.W * min(max(op.Value, 0), 1); w > 0 {
		filled := cgRect(bar.Origin.X, bar.Origin.Y, w, h)
		p.fill(fill)
		p.roundedPath(filled, min(h/2, w/2), func() { cgContextFillPath(p.ctx) })
	}
}

// icon draws an Icon's shape in its style's color, or its text when the
// name has no shape.
// Scroll bar thumb sizes in points: its width, and its shortest length.
const (
	thumbWidth = 5.0
	thumbMin   = 16.0
)

// scrollBar draws a thin rounded thumb centered in the bar's column, in
// the bar's color or a translucent subtle grey, and nothing when the
// content fits.
func (p *painter) scrollBar(op layout.ScrollBarOp) {
	if !op.Show {
		return
	}
	r := op.Rect
	w := min(thumbWidth, r.W)
	h := min(max(op.Length*r.H, thumbMin), r.H)
	// The thumb moves over the track less its length, as the content
	// moves under the window.
	y := r.Y
	if op.Length < 1 {
		y += op.Start / (1 - op.Length) * (r.H - h)
	}
	c := p.th.subtle
	c.A *= 0.6
	if op.Color != nil {
		c = rgbaOf(op.Color)
	}
	p.fill(c)
	p.roundedPath(cgRect(r.X+(r.W-w)/2, y, w, h), w/2, func() { cgContextFillPath(p.ctx) })
}

func (p *painter) icon(op layout.IconOp) {
	if g, ok := iconShape(op.Name, op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H, p.m.Column); ok {
		p.shape(g, p.roleColor(op.Style))
		return
	}
	f := p.th.styleFont(op.Style)
	p.line(op.Text, f, op.Rect.X, op.Rect.Y, f.pitch(), p.roleColor(op.Style), op.Style.Bg, op.Style.Underline, op.Style.Strikethrough)
}

// shape fills and strokes g in fg.
func (p *painter) shape(g cellShape, fg rgba) {
	for _, r := range g.rects {
		c := fg
		c.A *= r.Alpha
		p.cellFill(c, r.X, r.Y, r.W, r.H)
	}
	p.fill(fg)
	p.stroke(fg)
	cgContextSetLineWidth(p.ctx, strokeWidth)
	for _, c := range g.circles {
		cgContextAddEllipseInRect(p.ctx, cgRect(c.c.X-c.r, c.c.Y-c.r, 2*c.r, 2*c.r))
		if c.fill {
			cgContextFillPath(p.ctx)
		} else {
			cgContextSetLineWidth(p.ctx, max(c.width, strokeWidth))
			cgContextStrokePath(p.ctx)
			cgContextSetLineWidth(p.ctx, strokeWidth)
		}
	}
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
			cgContextSetLineWidth(p.ctx, max(pl.width, strokeWidth))
			if pl.round {
				cgContextSetLineCap(p.ctx, cgLineCapRound)
				cgContextSetLineJoin(p.ctx, cgLineJoinRound)
			}
			cgContextStrokePath(p.ctx)
			cgContextSetLineWidth(p.ctx, strokeWidth)
			cgContextSetLineCap(p.ctx, cgLineCapButt)
			cgContextSetLineJoin(p.ctx, cgLineJoinMiter)
		}
	}
}
