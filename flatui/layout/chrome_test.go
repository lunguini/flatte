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

// boxChrome is a test chrome painter: a '#' frame filled with '.' rows.
func boxChrome(r Rect) string {
	if r.W < 2 || r.H < 2 {
		return ""
	}
	top := strings.Repeat("#", r.W)
	mid := "#" + strings.Repeat(".", r.W-2) + "#"
	rows := make([]string, r.H)
	rows[0] = top
	for i := 1; i < r.H-1; i++ {
		rows[i] = mid
	}
	rows[r.H-1] = top
	return strings.Join(rows, "\n")
}

// A container with Chrome paints the custom decoration instead of the default
// Pad/Bordered fill; children still land inside the Pad/Bordered inset, on top
// of the chrome.
func TestCustomChromeReplacesDefaultPaint(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Bordered: true, Chrome: boxChrome},
		Children: []Node{Text{String: "hi"}},
	}
	composed, _ := SolveAndCompose(tree, 8, 4)
	// The child Text fills its full inner rect (styled spaces), covering the
	// chrome's '.' fill on its row; the border frame and the un-childed inner
	// row reveal the chrome.
	want := []string{
		"########",
		"#hi    #",
		"#......#",
		"########",
	}
	if got := stripLines(composed); !reflect.DeepEqual(got, want) {
		t.Fatalf("custom chrome:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Chrome is painted even when the node declares no inset (Pad 0, unbordered):
// children then cover it at the full rect, and uncovered cells reveal it.
func TestCustomChromeWithZeroInset(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Chrome: boxChrome},
		Children: []Node{Text{NodeBase: NodeBase{H: Fixed(1)}, String: "hi"}},
	}
	composed, _ := SolveAndCompose(tree, 6, 3)
	got := stripLines(composed)
	// Row 0 is the child — a Text fills its whole rect (styled spaces), so it
	// covers the chrome's top edge; rows 1-2 reveal the chrome underneath.
	want := []string{
		"hi",
		"#....#",
		"######",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zero-inset chrome:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An overlay container with Chrome renders its decoration through the overlay
// pass, and its children get rects.
func TestOverlayContainerWithChrome(t *testing.T) {
	tree := Col{Children: []Node{
		Text{String: strings.Repeat("b\n", 4) + "b"},
		Col{
			NodeBase: NodeBase{Overlay: true, W: Fixed(6), H: Fixed(4), Bordered: true, Chrome: boxChrome},
			Children: []Node{Text{NodeBase: NodeBase{ID: "modal-body"}, String: "hi"}},
		},
	}}
	composed, rects := SolveAndCompose(tree, 12, 6)
	stripped := rawLines(composed)
	found := false
	for _, line := range stripped {
		if strings.Contains(line, "#hi....#"[0:3]) || strings.Contains(line, "#hi") {
			found = true
		}
	}
	if !found {
		t.Fatalf("overlay chrome+child not painted:\n%s", strings.Join(stripped, "\n"))
	}
	body, ok := rects["modal-body"]
	if !ok {
		t.Fatal("overlay child rect not recorded")
	}
	// The 6x4 layer centers at (3,1) in 12x6; its bordered inset starts the
	// inner box at (4,2), and the content-sized Text claims one row of it.
	if body != (Rect{4, 2, 4, 1}) {
		t.Fatalf("overlay child rect = %+v, want {4 2 4 1}", body)
	}
}

// A childless container with Chrome renders the chrome from the string path
// too (Row/Col.Render delegate consistency).
func TestCustomChromeChildlessStringPath(t *testing.T) {
	c := Col{NodeBase: NodeBase{Chrome: boxChrome}}
	want := []string{
		"######",
		"#....#",
		"######",
	}
	if got := stripLines(c.Render(Rect{0, 0, 6, 3})); !reflect.DeepEqual(got, want) {
		t.Fatalf("childless chrome string path:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Per-side padding: PadLeft/PadTop offset content on exactly that side, and
// Size claims exactly the padded axes (d21/d03 extraction trigger, fired by
// flat-docker's list pane and flat-workspace's panes).
func TestPerSidePaddingPositionsContent(t *testing.T) {
	leaf := Text{NodeBase: NodeBase{PadLeft: 2, PadTop: 1}, String: "hi"}

	w, h := leaf.Size()
	if w.Value != 2+2 || h.Value != 1+1 {
		t.Fatalf("Size = %dx%d, want 4x2 (content plus left/top pads)", w.Value, h.Value)
	}

	lines := rawLines(leaf.Render(Rect{0, 0, 6, 3}))
	if strings.TrimRight(lines[0], " ") != "" {
		t.Fatalf("row 0 should be top padding, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  hi") {
		t.Fatalf("row 1 should be \"  hi\", got %q", lines[1])
	}
}

// Per-side padding on a container insets children on exactly the padded
// sides, in both the geometry and the composed output; Pad and Bordered
// still add symmetrically on top of it.
func TestContainerPerSidePaddingInsetsChildren(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{Bordered: true, PadLeft: 1, PadTop: 2},
		Children: []Node{Text{NodeBase: NodeBase{ID: "row"}, String: "x"}},
	}
	composed, rects := SolveAndCompose(tree, 8, 6)

	// border(1)+PadLeft(1) = X 2; border(1)+PadTop(2) = Y 3.
	r, ok := rects["row"]
	if !ok {
		t.Fatal("child rect not recorded")
	}
	if r.X != 2 || r.Y != 3 {
		t.Fatalf("child rect = %+v, want X=2 Y=3", r)
	}
	// right/bottom have only the border inset: inner W = 8-1-2 = 5, H = 6-3-1 = 2.
	if r.W != 5 {
		t.Fatalf("child width = %d, want 5 (right side pads only the border)", r.W)
	}
	lines := stripLines(composed)
	if len(lines) < 4 || !strings.HasPrefix(lines[3], "│ x") {
		t.Fatalf("child glyph should sit at row 3 col 2 inside the border:\n%s",
			strings.Join(lines, "\n"))
	}
}

// The string render path honors per-side padding identically to compose.
func TestPerSidePaddingStringPathMatchesCompose(t *testing.T) {
	tree := Col{
		NodeBase: NodeBase{PadTop: 1, PadLeft: 3},
		Children: []Node{Text{String: "hi"}},
	}
	composed, _ := SolveAndCompose(tree, 8, 3)
	direct := tree.Render(Rect{0, 0, 8, 3})
	got, want := stripLines(direct), stripLines(composed)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("string path:\n%s\ncompose path:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(want) < 2 || !strings.HasPrefix(want[1], "   hi") {
		t.Fatalf("child should sit at row 1 col 3:\n%s", strings.Join(want, "\n"))
	}
}
