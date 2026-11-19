package layout

import "testing"

func tokenTree() Node {
	return Col{Align: AlignStart, Children: []Node{
		Text{NodeBase: NodeBase{ID: "row", H: RowSize(), W: RowSize()}, String: "a"},
		Text{NodeBase: NodeBase{ID: "control", H: ControlSize()}, String: "b"},
		Text{NodeBase: NodeBase{ID: "bar", H: BarSize()}, String: "c"},
		Text{NodeBase: NodeBase{ID: "header", H: HeaderSize()}, String: "d"},
		Text{NodeBase: NodeBase{ID: "bad", H: Size{Kind: SizeToken, Value: 99}}, String: "e"},
	}}
}

// A terminal reads token heights as one line, and widths as the token's
// regular points in columns.
func TestTokenSizesInCells(t *testing.T) {
	c := Compose(tokenTree(), 40, 20)
	for _, id := range []string{"row", "control", "bar", "header"} {
		if h := c.Rects[id].H; h != 1 {
			t.Errorf("%s height %d", id, h)
		}
	}
	if w := c.Rects["row"].W; w != 3 { // 24pt in 8pt columns
		t.Errorf("row width %d", w)
	}
	// A malformed token is solved as an unknown size kind is.
	unknown := Compose(Col{Align: AlignStart, Children: []Node{Text{NodeBase: NodeBase{ID: "bad", H: Size{Kind: 77}}, String: "e"}}}, 40, 20)
	if h, want := c.Rects["bad"].H, unknown.Rects["bad"].H; h != want {
		t.Errorf("malformed token height %d, unknown kind %d", h, want)
	}
	if got := normalizeSize(Size{Kind: SizeToken, Value: -1}); got.Kind != SizeAuto {
		t.Errorf("normalize %+v", got)
	}
}

// A GUI reads them from Metrics.Sizes, zero fields at the regular sizes.
func TestTokenSizesInPoints(t *testing.T) {
	m := testMetrics
	s := LayoutScene(tokenTree(), 400, 400, m)
	for id, want := range map[string]float64{"row": 24, "control": 28, "bar": 36, "header": 44} {
		if h := s.Rects[id].H; h != want {
			t.Errorf("default %s height %v, want %v", id, h, want)
		}
	}
	m.Sizes = Sizes{Row: 22, Control: 24, Bar: 30}
	s = LayoutScene(tokenTree(), 400, 400, m)
	for id, want := range map[string]float64{"row": 22, "control": 24, "bar": 30, "header": 44} {
		if h := s.Rects[id].H; h != want {
			t.Errorf("compact %s height %v, want %v", id, h, want)
		}
	}
	if w := s.Rects["row"].W; w != 22 {
		t.Errorf("row width %v", w)
	}
}
