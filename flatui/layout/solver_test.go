package layout

import (
	"reflect"
	"testing"
)

func TestSolveColFixedGrowFixed(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "header", H: Fixed(3)},
			{ID: "body", H: Grow(1)},
			{ID: "footer", H: Fixed(1)},
		},
	}
	rects := Solve(tree, 80, 24)

	want := map[string]Rect{
		"root":   {0, 0, 80, 24},
		"header": {0, 0, 80, 3},
		"body":   {0, 3, 80, 20},
		"footer": {0, 23, 80, 1},
	}
	assertRects(t, rects, want)
}

func TestSolveRowFixedGrow(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: RowDir,
		Children: []Node{
			{ID: "sidebar", W: Fixed(20)},
			{ID: "content", W: Grow(1)},
		},
	}
	rects := Solve(tree, 80, 24)

	want := map[string]Rect{
		"root":    {0, 0, 80, 24},
		"sidebar": {0, 0, 20, 24},
		"content": {20, 0, 60, 24},
	}
	assertRects(t, rects, want)
}

func TestSolveGrowWeightProportional(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: RowDir,
		Children: []Node{
			{ID: "a", W: Grow(1)},
			{ID: "b", W: Grow(2)},
			{ID: "c", W: Grow(1)},
		},
	}
	rects := Solve(tree, 80, 10)

	// Total=4 weights. 80/4=20 each unit. a=20, b=40, c=20.
	want := map[string]Rect{
		"root": {0, 0, 80, 10},
		"a":    {0, 0, 20, 10},
		"b":    {20, 0, 40, 10},
		"c":    {60, 0, 20, 10},
	}
	assertRects(t, rects, want)
}

func TestSolveDeterministicRemainder(t *testing.T) {
	// 80px, 3 Grow(1) children: 80/3 = 26r2. First two get +1 (27), last gets 26.
	tree := Node{
		ID:  "root",
		Dir: RowDir,
		Children: []Node{
			{ID: "a", W: Grow(1)},
			{ID: "b", W: Grow(1)},
			{ID: "c", W: Grow(1)},
		},
	}
	rects := Solve(tree, 80, 10)

	a := rects["a"]
	b := rects["b"]
	c := rects["c"]

	if a.W != 27 || b.W != 27 || c.W != 26 {
		t.Fatalf("remainder distribution: a.W=%d b.W=%d c.W=%d (want 27,27,26)", a.W, b.W, c.W)
	}
	// Verify they're contiguous and fill the width.
	if a.X != 0 {
		t.Fatalf("a.X=%d want 0", a.X)
	}
	if b.X != a.X+a.W {
		t.Fatalf("b.X=%d want %d", b.X, a.X+a.W)
	}
	if c.X != b.X+b.W {
		t.Fatalf("c.X=%d want %d", c.X, b.X+b.W)
	}
	if c.X+c.W != 80 {
		t.Fatalf("right edge=%d want 80", c.X+c.W)
	}
}

func TestSolveCrossAxisStretch(t *testing.T) {
	// In a Col, children stretch to fill width (cross-axis) unless Fixed.
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "full", H: Fixed(10)},
			{ID: "narrow", H: Fixed(10), W: Fixed(30)},
		},
	}
	rects := Solve(tree, 80, 24)

	// "full" has no W constraint → stretches to 80.
	// "narrow" has Fixed(30) → stays 30 wide, positioned at X=0.
	if rects["full"].W != 80 {
		t.Fatalf("full.W=%d want 80 (stretch)", rects["full"].W)
	}
	if rects["narrow"].W != 30 {
		t.Fatalf("narrow.W=%d want 30 (fixed cross)", rects["narrow"].W)
	}
}

func TestSolveBorderInset(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "child", H: Grow(1)},
		},
	}
	tree.Bordered = true
	rects := Solve(tree, 80, 24)

	// Border = 1 cell inset on all sides.
	root := rects["root"]
	child := rects["child"]
	if child.X != 1 || child.Y != 1 {
		t.Fatalf("border inset: child at (%d,%d) want (1,1)", child.X, child.Y)
	}
	if child.W != 78 || child.H != 22 {
		t.Fatalf("border inset: child W=%d H=%d want 78,22", child.W, child.H)
	}
	_ = root
}

func TestSolvePaddingInset(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Pad: 2,
		Children: []Node{
			{ID: "child", H: Grow(1)},
		},
	}
	rects := Solve(tree, 80, 24)

	child := rects["child"]
	if child.X != 2 || child.Y != 2 {
		t.Fatalf("padding inset: child at (%d,%d) want (2,2)", child.X, child.Y)
	}
	if child.W != 76 || child.H != 20 {
		t.Fatalf("padding inset: child W=%d H=%d want 76,20", child.W, child.H)
	}
}

func TestSolveSpacingBetweenChildren(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: RowDir,
		Gap: 2,
		Children: []Node{
			{ID: "a", W: Fixed(20)},
			{ID: "b", W: Grow(1)},
			{ID: "c", W: Fixed(20)},
		},
	}
	rects := Solve(tree, 80, 10)

	a := rects["a"]
	b := rects["b"]
	c := rects["c"]

	// a at X=0, gap=2, b at X=22, gap=2, c at X=22+b.W+2.
	if a.X != 0 {
		t.Fatalf("a.X=%d want 0", a.X)
	}
	if b.X != a.X+a.W+2 {
		t.Fatalf("b.X=%d want %d (spacing)", b.X, a.X+a.W+2)
	}
	// Available for grow: 80 - 20 - 20 - 2*2 = 36
	if b.W != 36 {
		t.Fatalf("b.W=%d want 36 (after spacing)", b.W)
	}
	if c.X != b.X+b.W+2 {
		t.Fatalf("c.X=%d want %d (spacing)", c.X, b.X+b.W+2)
	}
}

func TestSolveNested(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "header", H: Fixed(3)},
			{
				ID:  "main",
				Dir: RowDir,
				H:   Grow(1),
				Children: []Node{
					{ID: "sidebar", W: Fixed(20)},
					{ID: "content", W: Grow(1)},
				},
			},
			{ID: "footer", H: Fixed(1)},
		},
	}
	rects := Solve(tree, 80, 24)

	// Header: 0,0 80x3. Footer: 0,23 80x1. Main: 0,3 80x20.
	// Sidebar: 0,3 20x20. Content: 20,3 60x20.
	assertRects(t, rects, map[string]Rect{
		"root":    {0, 0, 80, 24},
		"header":  {0, 0, 80, 3},
		"main":    {0, 3, 80, 20},
		"sidebar": {0, 3, 20, 20},
		"content": {20, 3, 60, 20},
		"footer":  {0, 23, 80, 1},
	})
}

func TestSolveLayerExcludedFromFlex(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "body", H: Grow(1)},
			{ID: "modal", H: Fixed(10), W: Fixed(40), Overlay: true},
		},
	}
	rects := Solve(tree, 80, 24)

	// "body" should get the full 24px height — the modal doesn't claim flex space.
	body := rects["body"]
	if body.H != 24 {
		t.Fatalf("body.H=%d want 24 (layer excluded from flex)", body.H)
	}

	// Modal is centered: x=(80-40)/2=20, y=(24-10)/2=7.
	modal := rects["modal"]
	if modal.X != 20 || modal.Y != 7 {
		t.Fatalf("modal at (%d,%d) want (20,7) (centered)", modal.X, modal.Y)
	}
	if modal.W != 40 || modal.H != 10 {
		t.Fatalf("modal W=%d H=%d want 40,10", modal.W, modal.H)
	}
}

func TestSolveLayerAutoFillsViewport(t *testing.T) {
	tree := Node{
		ID:  "root",
		Dir: ColDir,
		Children: []Node{
			{ID: "body", H: Grow(1)},
			{ID: "overlay", Overlay: true},
		},
	}
	rects := Solve(tree, 80, 24)

	// Auto layer fills the entire viewport.
	overlay := rects["overlay"]
	if overlay.W != 80 || overlay.H != 24 {
		t.Fatalf("overlay W=%d H=%d want 80,24 (full viewport)", overlay.W, overlay.H)
	}
	if overlay.X != 0 || overlay.Y != 0 {
		t.Fatalf("overlay at (%d,%d) want (0,0)", overlay.X, overlay.Y)
	}
}

func TestSolveLeafNode(t *testing.T) {
	tree := Node{ID: "solo", Dir: LeafDir}
	rects := Solve(tree, 80, 24)

	if rects["solo"] != (Rect{0, 0, 80, 24}) {
		t.Fatalf("leaf: got %+v want {0 0 80 24}", rects["solo"])
	}
}

func TestSolveEmptyContainer(t *testing.T) {
	tree := Node{
		ID:       "empty",
		Dir:      RowDir,
		Children: nil,
	}
	rects := Solve(tree, 80, 24)

	if rects["empty"] != (Rect{0, 0, 80, 24}) {
		t.Fatalf("empty container: got %+v", rects["empty"])
	}
}

func TestSolveOverflowFixedChildren(t *testing.T) {
	// Fixed children that exceed available space get clamped to 0 remainder.
	tree := Node{
		ID:  "root",
		Dir: RowDir,
		Children: []Node{
			{ID: "a", W: Fixed(50)},
			{ID: "b", W: Fixed(50)},
		},
	}
	rects := Solve(tree, 80, 10)

	// 50+50=100 > 80. Fixed children still get their requested size (the solver
	// doesn't truncate — it's the caller's problem to design a layout that fits).
	// They just overflow past the right edge.
	a := rects["a"]
	b := rects["b"]
	if a.W != 50 || b.W != 50 {
		t.Fatalf("overflow: a.W=%d b.W=%d want 50,50", a.W, b.W)
	}
}

func TestRectContains(t *testing.T) {
	r := Rect{5, 5, 10, 10}
	tests := []struct {
		x, y int
		want bool
	}{
		{5, 5, true},   // top-left corner
		{14, 14, true}, // bottom-right corner (exclusive)
		{4, 5, false},  // left edge
		{15, 5, false}, // right edge (exclusive)
		{5, 4, false},  // top edge
		{5, 15, false}, // bottom edge (exclusive)
	}
	for _, tc := range tests {
		if got := r.Contains(tc.x, tc.y); got != tc.want {
			t.Fatalf("Contains(%d,%d)=%v want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestRectInset(t *testing.T) {
	r := Rect{0, 0, 80, 24}
	inset := r.Inset(2)
	if inset != (Rect{2, 2, 76, 20}) {
		t.Fatalf("Inset(2)=%+v want {2 2 76 20}", inset)
	}
}

// assertRects compares the solved rects map against the expected map.
func assertRects(t *testing.T, got, want map[string]Rect) {
	t.Helper()
	for id, wr := range want {
		gr, ok := got[id]
		if !ok {
			t.Fatalf("missing rect for node %q", id)
		}
		if !reflect.DeepEqual(gr, wr) {
			t.Fatalf("node %q: got %+v want %+v", id, gr, wr)
		}
	}
	// Check no extra rects (unless they're expected).
	if len(got) != len(want) {
		t.Fatalf("rect count: got %d want %d (got=%+v want=%+v)", len(got), len(want), got, want)
	}
}
