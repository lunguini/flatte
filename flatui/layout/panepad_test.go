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
	s = LayoutScene(body(NodeBase{PanePadding: true, PadLeft: 2}), 200, 100, Metrics{Column: 8, Line: 16, Step: 8})
	if b, r := s.Rects["body"], s.Rects["row"]; r.Y != b.Y {
		t.Errorf("explicit padding should override the pane inset: top %v", r.Y-b.Y)
	}
}
