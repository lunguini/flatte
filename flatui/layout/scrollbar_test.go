package layout

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// oldScrollbar is the bar dockerapp drew before ScrollBar existed: the
// reference the node must match.
func oldScrollbar(offset, visible, total, height int) string {
	if total <= visible || height <= 0 || visible <= 0 {
		return strings.Repeat(" ", height)
	}
	maxOffset := total - visible
	thumbSize := min(max(1, height*visible/total), height)
	thumbPos := int(float64(offset) / float64(maxOffset) * float64(height-thumbSize))
	thumbPos = min(max(thumbPos, 0), height-thumbSize)
	rows := make([]string, height)
	for i := range height {
		if i >= thumbPos && i < thumbPos+thumbSize {
			rows[i] = "█"
		} else {
			rows[i] = "░"
		}
	}
	return strings.Join(rows, "\n")
}

func TestScrollBarMatchesTheReference(t *testing.T) {
	for height := 1; height <= 9; height++ {
		for total := 0; total <= 14; total++ {
			for visible := 0; visible <= total+1; visible++ {
				for offset := -1; offset <= total; offset++ {
					bar := ScrollBar{Offset: offset, Visible: visible, Total: total}
					got := ansi.Strip(bar.Render(Rect{W: 1, H: height}))
					if want := splitRows(oldScrollbar(offset, visible, total, height)); got != want {
						t.Fatalf("offset %d visible %d total %d height %d:\n%q\nwant\n%q", offset, visible, total, height, got, want)
					}
				}
			}
		}
	}
}

// splitRows lays the reference out one glyph per line (the blank bar is a
// run of spaces on one line).
func splitRows(s string) string {
	if !strings.Contains(s, "\n") && strings.TrimSpace(s) == "" {
		return strings.TrimSuffix(strings.Repeat(" \n", len(s)), "\n")
	}
	return s
}

func TestScrollBarSizeAndColor(t *testing.T) {
	w, h := ScrollBar{}.Size()
	if w != (Size{Kind: SizeContent, Value: 1}) || h.Kind != SizeGrow {
		t.Fatalf("Size = %+v × %+v, want one column × grow", w, h)
	}
	_, rects := SolveAndCompose(Col{Children: []Node{Row{Children: []Node{
		Label{NodeBase: NodeBase{W: Grow(1)}, Text: "a\nb\nc"},
		ScrollBar{NodeBase: NodeBase{ID: "bar"}, Offset: 1, Visible: 3, Total: 6},
	}}}}, 10, 5)
	if rects["bar"] != (Rect{X: 9, W: 1, H: 3}) {
		t.Fatalf("bar rect %+v, want the row's last column, its height", rects["bar"])
	}
	// A thumb takes its color; a blank bar stays plain.
	red := lipgloss.Color("#ff0000")
	if s := (ScrollBar{Visible: 1, Total: 2, Color: red}).Render(Rect{W: 1, H: 2}); !strings.Contains(s, "\x1b[") {
		t.Fatalf("colored bar is plain: %q", s)
	}
	if s := (ScrollBar{Visible: 2, Total: 2, Color: red}).Render(Rect{W: 1, H: 2}); strings.Contains(s, "\x1b[") {
		t.Fatalf("blank bar is styled: %q", s)
	}
}

func TestSceneScrollBar(t *testing.T) {
	root := Row{Children: []Node{
		ScrollBar{NodeBase: NodeBase{ID: "bar"}, Offset: 10, Visible: 10, Total: 40, Color: lipgloss.Color("#00ff00")},
		ScrollBar{NodeBase: NodeBase{ID: "fits"}, Visible: 10, Total: 10},
		ScrollBar{NodeBase: NodeBase{ID: "over"}, Offset: 99, Visible: 10, Total: 40},
	}}
	s := LayoutScene(root, 10*testMetrics.Column, 5*testMetrics.Line, testMetrics)
	var ops []ScrollBarOp
	for _, op := range s.Ops {
		if b, ok := op.(ScrollBarOp); ok {
			ops = append(ops, b)
		}
	}
	if len(ops) != 3 {
		t.Fatalf("%d scroll bar ops, want 3", len(ops))
	}
	if b := ops[0]; !b.Show || b.Start != 0.25 || b.Length != 0.25 || b.Rect != s.Rects["bar"] || b.Color == nil {
		t.Fatalf("bar op %+v, want a quarter-long thumb a quarter down", b)
	}
	if b := ops[1]; b.Show {
		t.Fatalf("content that fits shows a thumb: %+v", b)
	}
	if b := ops[2]; b.Start != 0.75 {
		t.Fatalf("overscrolled thumb starts at %v, want clamped to 0.75", b.Start)
	}
	if r := s.Rects["bar"]; r.W != testMetrics.Column || r.H != 5*testMetrics.Line {
		t.Fatalf("bar rect %+v, want a column wide and the full height", r)
	}
}
