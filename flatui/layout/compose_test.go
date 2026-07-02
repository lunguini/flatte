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
