package layout

import "testing"

// A pane body's inset is the same on every side in a GUI, so its first
// row's selection does not touch the header above it; a terminal keeps one
// column left and right and spares its rows.
func TestPanePadding(t *testing.T) {
	body := func(nb NodeBase) Node {
		nb.ID, nb.W, nb.H = "body", Grow(1), Grow(1)
		return Col{NodeBase: nb, Children: []Node{Label{NodeBase: NodeBase{ID: "row", H: Points(20)}, Text: "a"}}}
	}
	s := LayoutScene(body(NodeBase{PanePadding: true}), 200, 100, Metrics{Column: 8, Line: 16, Step: 8})
	b, r := s.Rects["body"], s.Rects["row"]
	left, top := r.X-b.X, r.Y-b.Y
	if left <= 0 || top != left || b.X+b.W-(r.X+r.W) != left {
		t.Errorf("GUI insets left %v top %v right %v, want equal and positive", left, top, b.X+b.W-(r.X+r.W))
	}
	_, rects := SolveAndCompose(body(NodeBase{PanePadding: true}), 20, 5)
	if c, rr := rects["body"], rects["row"]; rr.X-c.X != 1 || rr.Y != c.Y || c.X+c.W-(rr.X+rr.W) != 1 {
		t.Errorf("cell insets: body %+v row %+v, want one column each side, no rows", c, rr)
	}
	// An explicit side replaces only that side's inset: a list that wants
	// more room above keeps the pane's sides (they once all went).
	s = LayoutScene(body(NodeBase{PanePadding: true, PadTop: 2}), 200, 100, Metrics{Column: 8, Line: 16, Step: 8})
	if b, r := s.Rects["body"], s.Rects["row"]; r.Y-b.Y != 16 || r.X-b.X != left || b.X+b.W-(r.X+r.W) != left {
		t.Errorf("GUI PadTop 2: top %v left %v right %v, want 16, %v, %v", r.Y-b.Y, r.X-b.X, b.X+b.W-(r.X+r.W), left, left)
	}
	_, rects = SolveAndCompose(body(NodeBase{PanePadding: true, PadTop: 1, PadBottom: 1}), 20, 5)
	if c, rr := rects["body"], rects["row"]; rr.X-c.X != 1 || rr.Y-c.Y != 1 || c.X+c.W-(rr.X+rr.W) != 1 {
		t.Errorf("cell PadTop 1: body %+v row %+v, want a row above and a column each side", c, rr)
	}
	// Pad sets every side, as without PanePadding.
	s = LayoutScene(body(NodeBase{PanePadding: true, Pad: 2}), 200, 100, Metrics{Column: 8, Line: 16, Step: 8})
	if b, r := s.Rects["body"], s.Rects["row"]; r.X-b.X != 16 || r.Y-b.Y != 16 {
		t.Errorf("GUI Pad 2: left %v top %v, want 16", r.X-b.X, r.Y-b.Y)
	}
}
