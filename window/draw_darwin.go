package window

import (
	"image/color"
	"strings"
	"sync"
	"unicode/utf16"

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
	bg, text, subtle, title, border, field, caret, marked rgba
}

var darkTheme = theme{
	bg:     rgba{0.118, 0.122, 0.133, 1},
	text:   rgba{0.871, 0.878, 0.898, 1},
	subtle: rgba{0.553, 0.569, 0.608, 1},
	title:  rgba{0.949, 0.953, 0.961, 1},
	border: rgba{0.298, 0.306, 0.333, 1},
	field:  rgba{0.169, 0.176, 0.196, 1},
	caret:  rgba{0.400, 0.655, 1.000, 1},
	marked: rgba{0.400, 0.655, 1.000, 1},
}

// Type sizes in points. Body text sets the line metric; the cell grid uses
// the user's fixed-pitch font at the same size.
const (
	bodySize   = 13.0
	titleSize  = 15.0
	stepPoints = 8.0
	radius     = 6.0
	fieldInset = 4.0
)

type fontKey struct {
	role               layout.TextRole
	bold, italic, mono bool
}

type font struct {
	ref                      uintptr
	ascent, descent, leading float64
}

func (f font) lineHeight() float64 { return f.ascent + f.descent + f.leading }

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
	kind, size := uint32(ctFontSystem), bodySize
	switch {
	case k.mono:
		kind = ctFontUserFixedPitch
	case k.role == layout.TextTitle:
		kind, size = ctFontEmphasizedSystem, titleSize
	}
	ref := ctFontCreateUIFontForLanguage(kind, size, 0)
	var traits uint32
	if k.bold {
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
// fixed-pitch advance, lines the body line height, a step 8pt, a border 1pt.
func metrics() layout.Metrics {
	mono := fontFor(fontKey{mono: true})
	zero := shape("0", mono)
	defer zero.release()
	return layout.Metrics{
		Column: zero.width,
		Line:   fontFor(fontKey{}).lineHeight(),
		Step:   stepPoints,
		Border: 1,
		MeasureText: func(text string, style layout.TextStyle) (float64, float64) {
			f := styleFont(style)
			lines := strings.Split(text, "\n")
			w := 0.0
			for _, s := range lines {
				l := shape(s, f)
				w = max(w, l.width)
				l.release()
			}
			return w, float64(len(lines)) * f.lineHeight()
		},
	}
}

// painter draws a scene into a CoreGraphics context whose user space is
// top-left origin points (a flipped view, or a flipped bitmap).
type painter struct {
	ctx uintptr
	m   layout.Metrics
	th  theme
	// marked is the input method's uncommitted text, shown at the focused
	// input's caret.
	marked string
	// caret is where the focused input's caret was drawn (for the input
	// method's candidate window); ok reports whether one was.
	caret   CGRect
	caretOK bool
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
		switch op := op.(type) {
		case layout.LayerOp:
			p.fill(p.th.bg)
			cgContextFillRect(p.ctx, rectOf(op.Rect))
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

func (p *painter) box(op layout.BoxOp) {
	rad := 0.0
	if op.Box.Border == layout.BorderRounded || (op.Box.Border != layout.BorderNone && op.Box.Border != layout.BorderNormal) {
		rad = radius
	}
	r := rectOf(op.Rect)
	if op.Box.Background != nil && op.Box.Background != color.Transparent {
		p.fill(rgbaOf(op.Box.Background))
		p.roundedPath(r, min(rad, r.Size.Width/2, r.Size.Height/2), func() { cgContextFillPath(p.ctx) })
	}
	if op.Box.Border == layout.BorderNone {
		return
	}
	c := p.th.border
	if op.Box.BorderColor != nil {
		c = rgbaOf(op.Box.BorderColor)
	}
	w := p.m.Border
	in := cgRect(op.Rect.X+w/2, op.Rect.Y+w/2, max(op.Rect.W-w, 0), max(op.Rect.H-w, 0))
	p.stroke(c)
	cgContextSetLineWidth(p.ctx, w)
	p.roundedPath(in, min(rad, in.Size.Width/2, in.Size.Height/2), func() { cgContextStrokePath(p.ctx) })
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

// line draws one line of text with its top-left at (x, y).
func (p *painter) line(text string, f font, x, y float64, fg rgba, bg color.Color, underline, strike bool) float64 {
	l := shape(text, f)
	defer l.release()
	if bg != nil {
		p.fill(rgbaOf(bg))
		cgContextFillRect(p.ctx, cgRect(x, y, l.width, f.lineHeight()))
	}
	p.fill(fg)
	base := y + f.ascent
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
	for i, s := range strings.Split(op.Text, "\n") {
		p.line(s, f, op.Rect.X, op.Rect.Y+float64(i)*f.lineHeight(), p.roleColor(op.Style), op.Style.Bg, op.Style.Underline, op.Style.Strikethrough)
	}
}

// input draws a text field: a filled rounded rect with the value (or the
// placeholder), scrolled so the caret stays visible, and the caret and any
// input-method text when focused.
func (p *painter) input(op layout.InputOp) {
	r := op.Rect
	p.fill(p.th.field)
	p.roundedPath(rectOf(r), min(4, r.H/2), func() { cgContextFillPath(p.ctx) })
	f := styleFont(op.Style)
	y := r.Y + (r.H-f.lineHeight())/2
	x0 := r.X + fieldInset
	avail := max(r.W-2*fieldInset, 1)
	if op.Value == "" && (!op.Focused || p.marked == "") {
		p.line(op.Placeholder, f, x0, y, p.th.subtle, nil, false, false)
		if op.Focused {
			p.caretAt(x0, y, f)
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
	p.line(text, f, x0-scroll, y, p.roleColor(op.Style), nil, op.Style.Underline, op.Style.Strikethrough)
	if markEnd > markStart {
		ml := shape(text, f)
		a, b := ml.offset(text, markStart), ml.offset(text, markEnd)
		ml.release()
		p.fill(p.th.marked)
		cgContextFillRect(p.ctx, cgRect(x0-scroll+a, y+f.lineHeight()-1.5, b-a, 1.5))
	}
	if op.Focused {
		p.caretAt(x0-scroll+caretX, y, f)
	}
}

// runeStart reports whether byte i of s starts a rune (or is the end).
func runeStart(s string, i int) bool { return i == len(s) || s[i]&0xC0 != 0x80 }

func (p *painter) caretAt(x, y float64, f font) {
	p.caret = cgRect(x, y, 1.5, f.lineHeight())
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
			start, style := x, c.Style
			var run strings.Builder
			for x < op.Cols {
				cc := buf.CellAt(x, y)
				if cc == nil || !cc.Style.Equal(&style) || (cc.Width > 1 && x > start) {
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

func (p *painter) cellRun(text string, st uv.Style, x, y, w float64) {
	fg, bg := p.th.text, rgba{}
	hasBg := false
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
	if hasBg {
		p.fill(bg)
		cgContextFillRect(p.ctx, cgRect(x, y, w, p.m.Line))
	}
	if strings.TrimSpace(text) == "" || st.Attrs&uv.AttrConceal != 0 {
		return
	}
	f := fontFor(fontKey{mono: true, bold: st.Attrs&uv.AttrBold != 0, italic: st.Attrs&uv.AttrItalic != 0})
	p.line(text, f, x, y+(p.m.Line-f.lineHeight())/2, fg, nil, st.Underline != uv.UnderlineNone, st.Attrs&uv.AttrStrikethrough != 0)
}
