package layout

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// rawLines strips ANSI but keeps trailing spaces and blank lines — chrome
// tests assert exact cell positions, so nothing may be trimmed away.
func rawLines(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

// A bordered container composed through SolveAndCompose must keep all four
// border sides — including the bottom row (d19 C1).
func TestComposedBorderKeepsAllSides(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Bordered: true},
		Children: []Node{Text{String: "hi"}},
	}
	composed, _ := SolveAndCompose(tree, 10, 4)
	want := []string{
		"╭────────╮",
		"│hi      │",
		"│        │",
		"╰────────╯",
	}
	if got := stripLines(composed); !reflect.DeepEqual(got, want) {
		t.Fatalf("composed bordered col:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A bordered leaf with empty content (flat-layout's header shape) must render
// a complete box, not a truncated one.
func TestBorderedEmptyTextKeepsAllSides(t *testing.T) {
	leaf := Text{NodeBase: NodeBase{Bordered: true}}
	want := []string{
		"╭────────╮",
		"│        │",
		"│        │",
		"╰────────╯",
	}
	if got := stripLines(leaf.Render(Rect{0, 0, 10, 4})); !reflect.DeepEqual(got, want) {
		t.Fatalf("bordered empty text:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Padding is symmetric: a padded Text draws its content at row/column `pad`,
// matching Size()'s 2*pad claim and the solver's symmetric inset (d19 C2).
func TestPaddedTextContentPosition(t *testing.T) {
	leaf := Text{NodeBase: NodeBase{Pad: 1}, String: "hi"}
	lines := rawLines(leaf.Render(Rect{0, 0, 6, 4}))
	if len(lines) != 4 {
		t.Fatalf("padded text height = %d, want 4:\n%q", len(lines), lines)
	}
	if strings.TrimRight(lines[0], " ") != "" {
		t.Fatalf("row 0 should be top padding, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], " hi") {
		t.Fatalf("row 1 should be \" hi…\", got %q", lines[1])
	}
}

// The string render path and the compositor must place a padded container's
// child on the same row (d19 C2 drift).
func TestPaddedColStringPathMatchesCompose(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Pad: 1},
		Children: []Node{Text{String: "hi"}},
	}
	composed, _ := SolveAndCompose(tree, 8, 4)
	direct := tree.Render(Rect{0, 0, 8, 4})
	got, want := stripLines(direct), stripLines(composed)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("string path:\n%s\ncompose path:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(want) < 2 || !strings.HasPrefix(want[1], " hi") {
		t.Fatalf("child should sit at row 1 col 1, got:\n%s", strings.Join(want, "\n"))
	}
}

// Over-tall and over-wide content is clipped to the inner box; the border
// survives on all sides instead of being truncated away.
func TestOverflowingContentKeepsBorder(t *testing.T) {
	leaf := Text{NodeBase: NodeBase{Bordered: true}, String: "aaaaaaaaaa\nb\nc\nd\ne"}
	want := []string{
		"╭────╮",
		"│aaaa│",
		"│b   │",
		"╰────╯",
	}
	if got := stripLines(leaf.Render(Rect{0, 0, 6, 4})); !reflect.DeepEqual(got, want) {
		t.Fatalf("overflowing bordered text:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
