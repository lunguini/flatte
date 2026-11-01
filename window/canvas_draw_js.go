//go:build js && wasm

package window

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

const (
	canvasMeterThickness = 6.0
	canvasThumbWidth     = 5.0
	canvasThumbMin       = 16.0
	canvasFocusRing      = 2.0
)

type canvasImage struct {
	bitmap          js.Value
	used            uint64
	loading, failed bool
}

type canvasPainter struct {
	ctx        js.Value
	theme      Theme
	text       canvasTextLayout
	metrics    layout.Metrics
	pres       presentation
	marked     string
	caretOn    bool
	scale      float64
	now        time.Time
	animated   bool
	layer      layout.PointRect
	inLayer    bool
	gen        uint64
	images     map[image.Image]*canvasImage
	invalidate func()
}

func newCanvasPainter(ctx js.Value, th Theme) *canvasPainter {
	th = th.over(Dark())
	txt := canvasTextLayout{ctx: ctx, theme: th}
	p := &canvasPainter{ctx: ctx, theme: th, text: txt, metrics: txt.metrics(), scale: 1, images: map[image.Image]*canvasImage{}}
	return p
}

func (p *canvasPainter) draw(scene layout.Scene, width, height float64, pres presentation) {
	p.pres, p.now, p.animated = pres, time.Now(), false
	p.gen++
	p.ctx.Set("globalAlpha", 1)
	p.ctx.Set("filter", "none")
	p.ctx.Call("clearRect", 0, 0, width, height)
	p.fillRect(layout.PointRect{W: width, H: height}, p.theme.Background)
	for i, item := range scene.Ops {
		switch op := item.(type) {
		case layout.LayerOp:
			raised := false
			if i+1 < len(scene.Ops) {
				if box, ok := scene.Ops[i+1].(layout.BoxOp); ok {
					raised = box.Rect == op.Rect
				}
			}
			if !raised {
				p.fillRect(op.Rect, p.theme.Background)
			}
			p.layer, p.inLayer = op.Rect, true
		case layout.BoxOp:
			p.withClip(op.Clip, func() { p.box(op) })
		case layout.TextOp:
			from, to := p.pres.sel.in(i, op)
			p.withClip(intersect(op.Rect, op.Clip), func() { p.textOp(op, from, to) })
		case layout.InputOp:
			p.withClip(op.Clip, func() { p.input(op) })
		case layout.IconOp:
			p.withClip(intersect(op.Rect, op.Clip), func() { p.icon(op) })
		case layout.ImageOp:
			p.withClip(intersect(op.Rect, op.Clip), func() { p.image(op) })
		case layout.MeterOp:
			p.withClip(intersect(op.Rect, op.Clip), func() { p.meter(op) })
		case layout.ControlOp:
			p.withClip(op.Clip, func() { p.control(op) })
		case layout.ScrollBarOp:
			p.withClip(intersect(op.Rect, op.Clip), func() { p.scrollBar(op) })
		case layout.CellsOp:
			p.withClip(intersect(op.Rect, op.Clip), func() { p.cells(op) })
		}
		if _, ok := item.(layout.LayerOp); !ok {
			if _, isBox := item.(layout.BoxOp); !isBox {
				p.inLayer = false
			}
		}
	}
	p.pruneImages()
}

func (p *canvasPainter) drawCursor(cursor flatte.Cursor) {
	x, y := float64(cursor.X)*p.metrics.Column, float64(cursor.Y)*p.metrics.Line
	w, h := p.metrics.Column, p.metrics.Line
	shape, c := flatte.CursorShapeDefault, p.theme.Text
	if cursor.Style != nil {
		shape = cursor.Style.Shape
		if cursor.Style.Color != nil {
			c = cursor.Style.Color
		}
	}
	switch shape {
	case flatte.CursorShapeBar:
		p.fillRect(layout.PointRect{X: x, Y: y, W: 2, H: h}, c)
	case flatte.CursorShapeUnderline:
		p.fillRect(layout.PointRect{X: x, Y: y + h - 2, W: w, H: 2}, c)
	default:
		p.ctx.Set("globalAlpha", 0.45)
		p.fillRect(layout.PointRect{X: x, Y: y, W: w, H: h}, c)
		p.ctx.Set("globalAlpha", 1)
	}
}

func (p *canvasPainter) close() {
	for source, entry := range p.images {
		if !entry.bitmap.IsUndefined() && !entry.bitmap.IsNull() && entry.bitmap.Get("close").Type() == js.TypeFunction {
			entry.bitmap.Call("close")
		}
		delete(p.images, source)
	}
}

func (p *canvasPainter) fill(c color.Color)   { p.ctx.Set("fillStyle", canvasColor(c)) }
func (p *canvasPainter) stroke(c color.Color) { p.ctx.Set("strokeStyle", canvasColor(c)) }

func canvasColor(c color.Color) string {
	if c == nil {
		return "rgba(0, 0, 0, 0)"
	}
	r := rgbaOf(c)
	return fmt.Sprintf("rgba(%d, %d, %d, %.4f)", int(r.R*255+0.5), int(r.G*255+0.5), int(r.B*255+0.5), r.A)
}

func (p *canvasPainter) roleColor(style layout.TextStyle) rgba {
	return roleRGBA(style, rgbaOf(p.theme.Text), rgbaOf(p.theme.Subtle), rgbaOf(p.theme.Title))
}

func (p *canvasPainter) fillRect(r layout.PointRect, c color.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	p.fill(c)
	p.ctx.Call("fillRect", r.X, r.Y, r.W, r.H)
}

func (p *canvasPainter) withClip(r layout.PointRect, fn func()) {
	p.ctx.Call("save")
	p.ctx.Call("beginPath")
	p.ctx.Call("rect", r.X, r.Y, max(r.W, 0), max(r.H, 0))
	p.ctx.Call("clip")
	fn()
	p.ctx.Call("restore")
}

func (p *canvasPainter) roundedPath(r layout.PointRect, radius float64) {
	rad := min(max(radius, 0), min(r.W, r.H)/2)
	p.ctx.Call("beginPath")
	p.ctx.Call("moveTo", r.X+rad, r.Y)
	p.ctx.Call("lineTo", r.X+r.W-rad, r.Y)
	p.ctx.Call("arcTo", r.X+r.W, r.Y, r.X+r.W, r.Y+rad, rad)
	p.ctx.Call("lineTo", r.X+r.W, r.Y+r.H-rad)
	p.ctx.Call("arcTo", r.X+r.W, r.Y+r.H, r.X+r.W-rad, r.Y+r.H, rad)
	p.ctx.Call("lineTo", r.X+rad, r.Y+r.H)
	p.ctx.Call("arcTo", r.X, r.Y+r.H, r.X, r.Y+r.H-rad, rad)
	p.ctx.Call("lineTo", r.X, r.Y+rad)
	p.ctx.Call("arcTo", r.X, r.Y, r.X+rad, r.Y, rad)
	p.ctx.Call("closePath")
}

func (p *canvasPainter) fillRounded(r layout.PointRect, radius float64, c color.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	p.roundedPath(r, radius)
	p.fill(c)
	p.ctx.Call("fill")
}

func (p *canvasPainter) strokeRounded(r layout.PointRect, radius, width float64, c color.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	p.roundedPath(r, radius)
	p.stroke(c)
	p.ctx.Set("lineWidth", width)
	p.ctx.Call("stroke")
}

func canvasRadius(th Theme, kind layout.BorderKind) float64 {
	if th.Radius < 0 {
		return 0
	}
	if kind == layout.BorderNormal {
		return th.Radius
	}
	return 2 * th.Radius
}

func canvasBoxRadius(r layout.PointRect, picture bool) float64 {
	if picture {
		return min(min(r.W, r.H)*pictureRound, 10)
	}
	return min(r.H/2, r.W/2, 10)
}

func canvasSelectionRadius(r layout.PointRect, th Theme) float64 {
	return min(canvasRadius(th, layout.BorderNormal), r.W/2, r.H/2)
}

func canvasBoxFillRadius(box layout.Box, r layout.PointRect, th Theme) float64 {
	if box.Backdrop != nil {
		return canvasBoxRadius(r, true)
	}
	if box.Interactive {
		return canvasSelectionRadius(r, th)
	}
	return canvasBoxRadius(r, false)
}

func (p *canvasPainter) box(op layout.BoxOp) {
	raised := p.inLayer && op.Rect == p.layer
	p.inLayer = false
	if bd := op.Box.Backdrop; bd != nil && bd.Source != nil {
		p.ctx.Call("save")
		if bd.Opacity > 0 && bd.Opacity < 1 {
			p.ctx.Set("globalAlpha", bd.Opacity)
		}
		if bd.Blur > 0 {
			p.ctx.Set("filter", fmt.Sprintf("blur(%.2fpx)", bd.Blur))
		}
		p.image(layout.ImageOp{Rect: op.Rect, Clip: op.Clip, Source: bd.Source, Fit: layout.ImageCover, Round: op.Box.Round})
		p.ctx.Call("restore")
	}
	if op.Box.Selected {
		p.fillRounded(op.Rect, canvasSelectionRadius(op.Rect, p.theme), p.theme.Selection)
	}
	bg, hasBG := op.Box.Background, op.Box.Background != nil && op.Box.Background != color.Transparent
	if !hasBG && raised {
		bg, hasBG = p.theme.Surface, true
	}
	if op.Box.Border == layout.BorderNone {
		if hasBG {
			if op.Box.Round {
				p.fillRounded(op.Rect, canvasBoxFillRadius(op.Box, op.Rect, p.theme), bg)
			} else {
				p.fillRect(op.Rect, bg)
			}
		}
	} else {
		in := layout.PointRect{X: op.Rect.X + borderBand/2, Y: op.Rect.Y + borderBand/2, W: max(op.Rect.W-borderBand, 0), H: max(op.Rect.H-borderBand, 0)}
		rad := min(canvasRadius(p.theme, op.Box.Border), in.W/2, in.H/2)
		if hasBG {
			p.fillRounded(in, rad, bg)
		}
		border := p.theme.Border
		if op.Box.BorderColor != nil {
			border = op.Box.BorderColor
		}
		p.strokeRounded(in, rad, strokeWidth, border)
		if op.Box.Title != "" {
			style := op.Box.TitleStyle
			font := p.text.font(style)
			x := op.Rect.X + max(rad, 8) + 4
			w, _ := p.text.measure(style, op.Box.Title)
			background := p.theme.Background
			if hasBG {
				background = bg
			}
			p.fillRect(layout.PointRect{X: x - 3, Y: op.Rect.Y + 1, W: w + 6, H: font.pitch}, background)
			p.textLine(op.Box.Title, style, x, in.Y-font.pitch/2, font)
		}
	}
	p.hoverFill(op)
}

func (p *canvasPainter) hoverFill(op layout.BoxOp) {
	if op.ID == "" || op.Box.Hover == nil && !op.Box.Interactive {
		return
	}
	level := p.level(p.pres.hovers, op.ID, op.ID == p.pres.hovered, false) + 0.5*p.level(p.pres.presses, op.ID, op.ID == p.pres.pressed, false)
	if level <= 0 {
		return
	}
	c := rgbaOf(p.theme.Text)
	c.A = 0.08
	if op.Box.Hover != nil {
		c = rgbaOf(op.Box.Hover)
	}
	c.A *= min(level, 1.5)
	p.fillRounded(op.Rect, canvasBoxFillRadius(op.Box, op.Rect, p.theme), colorFromRGBA(c))
}

func (p *canvasPainter) level(values map[string]float64, id string, on, disabled bool) float64 {
	if disabled || id == "" {
		return 0
	}
	if values != nil {
		return values[id]
	}
	if on {
		return 1
	}
	return 0
}

func colorFromRGBA(c rgba) color.Color {
	return color.NRGBA{R: uint8(min(max(c.R, 0), 1)*255 + 0.5), G: uint8(min(max(c.G, 0), 1)*255 + 0.5), B: uint8(min(max(c.B, 0), 1)*255 + 0.5), A: uint8(min(max(c.A, 0), 1)*255 + 0.5)}
}

func (p *canvasPainter) textOp(op layout.TextOp, from, to int) {
	runs := op.Runs
	if len(runs) == 0 {
		runs = []layout.Run{{Text: op.Text, Style: op.Style}}
	}
	rt := p.text.richText(runs, op.Style)
	for _, line := range rt.layout(op) {
		selectedFrom, selectedTo := max(from, line.start), min(to, line.end)
		if selectedFrom < selectedTo {
			x := rt.width(line.start, selectedFrom)
			w := rt.width(selectedFrom, selectedTo)
			p.fillRect(layout.PointRect{X: op.Rect.X + x, Y: line.y, W: w, H: line.pitch}, p.theme.Selection)
		}
		base := line.y + (line.pitch-line.height)/2 + line.ascent
		end := line.end
		ellipsis, ellipsisRun := false, 0
		if op.Truncate && rt.width(line.start, line.end) > op.Rect.W {
			end, ellipsisRun = rt.truncated(line.span, op.Rect.W)
			ellipsis = true
		}
		x := op.Rect.X
		rt.pieces(line.start, end, func(i, a, b int) {
			style := rt.runs[i].Style
			if zeroCanvasStyle(style) {
				style = op.Style
			}
			fg := p.roleColor(style)
			if op.HoverFg != nil && op.ID != "" {
				if level := p.level(p.pres.hovers, op.ID, op.ID == p.pres.hovered, false); level > 0 {
					fg = mix(fg, rgbaOf(op.HoverFg), level)
				}
			}
			x += p.textRun(rt.text[a:b], style, rt.fonts[i], x, base, line.y, line.pitch, fg)
		})
		if ellipsis {
			style := rt.runs[ellipsisRun].Style
			if zeroCanvasStyle(style) {
				style = op.Style
			}
			p.textRun("…", style, rt.fonts[ellipsisRun], x, base, line.y, line.pitch, p.roleColor(style))
		}
	}
}

func (p *canvasPainter) textRun(text string, style layout.TextStyle, font canvasFont, x, base, y, h float64, fg rgba) float64 {
	w := p.text.width(font, text)
	if style.Bg != nil {
		p.fillRect(layout.PointRect{X: x, Y: y, W: w, H: h}, style.Bg)
	}
	if style.Fg != nil {
		fg = rgbaOf(style.Fg)
	}
	p.ctx.Set("font", font.css)
	p.ctx.Set("textBaseline", "alphabetic")
	p.fill(colorFromRGBA(fg))
	p.ctx.Call("fillText", text, x, base)
	if style.Underline || style.Strikethrough {
		lineY := base + max(font.descent*0.35, 1)
		if style.Strikethrough {
			lineY = base - font.ascent*0.35
		}
		p.fill(colorFromRGBA(fg))
		p.ctx.Call("fillRect", x, lineY, w, 1)
	}
	return w
}

func (p *canvasPainter) textLine(text string, style layout.TextStyle, x, y float64, font canvasFont) {
	base := y + (font.pitch-(font.ascent+font.descent))/2 + font.ascent
	p.textRun(text, style, font, x, base, y, font.pitch, p.roleColor(style))
}

func (p *canvasPainter) input(op layout.InputOp) {
	field := op.Field
	if field == (layout.PointRect{}) {
		field = op.Rect
	}
	if !op.Bare {
		p.fillRounded(field, min(canvasRadius(p.theme, layout.BorderNormal)+1, field.H/2), p.theme.Field)
		border := p.theme.Border
		if op.Focused {
			border = p.theme.Accent
		}
		p.strokeRounded(field, min(canvasRadius(p.theme, layout.BorderNormal)+1, field.H/2), 1, border)
	}
	p.withClip(intersect(op.Rect, op.Clip), func() { p.inputText(op) })
}

func (p *canvasPainter) inputText(op layout.InputOp) {
	fl := p.text.layoutField(op, p.marked)
	rowEnd := len(fl.rows)
	first := 0
	if !fl.single {
		first, rowEnd = fl.first, min(fl.first+int(op.Rect.H/max(fl.pitch, 1)), len(fl.rows))
	}
	if op.Value == "" && op.Placeholder != "" {
		font := p.text.font(layout.TextStyle{Role: layout.TextSubtle})
		y := op.Rect.Y + (op.Rect.H-font.pitch)/2 + font.ascent
		p.ctx.Set("font", font.css)
		p.fill(p.theme.Subtle)
		p.ctx.Call("fillText", op.Placeholder, fl.x0, y)
		if op.Focused && p.caretOn {
			p.fill(p.theme.Accent)
			p.ctx.Call("fillRect", fl.x0, y-font.ascent, 1.5, font.ascent+font.descent)
		}
		return
	}
	for i := first; i < rowEnd; i++ {
		row := fl.rows[i]
		y := fl.y0 + float64(i-first)*fl.pitch
		base := y + (fl.pitch-(fl.font.ascent+fl.font.descent))/2 + fl.font.ascent
		left := fl.x0 - fl.scroll
		start, end := row.start, row.end
		if op.Multiline {
			from, to := fl.shift(min(op.SelStart, op.SelEnd)), fl.shift(max(op.SelStart, op.SelEnd))
			if from < end && to > start {
				a, b := max(from, start), min(to, end)
				p.fillRect(layout.PointRect{X: left + p.text.width(fl.font, fl.text[start:a]), Y: y, W: p.text.width(fl.font, fl.text[a:b]), H: fl.pitch}, p.theme.Selection)
			}
		}
		p.ctx.Set("font", fl.font.css)
		p.fill(colorFromRGBA(p.roleColor(op.Style)))
		if fl.mark > 0 && fl.caret >= start && fl.caret <= end {
			pre := fl.text[start:fl.caret]
			markEnd := min(fl.caret+fl.mark, end)
			post := fl.text[markEnd:end]
			x := left + p.text.width(fl.font, pre)
			p.ctx.Call("fillText", pre, left, base)
			p.ctx.Call("fillText", fl.text[fl.caret:markEnd], x, base)
			mw := p.text.width(fl.font, fl.text[fl.caret:markEnd])
			p.ctx.Call("fillRect", x, base+1, mw, 1)
			p.ctx.Call("fillText", post, x+mw, base)
		} else {
			p.ctx.Call("fillText", fl.text[start:end], left, base)
		}
		if op.Focused && p.caretOn && i == caretRow(fl.rows, fl.at) {
			cx := left + fl.x(i, fl.at)
			p.fill(p.theme.Accent)
			p.ctx.Call("fillRect", cx, base-fl.font.ascent, 1.5, fl.font.ascent+fl.font.descent)
		}
	}
	if op.Clearable {
		if clear, ok := inputClearRect(op); ok {
			font := p.text.font(layout.TextStyle{Role: layout.TextBody, Size: 15})
			p.ctx.Set("font", font.css)
			p.fill(p.theme.Subtle)
			p.ctx.Set("textAlign", "center")
			p.ctx.Set("textBaseline", "middle")
			p.ctx.Call("fillText", "×", clear.X+clear.W/2, clear.Y+clear.H/2)
			p.ctx.Set("textAlign", "start")
		}
	}
}

func (p *canvasPainter) icon(op layout.IconOp) {
	if op.Animate && op.Name == layout.IconEqualizer {
		p.animated = true
		s := min(0.7*op.Rect.H, op.Rect.W)
		p.shape(equalizer(op.Rect.X+op.Rect.W/2, op.Rect.Y+op.Rect.H/2, s, p.now), p.roleColor(op.Style))
		return
	}
	if g, ok := iconShape(op.Name, op.Rect.X, op.Rect.Y, op.Rect.W, op.Rect.H, p.metrics.Column); ok {
		p.shape(g, p.roleColor(op.Style))
		return
	}
	font := p.text.font(op.Style)
	y := op.Rect.Y + (op.Rect.H-font.pitch)/2 + font.ascent
	p.ctx.Set("font", font.css)
	p.fill(colorFromRGBA(p.roleColor(op.Style)))
	p.ctx.Call("fillText", op.Text, op.Rect.X, y)
}

func (p *canvasPainter) shape(g cellShape, fg rgba) {
	for _, r := range g.rects {
		c := fg
		c.A *= r.Alpha
		p.fill(colorFromRGBA(c))
		p.ctx.Call("fillRect", r.X, r.Y, r.W, r.H)
	}
	p.stroke(colorFromRGBA(fg))
	p.fill(colorFromRGBA(fg))
	p.ctx.Set("lineWidth", strokeWidth)
	p.ctx.Set("lineCap", "round")
	p.ctx.Set("lineJoin", "round")
	for _, c := range g.circles {
		p.ctx.Call("beginPath")
		p.ctx.Call("arc", c.c.X, c.c.Y, c.r, 0, 2*math.Pi)
		if c.fill {
			p.ctx.Call("fill")
		} else {
			p.ctx.Set("lineWidth", max(c.width, strokeWidth))
			p.ctx.Call("stroke")
			p.ctx.Set("lineWidth", strokeWidth)
		}
	}
	for _, a := range g.arcs {
		p.ctx.Call("beginPath")
		p.ctx.Call("moveTo", a.from.X, a.from.Y)
		p.ctx.Call("arcTo", a.corner.X, a.corner.Y, a.to.X, a.to.Y, a.r)
		p.ctx.Call("lineTo", a.to.X, a.to.Y)
		p.ctx.Call("stroke")
	}
	for _, pl := range g.polys {
		if len(pl.pts) == 0 {
			continue
		}
		p.ctx.Call("beginPath")
		p.ctx.Call("moveTo", pl.pts[0].X, pl.pts[0].Y)
		for _, pt := range pl.pts[1:] {
			p.ctx.Call("lineTo", pt.X, pt.Y)
		}
		if pl.fill {
			p.ctx.Call("closePath")
			p.ctx.Call("fill")
		} else {
			p.ctx.Set("lineWidth", max(pl.width, strokeWidth))
			p.ctx.Call("stroke")
			p.ctx.Set("lineWidth", strokeWidth)
		}
	}
}

func (p *canvasPainter) meter(op layout.MeterOp) {
	r := op.Rect
	h := min(canvasMeterThickness, r.H)
	bar := layout.PointRect{X: r.X, Y: r.Y + (r.H-h)/2, W: r.W, H: h}
	track, fill := p.theme.Track, p.theme.Accent
	if op.Track != nil {
		track = op.Track
	}
	if op.Fill != nil {
		fill = op.Fill
	}
	p.fillRounded(bar, h/2, track)
	if w := r.W * min(max(op.Value, 0), 1); w > 0 {
		p.fillRounded(layout.PointRect{X: bar.X, Y: bar.Y, W: w, H: h}, min(h/2, w/2), fill)
	}
}

func (p *canvasPainter) scrollBar(op layout.ScrollBarOp) {
	if !op.Show {
		return
	}
	r := op.Rect
	w, h := min(canvasThumbWidth, r.W), min(max(op.Length*r.H, canvasThumbMin), r.H)
	y := r.Y
	if op.Length < 1 {
		y += op.Start / (1 - op.Length) * (r.H - h)
	}
	c := rgbaOf(p.theme.Subtle)
	c.A *= 0.6
	color := colorFromRGBA(c)
	if op.Color != nil {
		color = op.Color
	}
	p.fillRounded(layout.PointRect{X: r.X + (r.W-w)/2, Y: y, W: w, H: h}, w/2, color)
}

func (p *canvasPainter) control(op layout.ControlOp) {
	hot := op.ID != "" && op.ID == p.pres.hovered && !op.Disabled
	down := hot && op.ID == p.pres.pressed
	switch op.Kind {
	case layout.ControlButton:
		p.button(op, p.level(p.pres.hovers, op.ID, hot, op.Disabled), p.level(p.pres.presses, op.ID, down, op.Disabled))
	case layout.ControlSlider:
		p.slider(op, hot, down)
	default:
		p.toggle(op, hot, down)
	}
}

func (p *canvasPainter) dim(c rgba, disabled bool) rgba {
	if disabled {
		return mix(c, rgbaOf(p.theme.Background), 0.55)
	}
	return c
}

func (p *canvasPainter) button(op layout.ControlOp, hot, down float64) {
	r := op.Rect
	bez := layout.PointRect{X: r.X + 0.5, Y: r.Y + 1.5, W: max(r.W-1, 0), H: max(r.H-3, 0)}
	radius := min(canvasRadius(p.theme, layout.BorderNormal)+1, bez.H/2)
	icon := op.Icon != layout.IconNone
	if icon && math.Abs(r.W-r.H) < 1 {
		bez = layout.PointRect{X: r.X + 1, Y: r.Y + 1, W: max(r.W-2, 0), H: max(r.H-2, 0)}
		if op.Default {
			radius = bez.H / 2
		}
	}
	fill := mix(rgbaOf(p.theme.Surface), rgbaOf(p.theme.Text), 0.08)
	text := p.roleColor(op.Style)
	if op.Default {
		fill, text = rgbaOf(p.theme.Accent), rgba{1, 1, 1, 1}
		if op.Style.Bg != nil {
			fill = rgbaOf(op.Style.Bg)
		}
		if op.Style.Fg != nil {
			text = rgbaOf(op.Style.Fg)
		}
	}
	if op.Plain && !op.Disabled && op.Style.Fg == nil {
		text = mix(text, rgbaOf(p.theme.Title), hot)
	}
	if !op.Plain {
		fill = p.dim(mix(mix(fill, rgba{1, 1, 1, 1}, 0.1*hot*(1-down)), rgbaOf(p.theme.Background), 0.25*down), op.Disabled)
		p.fillRounded(bez, radius, colorFromRGBA(fill))
		if !op.Default {
			p.strokeRounded(bez, radius, strokeWidth, colorFromRGBA(p.dim(rgbaOf(p.theme.Border), op.Disabled)))
		}
	} else if hot > 0 || down > 0 {
		lift := 0.1*hot + 0.06*down
		c := mix(rgbaOf(p.theme.Background), rgbaOf(p.theme.Text), 0.16)
		c.A *= min(lift/0.16, 1)
		p.fillRounded(bez, radius, colorFromRGBA(c))
	}
	if icon {
		size := min(r.W, r.H) * 0.5
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		shape, ok := symbol(op.Icon, cx, cy, size)
		if !ok {
			shape, ok = iconShape(op.Icon, cx-size/2, cy-size/2, size, size, size)
		}
		if ok {
			p.shape(shape, p.dim(text, op.Disabled))
		}
	} else {
		font := p.text.font(op.Style)
		width := p.text.width(font, op.Label)
		content := op.Content
		if content == (layout.PointRect{}) {
			content = r
		}
		baseline := content.Y + (content.H-(font.ascent+font.descent))/2 + font.ascent
		p.ctx.Set("font", font.css)
		p.fill(colorFromRGBA(p.dim(text, op.Disabled)))
		p.ctx.Set("textAlign", "start")
		p.ctx.Call("fillText", op.Label, content.X+(content.W-width)/2, baseline)
	}
	if op.Focused && !op.Disabled {
		p.strokeRounded(bez, radius+2, canvasFocusRing, p.theme.Accent)
	}
}

func (p *canvasPainter) toggle(op layout.ControlOp, hot, down bool) {
	r := op.Content
	if r == (layout.PointRect{}) {
		r = op.Rect
	}
	cols := 3.0
	if op.Kind == layout.ControlSwitch {
		cols = 4
	}
	areaW := cols * p.metrics.Column
	cy := r.Y + r.H/2
	var ind layout.PointRect
	switch op.Kind {
	case layout.ControlSwitch:
		ind = layout.PointRect{X: r.X + (areaW-26)/2, Y: cy - 7.5, W: 26, H: 15}
		track := p.theme.Track
		if op.Checked {
			track = p.theme.Accent
		}
		p.fillRounded(ind, 7.5, colorFromRGBA(p.dim(p.shade(rgbaOf(track), hot, down), op.Disabled)))
		kx := ind.X + 2
		if op.Checked {
			kx = ind.X + 26 - 2 - 11
		}
		p.fillRounded(layout.PointRect{X: kx, Y: cy - 5.5, W: 11, H: 11}, 5.5, colorFromRGBA(p.dim(rgbaOf(color.White), op.Disabled)))
	case layout.ControlRadio:
		ind = layout.PointRect{X: r.X + (areaW-14)/2, Y: cy - 7, W: 14, H: 14}
		p.fillRounded(ind, 7, colorFromRGBA(p.dim(p.shade(rgbaOf(p.theme.Field), hot, down), op.Disabled)))
		p.strokeRounded(ind, 7, 1, colorFromRGBA(p.dim(rgbaOf(p.theme.Border), op.Disabled)))
		if op.Checked {
			p.fillRounded(layout.PointRect{X: ind.X + 4.5, Y: ind.Y + 4.5, W: 5, H: 5}, 2.5, colorFromRGBA(p.dim(rgbaOf(color.White), op.Disabled)))
		}
	default:
		ind = layout.PointRect{X: r.X + (areaW-14)/2, Y: cy - 7, W: 14, H: 14}
		p.fillRounded(ind, 3, colorFromRGBA(p.dim(p.shade(rgbaOf(p.theme.Field), hot, down), op.Disabled)))
		p.strokeRounded(ind, 3, 1, colorFromRGBA(p.dim(rgbaOf(p.theme.Border), op.Disabled)))
		if op.Checked {
			p.ctx.Call("beginPath")
			p.ctx.Call("moveTo", ind.X+3.5, ind.Y+7.2)
			p.ctx.Call("lineTo", ind.X+6, ind.Y+9.8)
			p.ctx.Call("lineTo", ind.X+10.5, ind.Y+4.3)
			p.stroke(colorFromRGBA(p.dim(rgbaOf(color.White), op.Disabled)))
			p.ctx.Set("lineWidth", 1.8)
			p.ctx.Call("stroke")
		}
	}
	if op.Label != "" {
		font := p.text.font(op.Style)
		baseline := r.Y + (r.H-(font.ascent+font.descent))/2 + font.ascent
		p.ctx.Set("font", font.css)
		p.fill(colorFromRGBA(p.dim(p.roleColor(op.Style), op.Disabled)))
		p.ctx.Call("fillText", op.Label, r.X+areaW, baseline)
	}
	if op.Focused && !op.Disabled {
		p.strokeRounded(op.Rect, canvasRadius(p.theme, layout.BorderNormal), canvasFocusRing, p.theme.Accent)
	}
}

func (p *canvasPainter) shade(c rgba, hot, down bool) rgba {
	h, d := 0.0, 0.0
	if hot {
		h = 1
	}
	if down {
		d = 1
	}
	return mix(mix(c, rgba{1, 1, 1, 1}, 0.1*h*(1-d)), rgbaOf(p.theme.Background), 0.25*d)
}

func (p *canvasPainter) slider(op layout.ControlOp, hot, down bool) {
	r := op.Rect
	cy := r.Y + r.H/2
	track := layout.PointRect{X: r.X, Y: cy - 2, W: r.W, H: 4}
	p.fillRounded(track, 2, colorFromRGBA(p.dim(p.shade(rgbaOf(p.theme.Track), hot, down), op.Disabled)))
	v := min(max(op.Value, 0), 1)
	fill := p.theme.Accent
	if op.Fill != nil {
		fill = op.Fill
	}
	p.fillRounded(layout.PointRect{X: r.X, Y: cy - 2, W: r.W * v, H: 4}, 2, colorFromRGBA(p.dim(p.shade(rgbaOf(fill), hot, down), op.Disabled)))
	knobX := r.X + r.W*v
	p.fillRounded(layout.PointRect{X: knobX - 7, Y: cy - 7, W: 14, H: 14}, 7, colorFromRGBA(p.dim(rgbaOf(color.White), op.Disabled)))
	if hot || down {
		p.strokeRounded(layout.PointRect{X: knobX - 7, Y: cy - 7, W: 14, H: 14}, 7, 1, p.theme.Accent)
	}
	if op.Focused && !op.Disabled {
		p.strokeRounded(r, canvasRadius(p.theme, layout.BorderNormal), canvasFocusRing, p.theme.Accent)
	}
}

func (p *canvasPainter) cells(op layout.CellsOp) {
	buf := uv.NewScreenBuffer(op.Cols, op.Lines)
	uv.NewStyledString(op.Content).Draw(buf, buf.Bounds())
	for y := range op.Lines {
		top := op.Rect.Y + float64(y)*p.metrics.Line
		for x := 0; x < op.Cols; {
			cell := buf.CellAt(x, y)
			if cell == nil || cell.Width == 0 {
				x++
				continue
			}
			cx := op.Rect.X + float64(x)*p.metrics.Column
			cw := float64(max(cell.Width, 1)) * p.metrics.Column
			if g, ok := canvasGridGlyph(cell, cx, top, cw, p.metrics.Line); ok {
				p.cellShape(g, cell.Style, cx, top, cw)
				x += max(cell.Width, 1)
				continue
			}
			start, style := x, cell.Style
			var text strings.Builder
			for x < op.Cols {
				c := buf.CellAt(x, y)
				if c == nil || !c.Style.Equal(&style) || c.Width > 1 && x > start {
					break
				}
				if _, ok := canvasGridGlyph(c, 0, 0, 1, 1); ok {
					break
				}
				if c.Width == 0 {
					x++
					continue
				}
				text.WriteString(canvasCellText(c))
				x += max(c.Width, 1)
				if c.Width > 1 {
					break
				}
			}
			p.cellRun(text.String(), style, op.Rect.X+float64(start)*p.metrics.Column, top, float64(x-start)*p.metrics.Column)
		}
	}
}

func canvasGridGlyph(c *uv.Cell, x, y, w, h float64) (cellShape, bool) {
	r, size := utf8.DecodeRuneInString(c.Content)
	if size == 0 || size != len(c.Content) {
		return cellShape{}, false
	}
	return cellGlyph(r, x, y, w, h)
}

func canvasCellText(c *uv.Cell) string {
	if c.Content == "" {
		return " "
	}
	return c.Content
}

func (p *canvasPainter) cellColors(style uv.Style) (fg, bg rgba, hasBG bool) {
	fg = rgbaOf(p.theme.Text)
	if style.Fg != nil {
		fg = rgbaOf(style.Fg)
	}
	if style.Bg != nil {
		bg, hasBG = rgbaOf(style.Bg), true
	}
	if style.Attrs&uv.AttrReverse != 0 {
		if !hasBG {
			bg = rgbaOf(p.theme.Background)
		}
		fg, bg, hasBG = bg, fg, true
	}
	if style.Attrs&uv.AttrFaint != 0 {
		fg.A *= 0.6
	}
	return
}

func (p *canvasPainter) cellFill(c rgba, x, y, w, h float64) {
	s := p.scale
	if s <= 0 {
		s = 1
	}
	x0, y0 := math.Round(x*s)/s, math.Round(y*s)/s
	x1, y1 := math.Round((x+w)*s)/s, math.Round((y+h)*s)/s
	p.fill(colorFromRGBA(c))
	p.ctx.Call("fillRect", x0, y0, x1-x0, y1-y0)
}

func (p *canvasPainter) cellRun(text string, style uv.Style, x, y, w float64) {
	fg, bg, hasBG := p.cellColors(style)
	if hasBG {
		p.cellFill(bg, x, y, w, p.metrics.Line)
	}
	if strings.TrimSpace(text) == "" || style.Attrs&uv.AttrConceal != 0 {
		return
	}
	family := p.theme.MonoFont
	if family == "" {
		family = "ui-monospace, SFMono-Regular, Menlo, monospace"
	}
	fontStyle := layout.TextStyle{Role: layout.TextBody, Bold: style.Attrs&uv.AttrBold != 0, Italic: style.Attrs&uv.AttrItalic != 0}
	f := p.text.fontFamily(fontStyle, p.theme.FontSize, family)
	base := y + (p.metrics.Line-(f.ascent+f.descent))/2 + f.ascent
	p.ctx.Set("font", f.css)
	p.fill(colorFromRGBA(fg))
	p.ctx.Call("fillText", text, x, base)
	if style.Underline != uv.UnderlineNone {
		p.ctx.Call("fillRect", x, base+max(f.descent*0.35, 1), w, 1)
	}
	if style.Attrs&uv.AttrStrikethrough != 0 {
		p.ctx.Call("fillRect", x, base-f.ascent*0.35, w, 1)
	}
}

func (p *canvasPainter) cellShape(shape cellShape, style uv.Style, x, y, w float64) {
	fg, bg, hasBG := p.cellColors(style)
	if hasBG {
		p.cellFill(bg, x, y, w, p.metrics.Line)
	}
	if style.Attrs&uv.AttrConceal == 0 {
		p.shape(shape, fg)
	}
}

func (p *canvasPainter) image(op layout.ImageOp) {
	if op.Source == nil {
		p.imageFallback(op.Rect, op.Round)
		return
	}
	if !reflect.TypeOf(op.Source).Comparable() {
		p.imageFallback(op.Rect, op.Round)
		return
	}
	entry := p.images[op.Source]
	if entry == nil {
		entry = &canvasImage{}
		p.images[op.Source] = entry
	}
	entry.used = p.gen
	if !entry.bitmap.IsUndefined() && !entry.bitmap.IsNull() {
		p.drawImageBitmap(op, entry.bitmap)
		return
	}
	if entry.failed {
		p.imageFallback(op.Rect, op.Round)
		return
	}
	if entry.loading {
		return
	}
	entry.loading = true
	var data bytes.Buffer
	if err := png.Encode(&data, op.Source); err != nil {
		entry.failed, entry.loading = true, false
		p.imageFallback(op.Rect, op.Round)
		return
	}
	array := js.Global().Get("Uint8Array").New(data.Len())
	js.CopyBytesToJS(array, data.Bytes())
	blob := js.Global().Get("Blob").New([]any{array}, map[string]any{"type": "image/png"})
	create := js.Global().Get("createImageBitmap")
	if create.Type() != js.TypeFunction {
		entry.failed, entry.loading = true, false
		p.imageFallback(op.Rect, op.Round)
		return
	}
	promise := create.Invoke(blob)
	var success, failure js.Func
	settled := false
	release := func() {
		if settled {
			return
		}
		settled = true
		success.Release()
		failure.Release()
	}
	success = js.FuncOf(func(_ js.Value, args []js.Value) any {
		bitmap := js.Undefined()
		if len(args) > 0 {
			bitmap = args[0]
		}
		entry.loading = false
		if p.images[op.Source] == entry && len(args) > 0 {
			entry.bitmap = bitmap
		} else if !bitmap.IsUndefined() && !bitmap.IsNull() && bitmap.Get("close").Type() == js.TypeFunction {
			bitmap.Call("close")
		}
		release()
		if p.invalidate != nil {
			p.invalidate()
		}
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any {
		entry.loading, entry.failed = false, true
		release()
		if p.invalidate != nil {
			p.invalidate()
		}
		return nil
	})
	promise.Call("then", success, failure)
}

func (p *canvasPainter) drawImageBitmap(op layout.ImageOp, bitmap js.Value) {
	w, h := op.Source.Bounds().Dx(), op.Source.Bounds().Dy()
	r := fitRect(op.Rect, float64(w), float64(h), op.Fit)
	p.ctx.Call("save")
	if op.Round {
		p.roundedPath(op.Rect, canvasBoxRadius(op.Rect, true))
		p.ctx.Call("clip")
	}
	p.ctx.Call("drawImage", bitmap, r.X, r.Y, r.W, r.H)
	p.ctx.Call("restore")
}

func (p *canvasPainter) imageFallback(r layout.PointRect, round bool) {
	bg := p.theme.Surface
	if round {
		p.fillRounded(r, canvasBoxRadius(r, true), bg)
	} else {
		p.fillRect(r, bg)
	}
	font := p.text.font(layout.TextStyle{Role: layout.TextSubtle, Size: min(r.H*0.42, 24)})
	p.ctx.Set("font", font.css)
	p.ctx.Set("textAlign", "center")
	p.ctx.Set("textBaseline", "middle")
	p.fill(p.theme.Subtle)
	p.ctx.Call("fillText", "♪", r.X+r.W/2, r.Y+r.H/2)
	p.ctx.Set("textAlign", "start")
}

func (p *canvasPainter) pruneImages() {
	for source, entry := range p.images {
		if entry.used == p.gen {
			continue
		}
		if !entry.bitmap.IsUndefined() && !entry.bitmap.IsNull() && entry.bitmap.Get("close").Type() == js.TypeFunction {
			entry.bitmap.Call("close")
		}
		delete(p.images, source)
	}
}

var _ = strings.Builder{}
