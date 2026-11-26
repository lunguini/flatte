package layout

import "testing"

func TestPointsSizes(t *testing.T) {
	root := Row{Children: []Node{
		&Spacer{NodeBase: NodeBase{ID: "a", W: Points(20), H: Points(17)}},
		&Spacer{NodeBase: NodeBase{ID: "b", W: Points(16), H: Points(16)}},
		&Spacer{NodeBase: NodeBase{ID: "c", W: Points(0), H: Points(-5)}},
		&Spacer{NodeBase: NodeBase{ID: "rest", W: Grow(1)}},
	}}
	// A terminal rounds points up to nominal cells: 20pt is 3 columns, 17pt
	// two lines; whole cells stay whole.
	cells := Solve(root, 40, 10)
	for id, want := range map[string]Rect{
		"a":    {X: 0, Y: 0, W: 3, H: 2},
		"b":    {X: 3, Y: 0, W: 2, H: 1},
		"c":    {X: 5, Y: 0, W: 0, H: 0},
		"rest": {X: 5, Y: 0, W: 35, H: 10},
	} {
		if got := cells[id]; got != want {
			t.Errorf("cells %s = %+v, want %+v", id, got, want)
		}
	}
	// A GUI lays them out exactly, whatever its cell size.
	scene := LayoutScene(root, 320, 160, testMetrics)
	for id, want := range map[string]PointRect{
		"a":    {X: 0, Y: 0, W: 20, H: 17},
		"b":    {X: 20, Y: 0, W: 16, H: 16},
		"rest": {X: 36, Y: 0, W: 284, H: 160},
	} {
		if got := scene.Rects[id]; got != want {
			t.Errorf("points %s = %+v, want %+v", id, got, want)
		}
	}
}

func TestPointsNormalize(t *testing.T) {
	if got := Points(-3); got != (Size{Kind: SizePoints}) {
		t.Errorf("Points(-3) = %+v", got)
	}
	if got := normalizeSize(Size{Kind: SizePoints, Value: -1}); got != (Size{Kind: SizePoints}) {
		t.Errorf("normalize = %+v", got)
	}
}

func TestSizeAtMost(t *testing.T) {
	root := Col{Children: []Node{
		&Spacer{NodeBase: NodeBase{ID: "cols", W: Fixed(84).AtMost(520), H: Fixed(1)}},
		&Spacer{NodeBase: NodeBase{ID: "under", W: Fixed(10).AtMost(520), H: Fixed(1)}},
		&Spacer{NodeBase: NodeBase{ID: "points", W: Points(600).AtMost(100), H: Points(40).AtMost(20)}},
		&Spacer{NodeBase: NodeBase{ID: "token", W: ControlSize().AtMost(10), H: Fixed(1)}},
		&Spacer{NodeBase: NodeBase{ID: "grow", W: Grow(1).AtMost(10), H: Fixed(1)}},
		&Spacer{NodeBase: NodeBase{ID: "uncapped", W: Fixed(5).AtMost(-1), H: Fixed(1)}},
	}}
	// A window caps in points whatever its column width (7pt here).
	m := testMetrics
	m.Column = 7
	scene := LayoutScene(root, 1000, 400, m)
	for id, want := range map[string]float64{"cols": 520, "under": 70, "points": 100, "token": 10, "grow": 1000, "uncapped": 35} {
		if got := scene.Rects[id].W; got != want {
			t.Errorf("points %s width = %v, want %v", id, got, want)
		}
	}
	if got := scene.Rects["points"].H; got != 20 {
		t.Errorf("points height = %v, want 20", got)
	}
	// A terminal reads the cap at the nominal cell: 520pt is 65 columns.
	cells := Solve(root, 120, 20)
	for id, want := range map[string]int{"cols": 65, "under": 10, "points": 13, "token": 2, "grow": 120, "uncapped": 5} {
		if got := cells[id].W; got != want {
			t.Errorf("cells %s width = %d, want %d", id, got, want)
		}
	}
	if got := Fixed(3).AtMost(-4).Max; got != 0 {
		t.Errorf("AtMost(-4).Max = %d, want 0", got)
	}
	if got := normalizeSize(Size{Kind: SizeFixed, Value: 2, Max: -1}); got != Fixed(2) {
		t.Errorf("normalize = %+v", got)
	}
}

// An overlay's capped width is its layer's: a popover no wider than the
// cap, and no wider than the frame.
func TestSizeAtMostOverlay(t *testing.T) {
	root := Col{Children: []Node{
		&Spacer{NodeBase: NodeBase{H: Grow(1)}},
		Col{NodeBase: NodeBase{ID: "pop", Overlay: true, W: Fixed(84).AtMost(520), H: Fixed(4)}},
	}}
	m := testMetrics
	m.Column = 7
	if got := LayoutScene(root, 1000, 400, m).Rects["pop"].W; got != 520 {
		t.Errorf("wide frame: width = %v, want 520", got)
	}
	if got := LayoutScene(root, 400, 400, m).Rects["pop"].W; got != 400 {
		t.Errorf("narrow frame: width = %v, want 400", got)
	}
}

// A size with Cells keeps its points in a GUI and is that many cells in a
// terminal: a hairline a terminal leaves out.
func TestSizeCells(t *testing.T) {
	col := Col{NodeBase: NodeBase{ID: "c", W: Fixed(10)}, Children: []Node{
		Label{Text: "a"},
		Col{NodeBase: NodeBase{ID: "line", W: Grow(1), H: Points(1).Cells(0), NoPadding: true}},
		Col{NodeBase: NodeBase{ID: "gap", W: Grow(1), H: Points(10).Cells(2), NoPadding: true}},
		Label{Text: "b"},
	}}
	_, rects := SolveAndCompose(col, 10, 10)
	if rects["line"].H != 0 || rects["gap"].H != 2 {
		t.Fatalf("cells: line %d gap %d", rects["line"].H, rects["gap"].H)
	}
	sc := LayoutScene(col, 200, 200, Metrics{Column: 8, Line: 16, Step: 8})
	if r := sc.Rects["line"]; r.H != 1 {
		t.Fatalf("points: line %v", r.H)
	}
	if r := sc.Rects["gap"]; r.H != 10 {
		t.Fatalf("points: gap %v", r.H)
	}
	if s := Points(3).Cells(1).Cells(-1); s.hasCells {
		t.Fatal("Cells(-1) did not take the length off")
	}
}
