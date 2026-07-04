package layout

import (
	"strings"
	"testing"
)

func TestColFixedGrowFixed(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "header", NodeBase: NodeBase{H: Fixed(3)}},
			Text{String: "body", NodeBase: NodeBase{H: Grow(1)}},
			Text{String: "footer", NodeBase: NodeBase{H: Fixed(1)}},
		},
	}
	// Should compile and solve without error
	rects := Solve(tree, 80, 24)
	// No IDs set, so rects should be empty
	if len(rects) != 0 {
		t.Fatalf("expected 0 rects, got %d", len(rects))
	}
	// Should render
	result, _ := SolveAndCompose(tree, 80, 24)
	if result == "" {
		t.Fatal("empty render")
	}
}

func TestRowFixedGrow(t *testing.T) {
	tree := Row{
		Children: []Node{
			Text{String: "sidebar", NodeBase: NodeBase{W: Fixed(20)}},
			Text{String: "content", NodeBase: NodeBase{W: Grow(1)}},
		},
	}
	result, _ := SolveAndCompose(tree, 80, 10)
	lines := strings.Split(result, "\n")
	if len(lines) == 0 {
		t.Fatal("empty output")
	}
	if !strings.Contains(lines[0], "sidebar") {
		t.Fatalf("missing sidebar: %q", lines[0])
	}
}

func TestGrowWeightProportional(t *testing.T) {
	tree := Row{
		Children: []Node{
			Text{String: "a", NodeBase: NodeBase{W: Grow(1), ID: "a"}},
			Text{String: "b", NodeBase: NodeBase{W: Grow(2), ID: "b"}},
		},
	}
	rects := Solve(tree, 90, 10)
	a := rects["a"]
	b := rects["b"]
	if a.W != 30 {
		t.Fatalf("a.W=%d want 30 (1/3 of 90)", a.W)
	}
	if b.W != 60 {
		t.Fatalf("b.W=%d want 60 (2/3 of 90)", b.W)
	}
}

func TestDeterministicRemainder(t *testing.T) {
	tree := Row{
		Children: []Node{
			Text{String: "a", NodeBase: NodeBase{W: Grow(1), ID: "a"}},
			Text{String: "b", NodeBase: NodeBase{W: Grow(1), ID: "b"}},
			Text{String: "c", NodeBase: NodeBase{W: Grow(1), ID: "c"}},
		},
	}
	rects := Solve(tree, 80, 10)
	// 80 / 3 = 26r2 → first two get 27, last gets 26
	a := rects["a"]
	c := rects["c"]
	if a.W != 27 {
		t.Fatalf("a.W=%d want 27", a.W)
	}
	if c.W != 26 {
		t.Fatalf("c.W=%d want 26", c.W)
	}
}

// Grow children whose weights sum to zero (Grow(0)) share space equally rather
// than collapsing (d19 M2).
func TestGrowZeroEqualShares(t *testing.T) {
	tree := Row{Children: []Node{
		Text{String: "a", NodeBase: NodeBase{W: Grow(0), ID: "a"}},
		Text{String: "b", NodeBase: NodeBase{W: Grow(0), ID: "b"}},
	}}
	rects := Solve(tree, 10, 1)
	if a := rects["a"]; a.W != 5 {
		t.Fatalf("a.W=%d want 5 (equal share of 10)", a.W)
	}
	if b := rects["b"]; b.W != 5 {
		t.Fatalf("b.W=%d want 5 (equal share of 10)", b.W)
	}
}

// A zero-value Size{Kind: SizeGrow} behaves like Grow(0) — equal share, not a
// dropped child (d19 M2).
func TestZeroValueGrowShares(t *testing.T) {
	tree := Row{Children: []Node{
		Text{String: "a", NodeBase: NodeBase{W: Size{Kind: SizeGrow}, ID: "a"}},
		Text{String: "b", NodeBase: NodeBase{W: Size{Kind: SizeGrow}, ID: "b"}},
	}}
	rects := Solve(tree, 8, 1)
	if a, b := rects["a"], rects["b"]; a.W != 4 || b.W != 4 {
		t.Fatalf("a.W=%d b.W=%d want 4 and 4", a.W, b.W)
	}
}

// A Row whose only child is a Spacer (grow on both axes) must not collapse: it
// should report Grow(1) and receive nonzero height inside a Col (d19 M3).
func TestRowWithOnlySpacerGetsHeight(t *testing.T) {
	tree := Col{Children: []Node{
		Row{NodeBase: NodeBase{ID: "row"}, Children: []Node{NewSpacer()}},
		Text{NodeBase: NodeBase{ID: "foot", H: Fixed(1)}, String: "f"},
	}}
	rects := Solve(tree, 20, 10)
	if row := rects["row"]; row.H != 9 {
		t.Fatalf("row.H=%d want 9 (Row with only a Spacer must fill, not collapse)", row.H)
	}
}

func TestTextAutoSizes(t *testing.T) {
	tx := Text{String: "hello"}
	w, h := tx.Size()
	if w.Kind != SizeContent || w.Value < 5 {
		t.Fatalf("w=%+v want SizeContent(>=5)", w)
	}
	if h.Kind != SizeContent || h.Value != 1 {
		t.Fatalf("h=%+v want SizeContent(1)", h)
	}
}

func TestTextExplicitSize(t *testing.T) {
	tx := Text{String: "hello", NodeBase: NodeBase{W: Fixed(20), H: Fixed(3)}}
	w, h := tx.Size()
	if w.Kind != SizeFixed || w.Value != 20 {
		t.Fatalf("w=%+v want Fixed(20)", w)
	}
	if h.Kind != SizeFixed || h.Value != 3 {
		t.Fatalf("h=%+v want Fixed(3)", h)
	}
}

func TestSpacerFillsRemaining(t *testing.T) {
	tree := Row{
		Children: []Node{
			Text{String: "left", NodeBase: NodeBase{W: Fixed(10), ID: "left"}},
			NewSpacer(),
			Text{String: "right", NodeBase: NodeBase{W: Fixed(10), ID: "right"}},
		},
	}
	rects := Solve(tree, 80, 1)
	if rects["right"].X != 70 {
		t.Fatalf("right.X=%d want 70", rects["right"].X)
	}
}

func TestRowAutoHeight(t *testing.T) {
	r := Row{
		Children: []Node{
			Text{String: "x"},
			Text{String: "y\nz"},
		},
	}
	_, h := r.Size()
	if h.Kind != SizeContent || h.Value != 2 {
		t.Fatalf("Row auto height=%+v want SizeContent(2)", h)
	}
}

func TestColAutoHeight(t *testing.T) {
	c := Col{
		Children: []Node{
			Text{String: "a"},
			Text{String: "b"},
		},
	}
	_, h := c.Size()
	if h.Kind != SizeContent || h.Value != 2 {
		t.Fatalf("Col auto height=%+v want SizeContent(2)", h)
	}
}

func TestNestedRender(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "HEADER"},
			Row{
				Children: []Node{
					Text{String: "L"},
					NewSpacer(),
					Text{String: "R"},
				},
			},
			Text{String: "FOOTER"},
		},
	}
	result, _ := SolveAndCompose(tree, 20, 3)
	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Fatalf("got %d lines want >=3", len(lines))
	}
	if !strings.Contains(lines[0], "HEADER") {
		t.Fatalf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[1], "L") || !strings.Contains(lines[1], "R") {
		t.Fatalf("line 1 missing L/R: %q", lines[1])
	}
	if !strings.Contains(lines[2], "FOOTER") {
		t.Fatalf("line 2: %q", lines[2])
	}
	// L should be left, R right
	if strings.Index(lines[1], "L") > 0 {
		t.Fatalf("L not at left: %q", lines[1])
	}
}

func TestOverlayExcludedFromLayout(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "body", NodeBase: NodeBase{H: Grow(1), ID: "body"}},
			Text{String: "modal", NodeBase: NodeBase{
				W: Fixed(10), H: Fixed(5), Overlay: true, ID: "modal",
			}},
		},
	}
	rects := Solve(tree, 80, 24)
	// Body should get full height — modal doesn't claim flex space
	body := rects["body"]
	if body.H != 24 {
		t.Fatalf("body.H=%d want 24 (overlay excluded)", body.H)
	}
	// Modal centered: x=(80-10)/2=35, y=(24-5)/2=9
	modal := rects["modal"]
	if modal.X != 35 || modal.Y != 9 {
		t.Fatalf("modal=%+v want centered", modal)
	}
}

func TestRenderWithGrowBody(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "H", NodeBase: NodeBase{H: Fixed(1)}},
			Text{String: "B", NodeBase: NodeBase{H: Grow(1)}},
			Text{String: "F", NodeBase: NodeBase{H: Fixed(1)}},
		},
	}
	result, _ := SolveAndCompose(tree, 10, 5)
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines want 5", len(lines))
	}
	if !strings.Contains(lines[0], "H") {
		t.Fatalf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[4], "F") {
		t.Fatalf("line 4: %q", lines[4])
	}
}

func TestCrossAxisStretch(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "full", NodeBase: NodeBase{H: Fixed(1), ID: "full"}},
			Text{String: "narrow", NodeBase: NodeBase{H: Fixed(1), W: Fixed(30), ID: "narrow"}},
		},
	}
	rects := Solve(tree, 80, 5)
	full := rects["full"]
	narrow := rects["narrow"]
	if full.W != 80 {
		t.Fatalf("full.W=%d want 80 (stretch)", full.W)
	}
	if narrow.W != 30 {
		t.Fatalf("narrow.W=%d want 30 (fixed)", narrow.W)
	}
}

func TestBorderInset(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Bordered: true, ID: "root"},
		Children: []Node{
			Text{String: "child", NodeBase: NodeBase{H: Grow(1), ID: "child"}},
		},
	}
	rects := Solve(tree, 80, 24)
	child := rects["child"]
	if child.X != 1 || child.Y != 1 {
		t.Fatalf("border inset: child at (%d,%d) want (1,1)", child.X, child.Y)
	}
	if child.W != 78 || child.H != 22 {
		t.Fatalf("border inset: child W=%d H=%d want 78,22", child.W, child.H)
	}
}

func TestPaddingInset(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Pad: 2, ID: "root"},
		Children: []Node{
			Text{String: "child", NodeBase: NodeBase{H: Grow(1), ID: "child"}},
		},
	}
	rects := Solve(tree, 80, 24)
	child := rects["child"]
	if child.X != 2 || child.Y != 2 {
		t.Fatalf("padding inset: child at (%d,%d) want (2,2)", child.X, child.Y)
	}
}

func TestSpacingBetweenChildren(t *testing.T) {
	tree := Row{
		NodeBase: NodeBase{Gap: 2},
		Children: []Node{
			Text{String: "a", NodeBase: NodeBase{W: Fixed(20), ID: "a"}},
			Text{String: "b", NodeBase: NodeBase{W: Grow(1), ID: "b"}},
			Text{String: "c", NodeBase: NodeBase{W: Fixed(20), ID: "c"}},
		},
	}
	rects := Solve(tree, 80, 10)
	a := rects["a"]
	b := rects["b"]
	c := rects["c"]
	if b.X != a.W+2 {
		t.Fatalf("b.X=%d want %d (after gap)", b.X, a.W+2)
	}
	if c.X != b.X+b.W+2 {
		t.Fatalf("c.X=%d want %d (after gap)", c.X, b.X+b.W+2)
	}
}

// Widget node for testing the embed-NodeBase pattern.
type testWidget struct {
	NodeBase
	content string
}

func (w testWidget) Size() (Size, Size) {
	w2, h := w.NodeBase.Size()
	if w2.Kind == SizeAuto || h.Kind == SizeAuto {
		mw, mh := measureString(w.content)
		inset := w.insetSides()
		if w2.Kind == SizeAuto {
			w2 = Size{Kind: SizeContent, Value: mw + inset.horiz()}
		}
		if h.Kind == SizeAuto {
			h = Size{Kind: SizeContent, Value: mh + inset.vert()}
		}
	}
	return w2, h
}

func (w testWidget) Render(r Rect) string {
	return fillRect(w.content, r, w.padSides(), w.Bordered)
}

func TestCustomWidgetInTree(t *testing.T) {
	tree := Col{
		Children: []Node{
			testWidget{content: "widget!", NodeBase: NodeBase{H: Fixed(1)}},
			testWidget{content: "grows", NodeBase: NodeBase{H: Grow(1)}},
		},
	}
	result, _ := SolveAndCompose(tree, 20, 3)
	if !strings.Contains(result, "widget!") {
		t.Fatalf("missing widget!: %q", result)
	}
	if !strings.Contains(result, "grows") {
		t.Fatalf("missing grows: %q", result)
	}
}

func TestCustomWidgetAutoSizing(t *testing.T) {
	w := testWidget{content: "hello"}
	sw, sh := w.Size()
	if sw.Kind != SizeContent || sw.Value < 5 {
		t.Fatalf("widget W=%+v want SizeContent(>=5)", sw)
	}
	if sh.Kind != SizeContent || sh.Value != 1 {
		t.Fatalf("widget H=%+v want SizeContent(1)", sh)
	}
}

func TestWidgetInRowWithSpacer(t *testing.T) {
	tree := Row{
		Children: []Node{
			testWidget{content: "LEFT", NodeBase: NodeBase{ID: "left"}},
			NewSpacer(),
			testWidget{content: "RIGHT", NodeBase: NodeBase{ID: "right"}},
		},
	}
	result, _ := SolveAndCompose(tree, 30, 1)
	lines := strings.Split(result, "\n")
	line := lines[0]
	if !strings.Contains(line, "LEFT") || !strings.Contains(line, "RIGHT") {
		t.Fatalf("missing parts: %q", line)
	}
	if strings.Index(line, "RIGHT") < 20 {
		t.Fatalf("RIGHT not right-aligned: %q", line)
	}
}

func TestRectContains(t *testing.T) {
	r := Rect{5, 5, 10, 10}
	if !r.Contains(5, 5) {
		t.Fatal("should contain top-left")
	}
	if r.Contains(15, 5) {
		t.Fatal("should not contain right edge")
	}
}

func TestRectInset(t *testing.T) {
	r := Rect{0, 0, 80, 24}.Inset(2)
	if r != (Rect{2, 2, 76, 20}) {
		t.Fatalf("inset=%+v", r)
	}
}

func TestRenderProducesCorrectHeight(t *testing.T) {
	tree := Col{
		Children: []Node{
			Text{String: "a", NodeBase: NodeBase{H: Fixed(3)}},
			Text{String: "b", NodeBase: NodeBase{H: Grow(1)}},
			Text{String: "c", NodeBase: NodeBase{H: Fixed(1)}},
		},
	}
	result, _ := SolveAndCompose(tree, 10, 10)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("got %d lines want 10", len(lines))
	}
}
