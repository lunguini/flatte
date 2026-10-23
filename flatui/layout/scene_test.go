package layout

import (
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// testMetrics: 8pt columns, 16pt lines, 6pt steps, 1pt border; text is
// 0.75 of a column per rune and one line per "\n"-separated line, titles
// 20pt tall.
var testMetrics = Metrics{
	Column: 8, Line: 16, Step: 6, Border: 1,
	MeasureText: func(text string, style TextStyle, _ float64) (float64, float64) {
		w := 0.0
		lines := strings.Split(text, "\n")
		for _, l := range lines {
			w = max(w, 6*float64(utf8.RuneCountInString(l)))
		}
		h := 16.0
		if style.Role == TextTitle {
			h = 20
		}
		return w, h * float64(len(lines))
	},
}

func TestSceneOpsInPaintOrder(t *testing.T) {
	grey := lipgloss.Color("240")
	root := Col{
		NodeBase: NodeBase{ID: "card", Pad: 1, Box: Box{Border: BorderNormal, BorderColor: grey}},
		Children: []Node{
			Label{NodeBase: NodeBase{ID: "title"}, Text: "Hello", Style: TextStyle{Role: TextTitle}},
			Row{NodeBase: NodeBase{Gap: 1}, Children: []Node{
				Label{Text: "name:"},
				Input{NodeBase: NodeBase{ID: "name"}, Value: "Ada", Caret: 1, Focused: true},
			}},
		},
	}
	s := LayoutScene(root, 200, 100, testMetrics)
	frame := PointRect{0, 0, 200, 100}
	// The card's content box starts at 7,7: a 1pt border plus one 6pt step.
	want := []Op{
		BoxOp{Rect: frame, Clip: frame, Box: Box{Border: BorderNormal, BorderColor: grey}, ID: "card"},
		TextOp{Rect: PointRect{7, 7, 186, 20}, Clip: frame, Text: "Hello", Style: TextStyle{Role: TextTitle}},
		TextOp{Rect: PointRect{7, 27, 30, 16}, Clip: frame, Text: "name:"},
		InputOp{ID: "name", Rect: PointRect{43, 27, 150, 16}, Clip: frame, Value: "Ada", Caret: 1, Focused: true},
	}
	if !reflect.DeepEqual(s.Ops, want) {
		t.Fatalf("ops:\n%#v\nwant\n%#v", s.Ops, want)
	}
	if s.Rects["card"] != frame || s.Rects["name"] != (PointRect{43, 27, 150, 16}) {
		t.Fatalf("rects = %+v", s.Rects)
	}
}

func TestSceneLabelInsetAndBox(t *testing.T) {
	blue := lipgloss.Color("4")
	root := Col{Children: []Node{
		Label{NodeBase: NodeBase{PadLeft: 2, Box: Box{Background: blue}}, Text: "x"},
	}}
	s := LayoutScene(root, 100, 50, testMetrics)
	want := []Op{
		BoxOp{Rect: PointRect{0, 0, 100, 16}, Clip: PointRect{0, 0, 100, 50}, Box: Box{Background: blue}},
		TextOp{Rect: PointRect{12, 0, 88, 16}, Clip: PointRect{0, 0, 100, 50}, Text: "x"},
	}
	if !reflect.DeepEqual(s.Ops, want) {
		t.Fatalf("ops = %#v", s.Ops)
	}
}

func TestSceneCellsFallback(t *testing.T) {
	// ANSI Text and unknown leaves draw as a cell block: the node's own
	// terminal rendering at the whole number of cells its rect holds.
	styled := lipgloss.NewStyle().Bold(true).Render("ab")
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "t"}, String: styled + "\ncd"},
		testWidget{content: "w", NodeBase: NodeBase{H: Fixed(1)}},
	}}
	s := LayoutScene(root, 40, 64, testMetrics)
	if len(s.Ops) != 2 {
		t.Fatalf("ops = %#v", s.Ops)
	}
	c, ok := s.Ops[0].(CellsOp)
	if !ok || c.Rect != (PointRect{0, 0, 40, 32}) || c.Cols != 5 || c.Lines != 2 {
		t.Fatalf("text op = %#v", s.Ops[0])
	}
	if want := (Text{String: styled + "\ncd"}).Render(Rect{W: 5, H: 2}); c.Content != want {
		t.Fatalf("cells content = %q, want %q", c.Content, want)
	}
	if w, ok := s.Ops[1].(CellsOp); !ok || ansi.Strip(w.Content) != "w    " || w.Lines != 1 {
		t.Fatalf("widget op = %#v", s.Ops[1])
	}
}

func TestSceneChromeIsCells(t *testing.T) {
	root := Col{NodeBase: NodeBase{Chrome: func(r Rect) string { return strings.Repeat("#", r.W) }, H: Fixed(1)}}
	s := LayoutScene(Col{Children: []Node{root}}, 24, 16, testMetrics)
	if len(s.Ops) != 1 {
		t.Fatalf("ops = %#v", s.Ops)
	}
	if c, ok := s.Ops[0].(CellsOp); !ok || c.Content != "###" || c.Cols != 3 {
		t.Fatalf("chrome op = %#v", s.Ops[0])
	}
}

func TestSceneSpacerBackground(t *testing.T) {
	blue := lipgloss.Color("4")
	s := LayoutScene(Row{Children: []Node{NewSpacer().WithBackground(blue), NewSpacer()}}, 100, 10, testMetrics)
	want := []Op{BoxOp{Rect: PointRect{0, 0, 50, 10}, Clip: PointRect{0, 0, 100, 10}, Box: Box{Background: blue}}}
	if !reflect.DeepEqual(s.Ops, want) {
		t.Fatalf("ops = %#v (a spacer without background draws nothing)", s.Ops)
	}
}

func TestSceneScrollClipsAndShifts(t *testing.T) {
	root := Col{Children: []Node{
		Label{NodeBase: NodeBase{H: Fixed(1)}, Text: "head"},
		Scroll{NodeBase: NodeBase{ID: "s", H: Fixed(2)}, OffsetY: 1, Child: Col{Children: []Node{
			Label{NodeBase: NodeBase{ID: "a", H: Fixed(1)}, Text: "a"},
			Label{NodeBase: NodeBase{ID: "b", H: Fixed(1)}, Text: "b"},
			Label{NodeBase: NodeBase{ID: "c", H: Fixed(1)}, Text: "c"},
		}}},
	}}
	s := LayoutScene(root, 50, 100, testMetrics)
	window := PointRect{0, 16, 50, 32}
	// a is scrolled out of the window: no op. b and c draw, clipped to it.
	var texts []string
	for _, op := range s.Ops {
		if t2, ok := op.(TextOp); ok {
			texts = append(texts, t2.Text)
			if t2.Text != "head" && t2.Clip != window {
				t.Errorf("%s clip = %+v, want the window %+v", t2.Text, t2.Clip, window)
			}
		}
	}
	if !reflect.DeepEqual(texts, []string{"head", "b", "c"}) {
		t.Fatalf("drawn texts = %q", texts)
	}
	if got := s.Rects["a"]; got != (PointRect{0, 0, 50, 16}) {
		t.Fatalf("a rect = %+v (shifted up one line)", got)
	}
	if id, _ := s.HitTest(5, 5); id != "" {
		t.Fatalf("hit on header row = %q, want none (a is scrolled out)", id)
	}
	if id, r := s.HitTest(5, 20); id != "b" || r != (PointRect{0, 16, 50, 16}) {
		t.Fatalf("hit = %q %+v", id, r)
	}
}

func TestSceneOverlayLayer(t *testing.T) {
	root := Col{Children: []Node{
		Label{NodeBase: NodeBase{ID: "base"}, Text: "base"},
		Label{NodeBase: NodeBase{ID: "modal", Overlay: true, W: Fixed(4), H: Fixed(1)}, Text: "m"},
	}}
	s := LayoutScene(root, 100, 48, testMetrics)
	layer := PointRect{34, 16, 32, 16}
	if len(s.Ops) != 3 || s.Ops[1] != Op(LayerOp{Rect: layer}) {
		t.Fatalf("ops = %#v, want base text, the layer, then the modal", s.Ops)
	}
	if id, _ := s.HitTest(35, 20); id != "modal" {
		t.Fatalf("hit on modal = %q", id)
	}
	if id, _ := s.HitTest(1, 1); id != "base" {
		t.Fatalf("hit beside modal = %q", id)
	}
}

func TestSceneMeasuresLabelsAndInputs(t *testing.T) {
	root := Row{Children: []Node{
		Col{NodeBase: NodeBase{ID: "col", Bordered: true}, Children: []Node{
			Label{Text: "abcd", Style: TextStyle{Role: TextTitle}},
			Input{NodeBase: NodeBase{ID: "in", PadLeft: 1}, Value: "x"},
		}},
		NewSpacer(),
	}}
	s := LayoutScene(root, 300, 200, testMetrics)
	// Width: the label's measured 24pt plus two borders; height: a 20pt
	// title, a 16pt input line, two borders.
	if got := s.Rects["col"]; got.W != 26 || got.H != 200 {
		t.Fatalf("col = %+v", got)
	}
	if got := s.Rects["in"]; got.H != 16 || got.Y != 21 {
		t.Fatalf("input = %+v, want one 16pt line below the 20pt title", got)
	}
	for _, op := range s.Ops {
		if in, ok := op.(InputOp); ok && in.Rect.X != 1+6 {
			t.Fatalf("input content x = %v, want border + one step", in.Rect.X)
		}
	}
}

func TestSceneEmptyAndNilMeasure(t *testing.T) {
	if s := LayoutScene(Label{Text: "x"}, 0, 10, testMetrics); len(s.Ops) != 0 || len(s.Rects) != 0 {
		t.Fatalf("zero-size scene = %+v", s)
	}
	// Without MeasureText, labels measure as cells.
	m := testMetrics
	m.MeasureText = nil
	s := LayoutScene(Row{Children: []Node{Label{NodeBase: NodeBase{ID: "l"}, Text: "abc"}, NewSpacer()}}, 100, 16, m)
	if got := s.Rects["l"]; got.W != 24 {
		t.Fatalf("cell-measured label width = %v, want 3 columns", got.W)
	}
}

func TestPointRectCells(t *testing.T) {
	for _, tc := range []struct {
		r    PointRect
		want Rect
	}{
		{PointRect{0, 0, 80, 32}, Rect{0, 0, 10, 2}},
		{PointRect{12, 20, 47.9, 15.9}, Rect{1, 1, 5, 0}},
		{PointRect{-16, -32, 16, 32}, Rect{-2, -2, 2, 2}},
		{PointRect{8 * 0.1 * 30, 16 * 0.1 * 30, 0, 0}, Rect{3, 3, 0, 0}}, // FP noise below a cell boundary
	} {
		if got := tc.r.Cells(testMetrics); got != tc.want {
			t.Errorf("%+v.Cells = %+v, want %+v", tc.r, got, tc.want)
		}
	}
	if got := (PointRect{0, 0, math.Inf(1), math.NaN()}).Cells(testMetrics); got.W != maxInt || got.H != 0 {
		t.Errorf("non-finite = %+v", got)
	}
}

func TestSceneMultilineInputGrows(t *testing.T) {
	root := Col{Children: []Node{
		Input{NodeBase: NodeBase{ID: "ed"}, Value: "a\nb", Multiline: true, Wrap: true, SelStart: 2, SelEnd: 3, OffsetY: 1},
		Label{Text: "footer"},
	}}
	s := LayoutScene(root, 20*testMetrics.Column, 10*testMetrics.Line, testMetrics)
	if r := s.Rects["ed"]; r.H != 9*testMetrics.Line {
		t.Fatalf("multi-line input %+v, want the 9 lines the footer leaves", r)
	}
	for _, op := range s.Ops {
		if in, ok := op.(InputOp); ok && (!in.Multiline || !in.Wrap || in.SelStart != 2 || in.SelEnd != 3 || in.OffsetY != 1) {
			t.Fatalf("InputOp lost multi-line fields: %+v", in)
		}
	}
	// The terminal agrees.
	_, rects := SolveAndCompose(root, 20, 10)
	if rects["ed"].H != 9 {
		t.Fatalf("terminal multi-line input %+v, want 9 lines", rects["ed"])
	}
}

func TestHoverAndBackdropBoxesCarryTheirID(t *testing.T) {
	root := Col{Children: []Node{
		Row{NodeBase: NodeBase{ID: "row", H: Fixed(1), Box: Box{Hover: color.White}}, Children: []Node{Label{Text: "x"}}},
		Row{NodeBase: NodeBase{ID: "head", H: Fixed(1), Box: Box{Backdrop: &Backdrop{Blur: 4}}}, Children: []Node{Label{Text: "y"}}},
		Icon{NodeBase: NodeBase{ID: "eq"}, Name: IconEqualizer, Text: "♪ ", Animate: true},
	}}
	s := LayoutScene(root, 200, 100, testMetrics)
	ids := map[string]bool{}
	animated := false
	for _, op := range s.Ops {
		switch op := op.(type) {
		case BoxOp:
			ids[op.ID] = true
		case IconOp:
			animated = animated || op.Animate
		}
	}
	if !ids["row"] || !ids["head"] || !animated {
		t.Fatalf("box ids %v, animated icon %v", ids, animated)
	}
}

func TestBoxInsetYDrawsInsideTheRect(t *testing.T) {
	row := func(inset float64) Node {
		return Col{Children: []Node{Row{NodeBase: NodeBase{ID: "row", H: Fixed(2), Box: Box{Background: color.White, InsetY: inset}}, Children: []Node{Label{Text: "x"}}}}}
	}
	box := func(n Node) PointRect {
		for _, op := range LayoutScene(n, 200, 100, testMetrics).Ops {
			if b, ok := op.(BoxOp); ok {
				return b.Rect
			}
		}
		t.Fatal("no box")
		return PointRect{}
	}
	full, inset := box(row(0)), box(row(3))
	if inset.Y != full.Y+3 || inset.H != full.H-6 || inset.X != full.X || inset.W != full.W {
		t.Fatalf("inset %+v, full %+v", inset, full)
	}
	// A larger inset than half the height leaves an empty box, never a negative one.
	if huge := box(row(1e9)); huge.H != 0 {
		t.Fatalf("huge inset %+v", huge)
	}
	// Layout and hit-testing keep the whole rect.
	c := Compose(row(3), 20, 5)
	if r := c.Rects["row"]; r.H != 2 {
		t.Fatalf("row rect %+v", r)
	}
}
