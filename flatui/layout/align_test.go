package layout

import "testing"

func TestRowAlignPlacesChildrenVertically(t *testing.T) {
	row := func(a Align) Row {
		return Row{NodeBase: NodeBase{H: Fixed(5)}, Align: a, Children: []Node{
			Label{NodeBase: NodeBase{ID: "label"}, Text: "a"},
			Col{NodeBase: NodeBase{ID: "box", W: Fixed(2), H: Fixed(3)}},
			Col{NodeBase: NodeBase{ID: "grow", W: Fixed(2), H: Grow(1)}},
		}}
	}
	for _, c := range []struct {
		align      Align
		label, box Rect
	}{
		{AlignStretch, Rect{0, 0, 1, 5}, Rect{1, 0, 2, 3}},
		{AlignStart, Rect{0, 0, 1, 1}, Rect{1, 0, 2, 3}},
		{AlignCenter, Rect{0, 2, 1, 1}, Rect{1, 1, 2, 3}},
		{AlignEnd, Rect{0, 4, 1, 1}, Rect{1, 2, 2, 3}},
	} {
		r := Solve(row(c.align), 10, 5)
		if r["label"] != c.label || r["box"] != c.box || r["grow"] != (Rect{3, 0, 2, 5}) {
			t.Errorf("align %d: label %+v box %+v grow %+v; want %+v %+v and a whole-height grow", c.align, r["label"], r["box"], r["grow"], c.label, c.box)
		}
	}
}

func TestColAlignPlacesChildrenHorizontally(t *testing.T) {
	col := Col{NodeBase: NodeBase{W: Fixed(10)}, Align: AlignCenter, Children: []Node{
		Label{NodeBase: NodeBase{ID: "label"}, Text: "abcd"},
	}}
	if r := Solve(col, 10, 3); r["label"] != (Rect{3, 0, 4, 1}) {
		t.Fatalf("centered label %+v", r["label"])
	}
	col.Align = AlignEnd
	if r := Solve(col, 10, 3); r["label"] != (Rect{6, 0, 4, 1}) {
		t.Fatalf("end label %+v", r["label"])
	}
}

func TestAlignCentersInPoints(t *testing.T) {
	// A 17pt label beside a 32pt button, centered: 7.5pt above the label.
	m := scaledMetric(1)
	root := Row{NodeBase: NodeBase{H: Points(32)}, Align: AlignCenter, Children: []Node{
		Button{NodeBase: NodeBase{ID: "b", W: Points(32), H: Points(32)}, Icon: IconPlay, Label: "▶"},
		Label{NodeBase: NodeBase{ID: "l"}, Text: "Now"},
	}}
	s := LayoutScene(root, 200, 32, m.Metrics)
	if l := s.Rects["l"]; l.Y != 7.5 || l.H != 17 {
		t.Fatalf("label %+v, want y 7.5 h 17", l)
	}
	if b := s.Rects["b"]; b.Y != 0 || b.H != 32 {
		t.Fatalf("button %+v", b)
	}
}
