package window

import (
	"image/color"
	"math"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestRichTextMeasure(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := darkTheme.metrics()
	body := darkTheme.styleFont(layout.TextStyle{})
	big := darkTheme.styleFont(layout.TextStyle{Size: 28})
	runs := []layout.Run{{Text: "Hello "}, {Text: "World", Style: layout.TextStyle{Size: 28}}}
	w, h := m.MeasureRuns(runs, 0)
	if want := body.width("Hello ") + big.width("World"); math.Abs(w-want) > 0.01 {
		t.Errorf("width %v, want %v", w, want)
	}
	// One line, as tall as its tallest font.
	if h != big.pitch() {
		t.Errorf("height %v, want the 28pt pitch %v", h, big.pitch())
	}
	// Wrapped: the line break falls between the runs, and each line is as
	// tall as its own fonts.
	w, h = m.MeasureRuns(runs, big.width("World")+1)
	if h != body.pitch()+big.pitch() {
		t.Errorf("wrapped height %v, want %v", h, body.pitch()+big.pitch())
	}
	if want := big.width("World"); math.Abs(w-want) > 0.01 {
		t.Errorf("wrapped width %v, want %v", w, want)
	}
	// A run of plain text measures as MeasureText measures it.
	text := "the quick brown fox jumps over the lazy dog"
	for _, mw := range []float64{0, 40, 90, 200} {
		w1, h1 := m.MeasureText(text, layout.TextStyle{}, mw)
		w2, h2 := m.MeasureRuns([]layout.Run{{Text: text[:9]}, {Text: text[9:]}}, mw)
		if math.Abs(w1-w2) > 0.5 || h1 != h2 {
			t.Errorf("maxWidth %v: runs %vx%v, text %vx%v", mw, w2, h2, w1, h1)
		}
	}
	// An empty line takes its run's font.
	if _, h := m.MeasureRuns([]layout.Run{{Text: "a\n", Style: layout.TextStyle{Size: 28}}, {Text: "b"}}, 0); h != big.pitch()+body.pitch() {
		t.Errorf("height %v with an empty line", h)
	}
}

func TestSnapshotRichText(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := color.RGBA{R: 255, A: 255}
	root := func() layout.Node {
		return layout.Row{Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "l"}, Runs: []layout.Run{
			{Text: "████"},
			{Text: "████", Style: layout.TextStyle{Fg: red}},
		}}}}
	}
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 3}, 2)
	if err != nil {
		t.Fatal(err)
	}
	m := metrics()
	s := layout.LayoutScene(root(), 30*m.Column, 3*m.Line, m)
	r := s.Rects["l"]
	// The label is its text wide (the row stretches it tall); sample its
	// first line.
	line := darkTheme.styleFont(layout.TextStyle{}).pitch()
	if want := 2 * darkTheme.styleFont(layout.TextStyle{}).width("████"); math.Abs(r.W-want) > 0.5 {
		t.Fatalf("label rect %+v, want %vpt wide", r, want)
	}
	at := func(fx float64) color.Color { return img.At(int((r.X+r.W*fx)*2), int((r.Y+line/2)*2)) }
	if c := at(0.25); !near(c, toColor(darkTheme.text), 40) {
		t.Errorf("first run %v, want the text color", c)
	}
	if c := at(0.75); !near(c, red, 40) {
		t.Errorf("second run %v, want red", c)
	}
}

func TestTextOffsetAndSelection(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	op := layout.TextOp{Rect: layout.PointRect{X: 10, Y: 5, W: 70, H: 100}, Text: "alpha beta gamma delta", Wrap: true, Selectable: true}
	op.Clip = op.Rect
	wt := windowText{th: func() theme { return darkTheme }}
	rt := darkTheme.opText(op)
	lines := rt.layout(op)
	if len(lines) < 2 {
		t.Fatalf("expected a wrap, got %+v", lines)
	}
	// Every rune boundary maps back to itself from just right of it, on
	// every line; above and below clamp to the first and last lines.
	for _, ln := range lines {
		for i := ln.start; i <= ln.end; i++ {
			x := op.Rect.X + rt.width(ln.start, i) + 0.5
			if got := wt.textOffset(op, x, ln.y+ln.pitch/2); got != i {
				t.Errorf("offset at byte %d's x = %d", i, got)
			}
		}
	}
	if got := wt.textOffset(op, 0, -50); got != 0 {
		t.Errorf("above the text: %d", got)
	}
	if got := wt.textOffset(op, 1e6, 1e6); got != len(op.Text) {
		t.Errorf("below the text: %d", got)
	}
	// A selection draws the selection color behind its bytes only.
	scene := layout.Scene{Ops: []layout.Op{op}}
	img := renderImage(scene, darkTheme, m, 100, 120, 2, nil, presentation{sel: textSel{op: 0, text: op.Text, from: 6, to: 10, ok: true}})
	ln := lines[0]
	sx := op.Rect.X + (rt.width(ln.start, 6)+rt.width(ln.start, 10))/2
	px := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	// The selection color is translucent: over the background.
	sel, bg := darkTheme.selection, darkTheme.bg
	mix := func(a, b float64) float64 { return a*sel.A + b*(1-sel.A) }
	highlight := toColor(rgba{R: mix(sel.R, bg.R), G: mix(sel.G, bg.G), B: mix(sel.B, bg.B), A: 1})
	// Sample the line's top edge, above the glyphs.
	if c := px(sx, ln.y+1); !near(c, highlight, 20) {
		t.Errorf("selected text background %v, want the selection color", c)
	}
	if c := px(op.Rect.X+rt.width(ln.start, 2), ln.y+1); near(c, highlight, 20) {
		t.Errorf("unselected text highlighted: %v", c)
	}
	// A stale selection (other text) draws nothing.
	img = renderImage(scene, darkTheme, m, 100, 120, 2, nil, presentation{sel: textSel{op: 0, text: "other", from: 6, to: 10, ok: true}})
	if c := px(sx, ln.y+1); near(c, highlight, 20) {
		t.Errorf("stale selection drawn: %v", c)
	}
}
