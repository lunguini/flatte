package layout

import (
	"image"
	"image/color"
	"math"
)

// Metrics sizes the layout vocabulary for a backend that lays out in points,
// such as a GUI: Fixed sizes are columns and lines, Pad and Gap are spacing
// steps, and Border is the space a border takes on each side (the backend
// draws its stroke within that band, as a terminal draws a border within a
// cell). MeasureText reports the size of a
// Label's text in the backend's font for its style, wrapped by words at
// maxWidth points when maxWidth > 0 (a wrapping Label); nil measures text
// as cells (one column per cell of width, one line per line). MeasureRuns
// does the same for a rich-text Label's runs (styles resolved), wrapped as
// one text; nil measures their text in the Label's style.
type Metrics struct {
	Column, Line float64
	Step         float64
	Border       float64
	MeasureText  func(text string, style TextStyle, maxWidth float64) (w, h float64)
	MeasureRuns  func(runs []Run, maxWidth float64) (w, h float64)
}

// PointRect is a rect in points.
type PointRect struct{ X, Y, W, H float64 }

// Cells converts r to whole columns and lines under m, rounding each field
// down (a viewport n lines tall shows n whole lines). Apps that read
// geometry in cells — LayoutEvent rects, MouseEvent coordinates — get it
// this way from a points backend. Values just below a boundary because of
// floating-point noise count as the boundary.
func (r PointRect) Cells(m Metrics) Rect {
	return Rect{cellsOf(r.X, m.Column), cellsOf(r.Y, m.Line), cellsOf(r.W, m.Column), cellsOf(r.H, m.Line)}
}

func cellsOf(v, unit float64) int {
	if unit <= 0 || math.IsNaN(v) {
		return 0
	}
	f := math.Floor(v/unit + 1e-9)
	switch {
	case f >= float64(maxInt):
		return maxInt
	case f <= float64(-maxInt-1):
		return -maxInt - 1
	}
	return int(f)
}

// Op is one drawing operation of a Scene, in paint order. The set is closed:
// a backend draws these ten and nothing else. Every op's Clip is the visible
// region (the frame narrowed by enclosing Scrolls); draw only inside it.
type Op interface{ isOp() }

// BoxOp draws a node's Box: its border within the Metrics.Border band
// around the inside of Rect, Box.Background behind the content, and a
// selection highlight when Box.Selected.
type BoxOp struct {
	Rect, Clip PointRect
	Box        Box
}

// TextOp draws a Label's text from the top-left of Rect (the label's content
// box), one line per "\n" — and, when Wrap, wrapped by words at Rect's
// width as MeasureText wraps it. For a rich-text Label, Runs holds its runs
// with their styles resolved (Text is their text, Style the Label's), wrapped
// as MeasureRuns wraps them. Selectable is the Label's: the backend lets
// the user select and copy the text.
type TextOp struct {
	Rect, Clip PointRect
	Text       string
	Style      TextStyle
	Wrap       bool
	Runs       []Run
	Selectable bool
}

// InputOp draws a text field in Rect (the input's content box). Caret is a
// byte offset into Value; a focused field shows it and receives text input.
// Scrolling a long value to keep the caret visible is the backend's job.
// A Multiline field breaks lines at "\n" (and wraps them at its width when
// Wrap), highlights the bytes SelStart..SelEnd, and scrolls from line
// OffsetY (the app's, in terminal rows: a hint) as little as keeps the
// caret's line visible.
type InputOp struct {
	ID          string
	Rect, Clip  PointRect
	Value       string
	Caret       int
	Placeholder string
	Focused     bool
	Style       TextStyle

	Multiline, Wrap  bool
	SelStart, SelEnd int
	OffsetY          int
}

// IconOp draws an Icon's named shape in Rect (the icon's content box), in
// Style's color; IconNone or an unknown name draws Text instead.
type IconOp struct {
	Rect, Clip PointRect
	Name       IconName
	Text       string
	Style      TextStyle
}

// MeterOp draws a Meter's gauge in Rect (its content box) filled to Value
// (0..1); nil colors are the backend's default.
type MeterOp struct {
	Rect, Clip  PointRect
	Value       float64
	Fill, Track color.Color
}

// ScrollBarOp draws a ScrollBar's thumb in Rect (its content box): it
// starts Start of the way down the track and is Length of it long (both
// fractions of Rect's height). Show reports whether there is anything to
// scroll; when false, draw nothing. A nil Color is the backend's default.
type ScrollBarOp struct {
	Rect, Clip    PointRect
	Start, Length float64
	Show          bool
	Color         color.Color
}

// ControlKind is the control a ControlOp draws.
type ControlKind int

const (
	ControlButton ControlKind = iota
	ControlCheckbox
	ControlSwitch
	ControlRadio
	ControlSlider
)

// ControlOp draws a control in Rect (its content box): a Button, a Toggle
// (checkbox, switch, or radio, with its Label), or a Slider at Value. A
// backend draws hover and press itself, for controls with an ID.
type ControlOp struct {
	ID                         string
	Kind                       ControlKind
	Rect, Clip                 PointRect
	Label                      string
	Style                      TextStyle
	Checked                    bool
	Value                      float64
	Fill                       color.Color
	Default, Disabled, Focused bool
}

// CellsOp draws terminal content — a Text leaf's ANSI string, a container's
// Chrome, or any node the scene does not know — as a grid of Cols×Lines
// monospace cells from the top-left of Rect. It is how every existing node
// still shows in a GUI.
type CellsOp struct {
	Rect, Clip  PointRect
	Cols, Lines int
	Content     string
}

// ImageOp draws an Image's Source in Rect (its content box) as Fit says.
type ImageOp struct {
	Rect, Clip PointRect
	Source     image.Image
	Fit        ImageFit
}

// LayerOp starts an overlay layer: clear Rect to the window background
// before drawing the ops that follow, as the terminal clears its cells.
type LayerOp struct{ Rect PointRect }

func (BoxOp) isOp()       {}
func (TextOp) isOp()      {}
func (ImageOp) isOp()     {}
func (InputOp) isOp()     {}
func (IconOp) isOp()      {}
func (MeterOp) isOp()     {}
func (ScrollBarOp) isOp() {}
func (ControlOp) isOp()   {}
func (CellsOp) isOp()     {}
func (LayerOp) isOp()     {}

// Scene is a frame solved in points: its drawing operations, the rect of
// every ID'd node, and the paint-order hit list.
type Scene struct {
	Ops   []Op
	Rects map[string]PointRect
	hits  []hitOf[float64]
	tips  []tip
}

// tip is a node's tooltip over its visible area, in paint order; an empty
// text is an overlay layer, which hides the tips under it.
type tip struct {
	area PointRect
	text string
}

// TooltipAt is the tooltip of the innermost (last painted) node at (x, y)
// that has one, or "" — also under an overlay that has none of its own.
func (s Scene) TooltipAt(x, y float64) string {
	for i := len(s.tips) - 1; i >= 0; i-- {
		if a := s.tips[i].area; x >= a.X && x < a.X+a.W && y >= a.Y && y < a.Y+a.H {
			return s.tips[i].text
		}
	}
	return ""
}

// HitTest returns the ID and rect of the topmost ID'd node at (x, y), with
// the same rules as Composition.HitTest.
func (s Scene) HitTest(x, y float64) (string, PointRect) {
	var m pointMetric
	for i := len(s.hits) - 1; i >= 0; i-- {
		if h := s.hits[i]; containsOf[float64](m, h.area, x, y) {
			if h.id == "" {
				return "", PointRect{}
			}
			return h.id, PointRect(h.rect)
		}
	}
	return "", PointRect{}
}

// LayoutScene solves root in a width×height-point frame with m. It runs the
// same walk as Compose — distribution, clipping, overlays, hit order — in
// points instead of cells.
func LayoutScene(root Node, width, height float64, m Metrics) Scene {
	if !(width > 0) || !(height > 0) {
		return Scene{Rects: map[string]PointRect{}}
	}
	pm := pointMetric{m}
	p := &scenePainter{m: pm}
	s := solveTree[float64](pm, root, width, height, p)
	rects := make(map[string]PointRect, len(s.rects))
	for id, r := range s.rects {
		rects[id] = PointRect(r)
	}
	return Scene{Ops: p.ops, Rects: rects, hits: s.hits, tips: p.tips}
}

// pointMetric is the metric for Metrics.
type pointMetric struct{ Metrics }

func (p pointMetric) cols(n int) float64           { return float64(n) * p.Column }
func (p pointMetric) lines(n int) float64          { return float64(n) * p.Line }
func (p pointMetric) steps(n int) float64          { return float64(n) * p.Step }
func (p pointMetric) points(n int, _ bool) float64 { return float64(n) }
func (p pointMetric) border() float64              { return p.Border }
func (pointMetric) add(a, b float64) float64       { return a + b }
func (pointMetric) sub(a, b float64) float64       { return a - b }
func (pointMetric) times(a float64, k int) float64 { return a * float64(k) }
func (pointMetric) half(a float64) float64         { return a / 2 }
func (pointMetric) one() float64                   { return 0 }
func (pointMetric) spread(rem float64, n int) (float64, int) {
	return rem / float64(n), 0
}
func (pointMetric) share(total, ratio float64) float64 {
	if total <= 0 || !(ratio > 0) {
		return 0
	}
	return total * min(ratio, 1)
}

// leaf measures the known leaves in points: text by the backend's font,
// inputs as one line (a multi-line one grows), and padding and borders in
// steps and strokes rather than cells. Other nodes report their Size, read
// as columns and lines.
func (p pointMetric) leaf(n Node, width float64) (w, h sized[float64]) {
	switch l := n.(type) {
	case Label:
		if l.Wrap {
			maxWidth := 0.0 // not known yet: unwrapped
			if width >= 0 {
				maxWidth = max(width-horizOf[float64](p, insetsBase[float64](p, l.NodeBase)), 1e-9)
			}
			tw, th := p.label(l, maxWidth)
			w, h = p.content(l.NodeBase, tw, th)
			if l.W.Kind == SizeAuto {
				w = growSized[float64]()
			}
			return w, h
		}
		tw, th := p.label(l, 0)
		return p.content(l.NodeBase, tw, th)
	case Text:
		cw, ch := measureString(l.String)
		return p.content(l.NodeBase, p.cols(cw), p.lines(ch))
	case Image:
		if pw, ph := l.pixels(); pw > 0 && ph > 0 {
			return p.content(l.NodeBase, float64(pw), float64(ph))
		}
		// No picture: its text, drawn as a Label's.
		tw, th := p.label(l.label(), 0)
		return p.content(l.NodeBase, tw, th)
	case Icon:
		// Columns, not the GUI font: an icon lines up with the terminal.
		cw, ch := measureString(l.Style.Render(l.Text))
		return p.content(l.NodeBase, p.cols(cw), p.lines(ch))
	case Meter:
		w, h = p.content(l.NodeBase, 0, p.Line)
		if l.W.Kind == SizeAuto {
			w = growSized[float64]()
		}
		return w, h
	case Input:
		w, h = p.content(l.NodeBase, 0, p.Line)
		if l.W.Kind == SizeAuto {
			w = growSized[float64]()
		}
		if l.Multiline && l.H.Kind == SizeAuto {
			h = growSized[float64]()
		}
		return w, h
	}
	return sizedOf[float64](p, n)
}

// content sizes a leaf whose Auto axes hold w×h of content inside its
// padding and border.
func (p pointMetric) content(nb NodeBase, w, h float64) (sw, sh sized[float64]) {
	sw, sh = toSized[float64](p, nb.W, true), toSized[float64](p, nb.H, false)
	inset := insetsBase[float64](p, nb)
	if sw.Kind == SizeAuto {
		sw = sized[float64]{Kind: SizeContent, Value: w + horizOf[float64](p, inset)}
	}
	if sh.Kind == SizeAuto {
		sh = sized[float64]{Kind: SizeContent, Value: h + vertOf[float64](p, inset)}
	}
	return sw, sh
}

// label measures a Label's text (see text).
func (p pointMetric) label(l Label, maxWidth float64) (float64, float64) {
	if len(l.Runs) == 0 {
		return p.text(l.Text, l.Style, maxWidth)
	}
	if p.MeasureRuns != nil {
		return p.MeasureRuns(l.runs(), maxWidth)
	}
	if p.MeasureText != nil {
		return p.MeasureText(l.plain(), l.Style, maxWidth)
	}
	text := l.plain()
	if maxWidth > 0 {
		text = wrapCells(text, max(int(maxWidth/p.Column), 1))
	}
	w, h := measureString(l.styled(text))
	return p.cols(w), p.lines(h)
}

func (p pointMetric) text(text string, style TextStyle, maxWidth float64) (float64, float64) {
	if p.MeasureText != nil {
		return p.MeasureText(text, style, maxWidth)
	}
	if maxWidth > 0 {
		text = wrapCells(text, max(int(maxWidth/p.Column), 1))
	}
	w, h := measureString(style.Render(text))
	return p.cols(w), p.lines(h)
}

// scenePainter records a Scene's ops. Nodes wholly outside the visible
// region produce none.
type scenePainter struct {
	m    pointMetric
	ops  []Op
	tips []tip
}

// tooltip records n's tooltip over its visible area.
func (p *scenePainter) tooltip(n Node, r, clip rect[float64]) {
	if t, ok := n.(tooltipNode); ok && t.tooltip() != "" {
		p.tips = append(p.tips, tip{area: PointRect(intersectOf[float64](p.m, r, clip)), text: t.tooltip()})
	}
}

func (p *scenePainter) visible(r, clip rect[float64]) bool {
	v := intersectOf[float64](p.m, r, clip)
	return v.W > 0 && v.H > 0
}

func (p *scenePainter) content(n Node, r rect[float64]) PointRect {
	return PointRect(insetOf[float64](p.m, r, insetsOf[float64](p.m, n)))
}

func (p *scenePainter) box(r, clip rect[float64], b Box) {
	if b.hasBorder() || b.hasBackground() || b.Selected {
		p.ops = append(p.ops, BoxOp{Rect: PointRect(r), Clip: PointRect(clip), Box: b})
	}
}

func (p *scenePainter) cells(r, clip rect[float64], render func(Rect) string) {
	c := PointRect(r).Cells(p.m.Metrics)
	if c.W <= 0 || c.H <= 0 {
		return
	}
	p.ops = append(p.ops, CellsOp{Rect: PointRect(r), Clip: PointRect(clip), Cols: c.W, Lines: c.H, Content: render(Rect{W: c.W, H: c.H})})
}

func (p *scenePainter) leaf(n Node, r, clip rect[float64]) {
	if !p.visible(r, clip) {
		return
	}
	p.tooltip(n, r, clip)
	control := func(op ControlOp, b Box) {
		p.box(r, clip, b)
		op.Rect, op.Clip = p.content(n, r), PointRect(clip)
		p.ops = append(p.ops, op)
	}
	switch l := n.(type) {
	case Button:
		control(ControlOp{ID: l.ID, Kind: ControlButton, Label: l.Label, Style: l.Style, Default: l.Default, Disabled: l.Disabled, Focused: l.Focused}, l.box())
	case Toggle:
		kind := ControlCheckbox
		switch l.Kind {
		case ToggleSwitch:
			kind = ControlSwitch
		case ToggleRadio:
			kind = ControlRadio
		}
		control(ControlOp{ID: l.ID, Kind: kind, Label: l.Label, Style: l.Style, Checked: l.Checked, Disabled: l.Disabled, Focused: l.Focused}, l.box())
	case Slider:
		control(ControlOp{ID: l.ID, Kind: ControlSlider, Value: l.value(), Fill: l.Fill, Disabled: l.Disabled, Focused: l.Focused}, l.box())
	case Label:
		p.box(r, clip, l.box())
		op := TextOp{Rect: p.content(n, r), Clip: PointRect(clip), Text: l.Text, Style: l.Style, Wrap: l.Wrap, Selectable: l.Selectable}
		if len(l.Runs) > 0 {
			op.Text, op.Runs = l.plain(), l.runs()
		}
		p.ops = append(p.ops, op)
	case Meter:
		p.box(r, clip, l.box())
		p.ops = append(p.ops, MeterOp{Rect: p.content(n, r), Clip: PointRect(clip), Value: l.value(), Fill: l.Fill, Track: l.Track})
	case Image:
		p.box(r, clip, l.box())
		if l.Source == nil {
			p.ops = append(p.ops, TextOp{Rect: p.content(n, r), Clip: PointRect(clip), Text: l.Text, Style: l.label().Style, Wrap: true})
		} else {
			p.ops = append(p.ops, ImageOp{Rect: p.content(n, r), Clip: PointRect(clip), Source: l.Source, Fit: l.Fit})
		}
	case Icon:
		p.box(r, clip, l.box())
		p.ops = append(p.ops, IconOp{Rect: p.content(n, r), Clip: PointRect(clip), Name: l.Name, Text: l.Text, Style: l.Style})
	case ScrollBar:
		p.box(r, clip, l.box())
		start, length, show := l.fraction()
		p.ops = append(p.ops, ScrollBarOp{Rect: p.content(n, r), Clip: PointRect(clip), Start: start, Length: length, Show: show, Color: l.Color})
	case Input:
		p.box(r, clip, l.box())
		p.ops = append(p.ops, InputOp{
			ID: l.ID, Rect: p.content(n, r), Clip: PointRect(clip),
			Value: l.Value, Caret: l.Caret, Placeholder: l.Placeholder, Focused: l.Focused, Style: l.Style,
			Multiline: l.Multiline, Wrap: l.Wrap, SelStart: l.SelStart, SelEnd: l.SelEnd, OffsetY: l.OffsetY,
		})
	case *Spacer:
		bg := l.bg
		if bg == nil {
			bg = l.Box.Background
		}
		p.box(r, clip, Box{Background: bg})
	default:
		p.cells(r, clip, n.Render)
	}
}

func (p *scenePainter) chrome(n Node, r, clip rect[float64]) {
	if !p.visible(r, clip) {
		return
	}
	p.tooltip(n, r, clip)
	if ch := getChrome(n); ch != nil {
		p.cells(r, clip, ch)
		return
	}
	p.box(r, clip, getBox(n))
}

func (p *scenePainter) layer(r rect[float64]) {
	p.ops = append(p.ops, LayerOp{Rect: PointRect(r)})
	p.tips = append(p.tips, tip{area: PointRect(r)})
}
