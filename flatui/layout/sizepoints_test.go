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
