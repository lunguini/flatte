//go:build js && wasm

package window

import (
	"fmt"
	"math"
	"strings"
	"syscall/js"
	"unicode/utf8"

	"github.com/lunguini/flatte/flatui/layout"
)

type canvasTextLayout struct {
	ctx   js.Value
	theme Theme
}

type canvasFont struct {
	css, family                  string
	size, ascent, descent, pitch float64
	style                        layout.TextStyle
}

func (t canvasTextLayout) font(style layout.TextStyle) canvasFont {
	th := t.theme
	size := th.FontSize
	family := th.Font
	if style.Role == layout.TextTitle {
		size, style.Bold = th.TitleSize, true
	}
	if style.Size > 0 && !math.IsNaN(style.Size) && !math.IsInf(style.Size, 0) {
		size = min(style.Size, maxFontSize)
	}
	if size <= 0 {
		size = 13
	}
	if family == "" {
		family = "system-ui, sans-serif"
	}
	return t.fontFamily(style, size, family)
}

func (t canvasTextLayout) fontFamily(style layout.TextStyle, size float64, family string) canvasFont {
	th := t.theme
	spacing := th.LineSpacing
	if style.Leading > 0 && !math.IsNaN(style.Leading) && !math.IsInf(style.Leading, 0) {
		spacing = min(max(style.Leading, minLeading), maxLeading)
	}
	weight := "400"
	if style.Bold {
		weight = "700"
	}
	fontCSS := fmt.Sprintf("%s %s %gpx %s", func() string {
		if style.Italic {
			return "italic"
		}
		return "normal"
	}(), weight, size, canvasCSSFamily(family))
	t.ctx.Set("font", fontCSS)
	metrics := t.ctx.Call("measureText", "Mg")
	ascent := canvasMetric(metrics.Get("fontBoundingBoxAscent"))
	descent := canvasMetric(metrics.Get("fontBoundingBoxDescent"))
	if ascent+descent <= 0 {
		ascent = canvasMetric(metrics.Get("actualBoundingBoxAscent"))
		descent = canvasMetric(metrics.Get("actualBoundingBoxDescent"))
	}
	if ascent+descent <= 0 {
		ascent, descent = size*0.78, size*0.22
	}
	if spacing <= 0 {
		spacing = 1.25
	}
	return canvasFont{css: fontCSS, family: family, size: size, ascent: ascent, descent: descent, pitch: math.Round((ascent + descent) * spacing), style: style}
}

func canvasMetric(value js.Value) float64 {
	if value.Type() == js.TypeNumber {
		return value.Float()
	}
	return 0
}

func canvasCSSFamily(family string) string {
	if strings.Contains(family, ",") || strings.HasPrefix(family, "\"") || strings.HasPrefix(family, "'") {
		return family
	}
	return `"` + strings.ReplaceAll(family, `"`, `\"`) + `"`
}

func (t canvasTextLayout) monoFont() canvasFont {
	family := t.theme.MonoFont
	if family == "" {
		family = "ui-monospace, SFMono-Regular, Menlo, monospace"
	}
	return t.fontFamily(layout.TextStyle{Role: layout.TextBody}, t.theme.FontSize, family)
}

func (t canvasTextLayout) width(font canvasFont, text string) float64 {
	t.ctx.Set("font", font.css)
	return t.ctx.Call("measureText", text).Get("width").Float()
}

func (t canvasTextLayout) measure(style layout.TextStyle, text string) (float64, canvasFont) {
	f := t.font(style)
	return t.width(f, text), f
}

func (t canvasTextLayout) metrics() layout.Metrics {
	body := t.font(layout.TextStyle{Role: layout.TextBody})
	mono := t.monoFont()
	column := t.width(mono, "0")
	if column <= 0 {
		column = t.theme.FontSize * 0.6
	}
	line := body.pitch
	if line <= 0 {
		line = t.theme.FontSize * 1.25
	}
	return layout.Metrics{
		Column: column, Line: line, Step: stepPoints, Border: borderBand,
		DefaultPadding: true, ControlHeight: t.theme.ControlHeight,
		MeasureText: func(text string, style layout.TextStyle, maxWidth float64) (float64, float64) {
			font := t.font(style)
			lines := wrapWords(text, maxWidth, func(s string) float64 { return t.width(font, s) })
			width := 0.0
			for _, line := range lines {
				width = max(width, t.width(font, line))
			}
			return width, float64(len(lines)) * font.pitch
		},
		MeasureRuns: func(runs []layout.Run, maxWidth float64) (float64, float64) {
			rt := t.richText(runs, layout.TextStyle{})
			width, height := 0.0, 0.0
			for _, line := range rt.lines(maxWidth) {
				width = max(width, rt.width(line.start, line.end))
				height += line.metrics.pitch
			}
			return width, height
		},
	}
}

type canvasRichText struct {
	text       string
	runs       []layout.Run
	fonts      []canvasFont
	ends       []int
	textLayout canvasTextLayout
}

type canvasLineMetrics struct{ pitch, ascent, height float64 }

type canvasLine struct {
	span
	y float64
	canvasLineMetrics
}

func (t canvasTextLayout) richText(runs []layout.Run, fallback layout.TextStyle) canvasRichText {
	rt := canvasRichText{runs: runs, fonts: make([]canvasFont, len(runs)), ends: make([]int, len(runs)), textLayout: t}
	var b strings.Builder
	for i, run := range runs {
		b.WriteString(run.Text)
		style := run.Style
		if zeroCanvasStyle(style) {
			style = fallback
		}
		rt.fonts[i] = t.font(style)
		rt.ends[i] = b.Len()
	}
	rt.text = b.String()
	return rt
}

func zeroCanvasStyle(style layout.TextStyle) bool {
	return style.Role == layout.TextBody && style.Fg == nil && style.Bg == nil && style.Size == 0 && style.Leading == 0 &&
		!style.Bold && !style.Italic && !style.Underline && !style.Strikethrough
}

func (rt canvasRichText) pieces(a, b int, fn func(int, int, int)) {
	start := 0
	for i, end := range rt.ends {
		if lo, hi := max(a, start), min(b, end); lo < hi {
			fn(i, lo, hi)
		}
		start = end
	}
}

func (rt canvasRichText) width(a, b int) float64 {
	width := 0.0
	rt.pieces(a, b, func(i, a, b int) { width += rt.textLayout.width(rt.fonts[i], rt.text[a:b]) })
	return width
}

func (rt canvasRichText) lines(maxWidth float64) []canvasLineMetricsSpan {
	spans := wrapSpans(rt.text, maxWidth, rt.width)
	lines := make([]canvasLineMetricsSpan, 0, len(spans))
	for _, span := range spans {
		lines = append(lines, canvasLineMetricsSpan{span: span, metrics: rt.lineMetrics(span)})
	}
	return lines
}

type canvasLineMetricsSpan struct {
	span
	metrics canvasLineMetrics
}

func (rt canvasRichText) lineMetrics(span span) canvasLineMetrics {
	var m canvasLineMetrics
	add := func(font canvasFont) {
		m.pitch = max(m.pitch, font.pitch)
		m.ascent = max(m.ascent, font.ascent)
		m.height = max(m.height, font.ascent+font.descent)
	}
	rt.pieces(span.start, span.end, func(i, _, _ int) { add(rt.fonts[i]) })
	if m.pitch == 0 && len(rt.fonts) > 0 {
		add(rt.fonts[0])
	}
	return m
}

func (rt canvasRichText) layout(op layout.TextOp) []canvasLine {
	wrap := 0.0
	if op.Wrap {
		wrap = op.Rect.W
	}
	spans := rt.lines(wrap)
	lines := make([]canvasLine, len(spans))
	y := op.Rect.Y
	for i, line := range spans {
		lines[i] = canvasLine{span: line.span, y: y, canvasLineMetrics: line.metrics}
		y += line.metrics.pitch
	}
	return lines
}

func (rt canvasRichText) nearest(line span, x float64) int {
	prev := 0.0
	for i := line.start; i < line.end; {
		_, n := utf8.DecodeRuneInString(rt.text[i:line.end])
		next := rt.width(line.start, i+n)
		if x < (prev+next)/2 {
			return i
		}
		prev, i = next, i+n
	}
	return line.end
}

func (rt canvasRichText) truncated(line span, width float64) (int, int) {
	cuts := []int{line.start}
	for i := line.start; i < line.end; {
		_, n := utf8.DecodeRuneInString(rt.text[i:line.end])
		i += n
		cuts = append(cuts, i)
	}
	runAt := func(i int) int {
		for k, end := range rt.ends {
			if i < end {
				return k
			}
		}
		return max(len(rt.ends)-1, 0)
	}
	last := 0
	for i := 1; i < len(cuts); i++ {
		run := runAt(max(cuts[i]-1, line.start))
		if rt.width(line.start, cuts[i])+rt.textLayout.width(rt.fonts[run], "…") > width {
			break
		}
		last = i
	}
	return cuts[last], runAt(max(cuts[last]-1, line.start))
}

func (p canvasTextLayout) caretAt(op layout.InputOp, x, y float64) int {
	field := p.layoutField(op, "")
	row := 0
	if !field.single {
		row = min(max(field.first+int(math.Floor((y-field.y0)/field.pitch)), 0), len(field.rows)-1)
	}
	return field.valueIndex(row, x-field.x0+field.scroll)
}

func (p canvasTextLayout) caretX(op layout.InputOp, caret int) float64 {
	field := p.layoutField(op, "")
	return field.x(caretRow(field.rows, caret), caret)
}

func (p canvasTextLayout) caretStep(op layout.InputOp, caret, dir int, x float64) int {
	field := p.layoutField(op, "")
	from := caretRow(field.rows, caret)
	to := min(max(from+dir, 0), len(field.rows)-1)
	if to == from {
		return caret
	}
	return field.valueIndex(to, x)
}

func (p canvasTextLayout) textOffset(op layout.TextOp, x, y float64) int {
	runs := op.Runs
	if len(runs) == 0 {
		runs = []layout.Run{{Text: op.Text, Style: op.Style}}
	}
	rt := p.richText(runs, op.Style)
	lines := rt.layout(op)
	for i, line := range lines {
		if y < line.y+line.pitch || i == len(lines)-1 {
			return rt.nearest(line.span, x-op.Rect.X)
		}
	}
	return 0
}

type canvasField struct {
	text        string
	rows        []areaRow
	font        canvasFont
	pitch       float64
	first       int
	scroll      float64
	x0, y0      float64
	caret, mark int
	at          int
	single      bool
	textLayout  canvasTextLayout
}

func (p canvasTextLayout) layoutField(op layout.InputOp, marked string) canvasField {
	r := inputTextRect(op)
	inset := fieldInset
	if op.Field.W != 0 || op.Field.H != 0 {
		inset = 0
	}
	f := p.font(op.Style)
	fl := canvasField{font: f, x0: r.X + inset, y0: r.Y, single: !op.Multiline, textLayout: p}
	avail := max(r.W-2*inset, 1)
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
		fl.pitch = max(f.pitch, 1)
		measure := func(s string) float64 { return p.width(f, s) }
		fl.rows = areaRows(fl.text, avail, op.Wrap, measure)
		fl.first = firstRow(op.OffsetY, caretRow(fl.rows, fl.at), int(r.H/fl.pitch), len(fl.rows))
	}
	if op.Focused && (fl.single || !op.Wrap) {
		if x := fl.x(caretRow(fl.rows, fl.at), fl.at); x > avail-2 {
			fl.scroll = x - avail + 2
		}
	}
	return fl
}

func (fl canvasField) shift(i int) int {
	if i > fl.caret {
		return i + fl.mark
	}
	return i
}

func (fl canvasField) x(row, at int) float64 {
	if row < 0 || row >= len(fl.rows) {
		return 0
	}
	r := fl.rows[row]
	at = min(max(at, r.start), r.end)
	return fl.textLayout.width(fl.font, fl.text[r.start:at])
}

func (fl canvasField) index(row int, x float64) int {
	if row < 0 || row >= len(fl.rows) {
		return 0
	}
	r := fl.rows[row]
	prev := 0.0
	for i := r.start; i < r.end; {
		_, n := utf8.DecodeRuneInString(fl.text[i:r.end])
		next := fl.textLayout.width(fl.font, fl.text[r.start:i+n])
		if x < (prev+next)/2 {
			return i
		}
		prev, i = next, i+n
	}
	return r.end
}

func (fl canvasField) valueIndex(row int, x float64) int {
	at := fl.index(row, x)
	if at > fl.caret+fl.mark {
		return at - fl.mark
	}
	if at > fl.caret {
		return fl.caret
	}
	return at
}

func (p canvasTextLayout) roleFont(style layout.TextStyle) canvasFont { return p.font(style) }

func (p canvasTextLayout) measureLine(style layout.TextStyle, text string) float64 {
	font := p.font(style)
	return p.width(font, text)
}

func (p canvasTextLayout) placeholderStyle() layout.TextStyle {
	return layout.TextStyle{Role: layout.TextSubtle}
}
