package layout

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// stripLines strips ANSI and trims each line's trailing spaces, leaving only
// visible glyph placement — the contract goldens actually assert.
func stripLines(s string) []string {
	lines := strings.Split(ansi.Strip(s), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	// drop trailing blank lines for a stable comparison
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// A representative tree: a header row (fixed + grow) over a footer line.
func sampleTree() Node {
	return Col{Children: []Node{
		Row{
			NodeBase: NodeBase{H: Fixed(1)},
			Children: []Node{
				Text{NodeBase: NodeBase{W: Fixed(5)}, String: "abc"},
				Text{NodeBase: NodeBase{W: Grow(1)}, String: "xyz"},
			},
		},
		Text{NodeBase: NodeBase{H: Fixed(1)}, String: "footer"},
	}}
}

// SolveAndCompose places the expected visible glyphs for a representative tree.
func TestSolveAndComposeGlyphs(t *testing.T) {
	composed, _ := SolveAndCompose(sampleTree(), 20, 2)
	want := []string{"abc  xyz", "footer"}
	if got := stripLines(composed); !reflect.DeepEqual(got, want) {
		t.Fatalf("compose glyphs = %q, want %q", got, want)
	}
}

// The id→rect map from a single compose pass must equal a standalone Solve, so
// hit-test geometry and painted geometry are provably the same.
func TestSolveAndComposeRectsMatchSolve(t *testing.T) {
	tree := Col{Children: []Node{
		Row{
			NodeBase: NodeBase{H: Grow(1)},
			Children: []Node{
				Text{NodeBase: NodeBase{ID: "left", W: Fixed(8)}, String: "L"},
				Text{NodeBase: NodeBase{ID: "right", W: Grow(1)}, String: "R"},
			},
		},
		Text{NodeBase: NodeBase{ID: "status", H: Fixed(1)}, String: "s"},
	}}

	_, got := SolveAndCompose(tree, 30, 10)
	want := Solve(tree, 30, 10)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compose rects != Solve rects:\ncompose: %v\nsolve:   %v", got, want)
	}
}

// A fixed child wider than the buffer must be clipped, never overflow the
// frame. (Join-based Render would emit a line wider than the width.)
func TestSolveAndComposeClipsOverflow(t *testing.T) {
	tree := Row{Children: []Node{
		Text{NodeBase: NodeBase{W: Fixed(8)}, String: "AAAAAAAA"},
		Text{NodeBase: NodeBase{W: Fixed(8)}, String: "BBBBBBBB"},
	}}
	composed, _ := SolveAndCompose(tree, 10, 1)
	for _, line := range strings.Split(ansi.Strip(composed), "\n") {
		if w := ansi.StringWidth(line); w > 10 {
			t.Fatalf("composed line width %d exceeds buffer width 10: %q", w, line)
		}
	}
}

// A bordered container paints its own chrome; a child that does not cover the
// border cells leaves them visible (no fill invariant required).
func TestSolveAndComposePaintsContainerChrome(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Bordered: true},
		Children: []Node{
			Text{String: "x"},
		},
	}
	composed, _ := SolveAndCompose(tree, 6, 4)
	stripped := ansi.Strip(composed)
	if !strings.ContainsAny(stripped, "─│╭╮╰╯") {
		t.Fatalf("bordered container did not paint border glyphs:\n%s", stripped)
	}
	if !strings.Contains(stripped, "x") {
		t.Fatalf("bordered container dropped child content:\n%s", stripped)
	}
}

// A child whose distributed rect overflows the parent is clamped to the
// parent's inner box (d19 H2): two Fixed(8) children in a 10-wide row leave the
// second child only the remaining 2 columns.
func TestChildRectClampedToParent(t *testing.T) {
	tree := Row{Children: []Node{
		Text{NodeBase: NodeBase{W: Fixed(8), ID: "a"}, String: "a"},
		Text{NodeBase: NodeBase{W: Fixed(8), ID: "b"}, String: "b"},
	}}
	rects := Solve(tree, 10, 1)
	if a := rects["a"]; a.W != 8 {
		t.Fatalf("a.W=%d want 8", a.W)
	}
	if b := rects["b"]; b.X != 8 || b.W != 2 {
		t.Fatalf("b=%+v want X=8 W=2 (clamped to frame)", b)
	}
}

// No solved rect may escape its parent or the frame, over a handful of
// deliberately overflowing trees (d19 H2).
func TestNoRectEscapesFrame(t *testing.T) {
	const W, H = 12, 6
	trees := []Node{
		Row{Children: []Node{
			Text{NodeBase: NodeBase{W: Fixed(8), ID: "x1"}, String: "x"},
			Text{NodeBase: NodeBase{W: Fixed(8), ID: "x2"}, String: "x"},
			Text{NodeBase: NodeBase{W: Fixed(8), ID: "x3"}, String: "x"},
		}},
		Col{
			NodeBase: NodeBase{Bordered: true},
			Children: []Node{
				Row{NodeBase: NodeBase{ID: "r"}, Children: []Node{
					Text{NodeBase: NodeBase{W: Fixed(50), ID: "wide"}, String: "w"},
				}},
			},
		},
		Col{Children: []Node{
			Text{NodeBase: NodeBase{H: Fixed(20), ID: "tall"}, String: "t"},
			Text{NodeBase: NodeBase{H: Fixed(20), ID: "tall2"}, String: "t"},
		}},
	}
	for ti, tr := range trees {
		for id, r := range Solve(tr, W, H) {
			if r.X < 0 || r.Y < 0 || r.X+r.W > W || r.Y+r.H > H {
				t.Fatalf("tree %d: rect %q = %+v escapes frame %dx%d", ti, id, r, W, H)
			}
		}
	}
}

// Row.Render delegates to the single walk, so its output for a nested tree is
// byte-identical to SolveAndCompose (d19 H4).
func TestRowRenderEqualsSolveAndCompose(t *testing.T) {
	tree := Row{
		NodeBase: NodeBase{Gap: 1},
		Children: []Node{
			Col{Children: []Node{Text{String: "a"}, Text{String: "bb"}}},
			NewSpacer(),
			Text{NodeBase: NodeBase{W: Fixed(4)}, String: "R"},
		},
	}
	direct := tree.Render(Rect{0, 0, 20, 3})
	composed, _ := SolveAndCompose(tree, 20, 3)
	if direct != composed {
		t.Fatalf("Row.Render != SolveAndCompose:\n%q\nvs\n%q", direct, composed)
	}
}

// A standalone Row.Render containing an overlay child must paint the overlay
// exactly once — the overlay pass must not run twice through delegation (d19 H4).
func TestRowRenderOverlayPaintedOnce(t *testing.T) {
	tree := Row{Children: []Node{
		Text{String: "base"},
		Text{NodeBase: NodeBase{Overlay: true, W: Fixed(3), H: Fixed(1)}, String: "OVL"},
	}}
	out := ansi.Strip(tree.Render(Rect{0, 0, 12, 3}))
	if n := strings.Count(out, "OVL"); n != 1 {
		t.Fatalf("overlay painted %d times, want 1:\n%s", n, out)
	}
}

// An ID'd child inside an overlay container is assigned a rect by both Solve
// and SolveAndCompose, and the two agree (d19 H3).
func TestOverlayChildRectsRecorded(t *testing.T) {
	tree := Col{Children: []Node{
		Text{String: "base"},
		Col{
			NodeBase: NodeBase{Overlay: true, W: Fixed(10), H: Fixed(4), ID: "modal"},
			Children: []Node{
				Text{NodeBase: NodeBase{ID: "title", H: Fixed(1)}, String: "T"},
				Text{NodeBase: NodeBase{ID: "content", H: Grow(1)}, String: "C"},
			},
		},
	}}
	solveRects := Solve(tree, 30, 12)
	_, composeRects := SolveAndCompose(tree, 30, 12)
	for _, id := range []string{"modal", "title", "content"} {
		sr, ok := solveRects[id]
		if !ok {
			t.Fatalf("Solve missing rect for %q", id)
		}
		if sr != composeRects[id] {
			t.Fatalf("rect mismatch for %q: solve=%+v compose=%+v", id, sr, composeRects[id])
		}
	}
	// modal centered: (30-10)/2=10, (12-4)/2=4
	if modal := solveRects["modal"]; modal != (Rect{10, 4, 10, 4}) {
		t.Fatalf("modal rect = %+v, want {10,4,10,4}", modal)
	}
	// title sits at the top-left of the modal interior.
	if title := solveRects["title"]; title.X != 10 || title.Y != 4 {
		t.Fatalf("title rect = %+v, want origin at modal top-left", title)
	}
}

// A bordered overlay container composed through the tree walk paints all four
// border sides (d19 H3): it must go through the compositor, not the legacy
// string path.
func TestOverlayBorderedPaintsAllSides(t *testing.T) {
	tree := Col{Children: []Node{
		Text{String: "base base base base"},
		Col{
			NodeBase: NodeBase{Overlay: true, Bordered: true, W: Fixed(8), H: Fixed(4)},
			Children: []Node{Text{String: "hi"}},
		},
	}}
	composed, _ := SolveAndCompose(tree, 20, 8)
	stripped := ansi.Strip(composed)
	for _, corner := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(stripped, corner) {
			t.Fatalf("overlay border missing corner %q:\n%s", corner, stripped)
		}
	}
}

// An overlay sized from measurable content (SizeContent) is content-sized, not
// stretched to the full viewport (d19 H3).
func TestContentSizedOverlayNotFullScreen(t *testing.T) {
	tree := Col{Children: []Node{
		Text{String: "base"},
		Col{
			NodeBase: NodeBase{Overlay: true, ID: "modal"},
			Children: []Node{Text{String: "hello"}},
		},
	}}
	rects := Solve(tree, 40, 20)
	modal := rects["modal"]
	if modal.W >= 40 || modal.H >= 20 {
		t.Fatalf("content-sized overlay filled the screen: %+v", modal)
	}
	if modal.W != 5 || modal.H != 1 {
		t.Fatalf("modal rect = %+v, want 5x1 (content of \"hello\")", modal)
	}
}

// The cells under an overlay's rect that the overlay itself does not paint are
// cleared, so the base does not bleed through (d19 H3, parity with the string
// compositor's FillArea).
func TestOverlayClearsBaseBleedThrough(t *testing.T) {
	baseStr := strings.TrimRight(strings.Repeat("XXXXXXXXXX\n", 6), "\n")
	tree := Col{Children: []Node{
		Text{NodeBase: NodeBase{H: Grow(1)}, String: baseStr},
		Col{
			NodeBase: NodeBase{Overlay: true, W: Fixed(6), H: Fixed(3)},
			Children: []Node{Text{String: "hi"}},
		},
	}}
	composed, _ := SolveAndCompose(tree, 10, 6)
	lines := strings.Split(ansi.Strip(composed), "\n")
	// Overlay centered at x=2,y=1,w=6,h=3. The child paints row 1 only; rows
	// 2 and 3 (columns 2..7) must be cleared, not filled with base 'X'.
	if got := lines[2][2:8]; strings.Contains(got, "X") {
		t.Fatalf("base bled through cleared overlay region: row2=%q seg=%q", lines[2], got)
	}
}

// Overlay children are painted centered on top and their rect recorded.
func TestSolveAndComposePaintsOverlay(t *testing.T) {
	tree := Col{Children: []Node{
		Text{String: "base"},
		Text{NodeBase: NodeBase{ID: "modal", W: Fixed(4), H: Fixed(1), Overlay: true}, String: "MODAL"},
	}}
	composed, rects := SolveAndCompose(tree, 20, 5)
	if !strings.Contains(ansi.Strip(composed), "MODA") {
		t.Fatalf("overlay not painted:\n%s", ansi.Strip(composed))
	}
	mr, ok := rects["modal"]
	if !ok {
		t.Fatal("overlay rect not recorded")
	}
	if mr.W != 4 || mr.H != 1 {
		t.Fatalf("overlay rect = %+v, want 4x1 centered", mr)
	}
}
