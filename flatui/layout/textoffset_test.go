package layout

import (
	"strings"
	"testing"
)

// A Text with OffsetY draws its grid a fraction of a line higher and one
// line taller, clipped to the node, carets moving with it; out-of-range
// offsets draw it in place.
func TestTextOffsetYShiftsTheGrid(t *testing.T) {
	text := Text{NodeBase: NodeBase{ID: "code"}, String: "a\nb\nc\nd", Caret: &Caret{X: 0, Y: 1}, Carets: []Caret{{X: 0, Y: 3}}}
	root := Col{Children: []Node{
		Col{NodeBase: NodeBase{H: Points(16)}}, // a header line above
		Col{NodeBase: NodeBase{H: Points(48)}, Children: []Node{text}},
	}}
	cells := func(n Node) (CellsOp, []CaretOp) {
		var c CellsOp
		var carets []CaretOp
		for _, op := range LayoutScene(n, 80, 64, testMetrics).Ops {
			switch o := op.(type) {
			case CellsOp:
				c = o
			case CaretOp:
				carets = append(carets, o)
			}
		}
		return c, carets
	}

	plain, _ := cells(root)
	if plain.Rect != (PointRect{0, 16, 80, 48}) || plain.Lines != 3 {
		t.Fatalf("unshifted grid %+v", plain)
	}

	text.OffsetY = 0.25
	root.Children[1] = Col{NodeBase: NodeBase{H: Points(48)}, Children: []Node{text}}
	got, carets := cells(root)
	if got.Rect != (PointRect{0, 12, 80, 64}) || got.Lines != 4 {
		t.Fatalf("shifted grid rect %+v lines %d", got.Rect, got.Lines)
	}
	if got.Clip != (PointRect{0, 16, 80, 48}) {
		t.Fatalf("shifted grid not clipped to the node: %+v", got.Clip)
	}
	if !strings.Contains(got.Content, "d") {
		t.Fatalf("the extra line is not drawn: %q", got.Content)
	}
	if got.Caret == nil || *got.Caret != (Caret{X: 0, Y: 1}) {
		t.Fatalf("caret %+v", got.Caret)
	}
	if len(carets) != 1 || carets[0].Cell.Y != 12+3*16 {
		t.Fatalf("extra caret did not move with the text: %+v", carets)
	}

	for _, off := range []float64{-0.5, 1, 3} {
		text.OffsetY = off
		root.Children[1] = Col{NodeBase: NodeBase{H: Points(48)}, Children: []Node{text}}
		if c, _ := cells(root); c.Rect != plain.Rect {
			t.Fatalf("OffsetY %v moved the grid: %+v", off, c.Rect)
		}
	}
}
