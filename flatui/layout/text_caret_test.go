package layout

import "testing"

func TestTextCaretPlacesCursor(t *testing.T) {
	text := func(c *Caret) Node {
		return Col{Children: []Node{
			Text{NodeBase: NodeBase{H: Fixed(1)}, String: "header"},
			Text{NodeBase: NodeBase{ID: "code", PadLeft: 2, H: Fixed(3)}, String: "abc\ndef\nghi", Caret: c},
		}}
	}
	if _, _, ok := Compose(text(nil), 20, 6).Cursor(); ok {
		t.Fatal("no caret, but a cursor")
	}
	x, y, ok := Compose(text(&Caret{X: 2, Y: 1}), 20, 6).Cursor()
	if !ok || x != 4 || y != 2 { // PadLeft 2, below the 1-line header
		t.Fatalf("cursor = %d,%d,%v, want 4,2", x, y, ok)
	}
	for _, c := range []Caret{{-1, 0}, {0, -1}, {18, 0}, {0, 3}} {
		if _, _, ok := Compose(text(&c), 20, 6).Cursor(); ok {
			t.Errorf("caret %+v outside the content still placed", c)
		}
	}
}

func TestTextCaretLastInPaintOrderWins(t *testing.T) {
	in := Input{NodeBase: NodeBase{H: Fixed(1), NoPadding: true}, Value: "hi", Caret: 1, Focused: true}
	tx := Text{NodeBase: NodeBase{H: Fixed(1)}, String: "code", Caret: &Caret{X: 3}}
	if x, y, _ := Compose(Col{Children: []Node{in, tx}}, 20, 4).Cursor(); x != 3 || y != 1 {
		t.Errorf("text after input: cursor %d,%d, want 3,1", x, y)
	}
	if x, y, _ := Compose(Col{Children: []Node{tx, in}}, 20, 4).Cursor(); y != 1 || x == 3 {
		t.Errorf("input after text: cursor %d,%d, want the input's", x, y)
	}
}

func TestTextCaretScrolledAwayIsHidden(t *testing.T) {
	root := Scroll{NodeBase: NodeBase{H: Fixed(2)}, OffsetY: 5, Child: Col{Children: []Node{
		Text{NodeBase: NodeBase{H: Fixed(1)}, String: "top", Caret: &Caret{}},
		Text{NodeBase: NodeBase{H: Fixed(10)}, String: "filler"},
	}}}
	if _, _, ok := Compose(root, 20, 2).Cursor(); ok {
		t.Fatal("caret scrolled out of view still placed")
	}
}

func TestSceneCellsOpCarriesCaret(t *testing.T) {
	root := Text{NodeBase: NodeBase{PadLeft: 1, PadTop: 1}, String: "abc\ndef", Caret: &Caret{X: 1, Y: 1}}
	scene := LayoutScene(root, 200, 100, Metrics{Column: 10, Line: 20, Border: 10})
	var got *Caret
	for _, op := range scene.Ops {
		if c, ok := op.(CellsOp); ok {
			got = c.Caret
		}
	}
	if got == nil || *got != (Caret{X: 2, Y: 2}) { // padding included
		t.Fatalf("CellsOp caret = %v, want {2 2}", got)
	}
	none := LayoutScene(Text{String: "abc", Caret: &Caret{X: 50}}, 200, 100, Metrics{Column: 10, Line: 20, Border: 10})
	for _, op := range none.Ops {
		if c, ok := op.(CellsOp); ok && c.Caret != nil {
			t.Fatalf("out-of-range caret reached the scene: %v", c.Caret)
		}
	}
}

func TestSceneEmitsCaretOpsForExtraCarets(t *testing.T) {
	m := Metrics{Column: 10, Line: 20, Border: 10}
	root := Text{NodeBase: NodeBase{PadLeft: 1}, String: "abc\ndef", Caret: &Caret{}, Carets: []Caret{{X: 2, Y: 0}, {X: 1, Y: 1}, {X: 40, Y: 0}}}
	scene := LayoutScene(root, 200, 100, m)
	var cells []PointRect
	for _, op := range scene.Ops {
		if c, ok := op.(CaretOp); ok {
			cells = append(cells, c.Cell)
		}
	}
	want := []PointRect{{X: 30, Y: 0, W: 10, H: 20}, {X: 20, Y: 20, W: 10, H: 20}} // padding included; out of range dropped
	if len(cells) != len(want) || cells[0] != want[0] || cells[1] != want[1] {
		t.Fatalf("caret cells = %v, want %v", cells, want)
	}
	// A terminal has one cursor: the primary.
	if x, y, ok := Compose(root, 20, 2).Cursor(); !ok || x != 1 || y != 0 {
		t.Fatalf("terminal cursor = %d,%d,%v", x, y, ok)
	}
}
