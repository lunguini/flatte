package layout

import (
	"maps"
	"testing"
)

// faceMetrics is testMetrics with a terminal face of 10pt columns and
// 20pt lines.
func faceMetrics() Metrics {
	m := testMetrics
	m.Faces = map[Face]CellSize{FaceTerminal: {Column: 10, Line: 20}}
	return m
}

func TestMetricsCell(t *testing.T) {
	m := faceMetrics()
	if c := m.Cell(FaceTerminal); c != (CellSize{10, 20}) {
		t.Fatalf("terminal cell %v", c)
	}
	if c := m.Cell(FaceCode); c != (CellSize{8, 16}) {
		t.Fatalf("code cell %v", c)
	}
	// A code entry never overrides Column and Line, and an empty cell is
	// no cell.
	m.Faces = map[Face]CellSize{FaceCode: {1, 1}, FaceTerminal: {0, 20}}
	if c := m.Cell(FaceCode); c != (CellSize{8, 16}) {
		t.Fatalf("code cell with an entry %v", c)
	}
	if c := m.Cell(FaceTerminal); c != (CellSize{8, 16}) {
		t.Fatalf("empty terminal cell %v", c)
	}
	if c := testMetrics.Cell(FaceTerminal); c != (CellSize{8, 16}) {
		t.Fatalf("terminal cell without faces %v", c)
	}
}

func TestCellSizeCells(t *testing.T) {
	r := CellSize{Column: 10, Line: 20}.Cells(PointRect{X: 25, Y: 40, W: 99, H: 59.9999999999})
	if r != (Rect{X: 2, Y: 2, W: 9, H: 3}) {
		t.Fatalf("cells %v", r)
	}
	if r := (CellSize{}).Cells(PointRect{W: 10, H: 10}); r != (Rect{}) {
		t.Fatalf("empty cell %v", r)
	}
}

func TestSceneTextFaceSizesItsGrid(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "term", W: Grow(1), H: Grow(1)}, String: "ab", Face: FaceTerminal, Caret: &Caret{X: 1, Y: 1}, Carets: []Caret{{X: 2, Y: 0}}},
	}}
	s := LayoutScene(root, 100, 64, faceMetrics())
	c, ok := s.Ops[0].(CellsOp)
	if !ok || c.Face != FaceTerminal || c.Cols != 10 || c.Lines != 3 {
		t.Fatalf("terminal grid %#v", s.Ops[0])
	}
	if want := (Text{String: "ab"}).Render(Rect{W: 10, H: 3}); c.Content != want {
		t.Fatalf("content %q, want %q", c.Content, want)
	}
	if c.Caret == nil || *c.Caret != (Caret{X: 1, Y: 1}) {
		t.Fatalf("caret %v", c.Caret)
	}
	// A further caret is placed in the face's cells.
	if k, ok := s.Ops[1].(CaretOp); !ok || k.Cell != (PointRect{X: 20, Y: 0, W: 10, H: 20}) {
		t.Fatalf("caret op %#v", s.Ops[1])
	}
	// The rects stay in points, whatever the face.
	if r := s.Rects["term"]; r != (PointRect{W: 100, H: 64}) {
		t.Fatalf("rect %v", r)
	}
}

func TestSceneTextFaceScrollsByItsLine(t *testing.T) {
	root := Text{NodeBase: NodeBase{W: Grow(1), H: Grow(1)}, String: "a\nb\nc\nd", Face: FaceTerminal, OffsetY: 0.5}
	s := LayoutScene(root, 100, 40, faceMetrics())
	c := s.Ops[0].(CellsOp)
	if c.Rect.Y != -10 || c.Rect.H != 60 || c.Lines != 3 {
		t.Fatalf("shifted grid %#v", c)
	}
}

func TestSceneDefaultFaceIsCode(t *testing.T) {
	// Without a face, a Text and a container's chrome are in the code
	// cells, whatever other faces measure.
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{W: Grow(1), H: Fixed(2)}, String: "ab"},
		Col{NodeBase: NodeBase{W: Grow(1), H: Fixed(1), Chrome: func(r Rect) string { return "#" }}},
	}}
	plain := LayoutScene(root, 80, 48, testMetrics)
	faced := LayoutScene(root, 80, 48, faceMetrics())
	if len(plain.Ops) != 2 || len(faced.Ops) != 2 {
		t.Fatalf("ops %#v / %#v", plain.Ops, faced.Ops)
	}
	for i := range plain.Ops {
		p, f := plain.Ops[i].(CellsOp), faced.Ops[i].(CellsOp)
		if p != f || f.Face != FaceCode || f.Cols != 10 {
			t.Fatalf("op %d: %#v / %#v", i, p, f)
		}
	}
	if !maps.Equal(plain.Rects, faced.Rects) {
		t.Fatalf("rects %v / %v", plain.Rects, faced.Rects)
	}
}
