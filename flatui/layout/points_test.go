package layout

import (
	"math"
	"math/rand/v2"
	"testing"
	"unicode/utf8"
)

// scaledMetric is a GUI-style metric scaled by k: 7.5pt columns, 17pt
// lines, 8pt steps, a 1pt border, and Labels measured proportionally (0.8
// of a column per rune), not as cells.
func scaledMetric(k float64) pointMetric {
	col := 7.5 * k
	return pointMetric{Metrics{
		Column: col, Line: 17 * k, Step: 8 * k, Border: k,
		MeasureText: func(text string, _ TextStyle) (float64, float64) {
			return 0.8 * col * float64(utf8.RuneCountInString(text)), 17 * k
		},
	}}
}

var pt = scaledMetric(1)

func solvePoints(root Node, w, h float64) map[string]rect[float64] {
	return solveTree[float64](pt, root, w, h, nil).rects
}

func TestPointsFixedColumnsAndSteps(t *testing.T) {
	root := Row{NodeBase: NodeBase{Pad: 1, Gap: 2}, Children: []Node{
		Text{NodeBase: NodeBase{ID: "side", W: Fixed(10)}, String: "x"},
		Text{NodeBase: NodeBase{ID: "main", W: Grow(1)}, String: "y"},
	}}
	r := solvePoints(root, 400, 200)
	// Pad is one 8pt step; Fixed(10) is ten 7.5pt columns; Gap two steps.
	if got := r["side"]; got != (rect[float64]{8, 8, 75, 184}) {
		t.Errorf("side = %+v", got)
	}
	if got := r["main"]; got != (rect[float64]{8 + 75 + 16, 8, 400 - 16 - 75 - 16, 184}) {
		t.Errorf("main = %+v", got)
	}
}

func TestPointsGrowSharesAreExact(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "a", H: Grow(1)}},
		Text{NodeBase: NodeBase{ID: "b", H: Grow(2)}},
		Text{NodeBase: NodeBase{ID: "c", H: Grow(4)}},
	}}
	r := solvePoints(root, 100, 100)
	// Cells would floor and hand out a remainder; points split exactly.
	want := []float64{100.0 / 7, 200.0 / 7, 400.0 / 7}
	y := 0.0
	for i, id := range []string{"a", "b", "c"} {
		got := r[id]
		if math.Abs(got.H-want[i]) > 1e-9 || math.Abs(got.Y-y) > 1e-9 {
			t.Errorf("%s = %+v, want y %.4f h %.4f", id, got, y, want[i])
		}
		y += want[i]
	}
}

func TestPointsBorderAndMeasuredLabel(t *testing.T) {
	root := Row{Children: []Node{
		Col{NodeBase: NodeBase{ID: "card", Bordered: true}, Children: []Node{
			Label{NodeBase: NodeBase{ID: "l"}, Text: "hello"},
		}},
		NewSpacer(),
	}}
	r := solvePoints(root, 300, 100)
	// The card's width is the label's measured width (0.8×5 columns) plus
	// two 1pt borders — not a whole number of cells.
	if got := r["card"].W; math.Abs(got-(0.8*7.5*5+2)) > 1e-9 {
		t.Errorf("card width = %v", got)
	}
	if got := r["l"]; got.X != 1 || got.Y != 1 {
		t.Errorf("label origin = %+v, want inside the 1pt border", got)
	}
}

func TestPointsScrollOffsetIsInLines(t *testing.T) {
	root := Scroll{NodeBase: NodeBase{ID: "s"}, OffsetY: 2, Child: Col{NodeBase: NodeBase{ID: "content"}, Children: []Node{
		Text{NodeBase: NodeBase{H: Fixed(10)}},
	}}}
	r := solvePoints(root, 100, 51)
	if got := r["content"]; got.Y != -34 || got.H != 170 {
		t.Errorf("content = %+v, want y -34 (two 17pt lines), h 170", got)
	}
	// Past the end clamps to content - window, in points.
	root.OffsetY = 100
	if got := solvePoints(root, 100, 51)["content"].Y; got != -(170 - 51) {
		t.Errorf("clamped content y = %v", got)
	}
}

func TestPointsOverlayCentredFractionally(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "modal", Overlay: true, W: Fixed(5), H: Fixed(1)}},
	}}
	if got := solvePoints(root, 100, 50)["modal"]; got != (rect[float64]{31.25, 16.5, 37.5, 17}) {
		t.Errorf("modal = %+v", got)
	}
}

// TestPointsCorpusInvariants: every generated tree solves to finite,
// non-negative geometry whose hit areas stay inside the frame.
func TestPointsCorpusInvariants(t *testing.T) {
	for i := range corpusTrees {
		rng := rand.New(rand.NewPCG(uint64(i), 0x5eed))
		root := (&treeGen{rng: rng}).node(0)
		w, h := 1+rng.Float64()*500, 1+rng.Float64()*300
		s := solveTree[float64](pt, root, w, h, nil)
		for id, r := range s.rects {
			for _, v := range []float64{r.X, r.Y, r.W, r.H} {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("tree %d: %s = %+v is not finite", i, id, r)
				}
			}
			if r.W < 0 || r.H < 0 {
				t.Fatalf("tree %d: %s = %+v has a negative size", i, id, r)
			}
		}
		for _, hit := range s.hits {
			a := hit.area
			if a.W > 0 && a.H > 0 && (a.X < 0 || a.Y < 0 || a.X+a.W > w+1e-9 || a.Y+a.H > h+1e-9) {
				t.Fatalf("tree %d: hit %q area %+v escapes the %vx%v frame", i, hit.id, a, w, h)
			}
		}
	}
}

// TestPointsHomogeneous: scaling every metric and the frame by k scales
// every rect by k. A hidden integer assumption (flooring, a +1, a cell
// constant) breaks this.
func TestPointsHomogeneous(t *testing.T) {
	const k = 2.5
	scaled := scaledMetric(k)
	for i := range corpusTrees {
		rng := rand.New(rand.NewPCG(uint64(i), 0x5eed))
		root := (&treeGen{rng: rng}).node(0)
		w, h := 1+rng.Float64()*500, 1+rng.Float64()*300
		base := solveTree[float64](pt, root, w, h, nil).rects
		big := solveTree[float64](scaled, root, w*k, h*k, nil).rects
		for id, r := range base {
			b := big[id]
			for j, pair := range [][2]float64{{r.X, b.X}, {r.Y, b.Y}, {r.W, b.W}, {r.H, b.H}} {
				if d := math.Abs(pair[0]*k - pair[1]); d > 1e-6*max(1, math.Abs(pair[1])) {
					t.Fatalf("tree %d: %s field %d: %v×%v = %v, scaled solve %v", i, id, j, pair[0], k, pair[0]*k, pair[1])
				}
			}
		}
	}
}
