package flatui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestViewportLayoutRendersVisibleSlice(t *testing.T) {
	var v Viewport
	v.SetSize(20, 2)
	v.SetContent("a\nb\nc")
	out, _ := layout.SolveAndCompose(v.Layout(), 20, 2)
	if !strings.Contains(out, "a") || strings.Contains(out, "c") {
		t.Fatalf("viewport layout window wrong:\n%s", out)
	}
}

// TestViewportAdoptsTheHostsLineCount: in a host that wraps the content
// into more lines than the viewport's own (a window's font), an adopted
// viewport scrolls over the host's lines — to the true end — and reports
// them; in the terminal, adopting changes nothing.
func TestViewportAdoptsTheHostsLineCount(t *testing.T) {
	// One point per column, but text twice as wide: every line wraps into
	// about twice as many.
	m := layout.Metrics{Column: 1, Line: 1, Step: 1, Border: 1}
	m.MeasureText = func(text string, _ layout.TextStyle, maxWidth float64) (float64, float64) {
		lines := 0
		for _, para := range strings.Split(text, "\n") {
			w := 2 * float64(len(para))
			if maxWidth > 0 {
				lines += max(int((w+maxWidth-1)/maxWidth), 1)
			} else {
				lines++
			}
		}
		return min(maxWidth, 2*float64(len(text))), float64(lines)
	}
	content := strings.Repeat("abcdefghij ", 12) // 132 runes: 14 lines at width 10
	var v Viewport
	v.SetWrappedContent(content)
	view := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, Children: []layout.Node{v.Layout()}},
		}}
	}
	settle := func() {
		for range 3 {
			s := layout.LayoutScene(view(), 10, 4, m)
			rects := map[string]layout.Rect{}
			for id, r := range s.Rects {
				rects[id] = r.Cells(m)
			}
			v.Adopt(rects, "body")
		}
	}
	settle()
	own := len(v.lines)
	if v.TotalLines() <= own {
		t.Fatalf("adopted %d lines, want the host's, more than the viewport's %d", v.TotalLines(), own)
	}
	v.GotoBottom()
	if v.Offset() != v.TotalLines()-4 || !v.AtBottom() || v.ScrollPercent() != 1 {
		t.Fatalf("bottom offset %d of %d lines", v.Offset(), v.TotalLines())
	}
	if v.VisibleLines() != 4 {
		t.Fatalf("visible lines at the bottom = %d, want the full window", v.VisibleLines())
	}
	// At the bottom it stays there when the host relays it out.
	settle()
	if !v.AtBottom() {
		t.Fatal("relayout left the bottom")
	}

	// The terminal: the content rect is the viewport's own lines.
	var tv Viewport
	tv.SetWrappedContent(content)
	tview := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, Children: []layout.Node{tv.Layout()}},
		}}
	}
	for range 3 {
		_, rects := layout.SolveAndCompose(tview(), 10, 4)
		tv.Adopt(rects, "body")
	}
	if tv.TotalLines() != len(tv.lines) {
		t.Fatalf("terminal adopted %d lines, want its own %d", tv.TotalLines(), len(tv.lines))
	}
}

func TestViewportFirstAdoptStartsAtTheTop(t *testing.T) {
	var v Viewport
	v.SetWrappedContent(strings.Repeat("word ", 50)) // wrapped on first size
	_, rects := layout.SolveAndCompose(layout.Col{Children: []layout.Node{
		layout.Col{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, Children: []layout.Node{v.Layout()}},
	}}, 10, 3)
	v.Adopt(rects, "body")
	if v.Offset() != 0 || v.TotalLines() <= 3 {
		t.Fatalf("first adopt: offset %d of %d lines, want the top of an overflowing view", v.Offset(), v.TotalLines())
	}
}

// TestViewportAdoptKeepsAPinnedBottom: content that grows between layouts
// (a log tail) stays at the bottom once sent there, even though the
// adopted line count is stale until the next layout; scrolling up unpins.
func TestViewportAdoptKeepsAPinnedBottom(t *testing.T) {
	var v Viewport
	view := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, Children: []layout.Node{v.Layout()}},
		}}
	}
	adopt := func() {
		for range 3 {
			_, rects := layout.SolveAndCompose(view(), 20, 3)
			v.Adopt(rects, "body")
		}
	}
	lines := []string{"1", "2"}
	v.SetLines(lines)
	adopt()
	for i := 3; i <= 12; i++ { // ten lines arrive before one layout
		lines = append(lines, strconv.Itoa(i))
		v.SetLines(lines)
		v.GotoBottom()
	}
	adopt()
	if v.Offset() != 9 || !v.AtBottom() {
		t.Fatalf("after growth: offset %d of %d, want the bottom (9)", v.Offset(), v.TotalLines())
	}
	v.LineUp(2)
	v.SetLines(append(lines, "13", "14"))
	adopt()
	if v.Offset() != 7 {
		t.Fatalf("scrolled up then grown: offset %d, want it kept at 7", v.Offset())
	}
	v.LineDown(99) // scrolled back to the end: pinned again
	v.SetLines(append(lines, "13", "14", "15"))
	adopt()
	if !v.AtBottom() {
		t.Fatalf("scrolled to the end then grown: offset %d of %d, want the bottom", v.Offset(), v.TotalLines())
	}
}

func TestViewportNotLaidOutUsesItsOwnLines(t *testing.T) {
	var v Viewport
	v.SetSize(10, 2)
	v.SetLines([]string{"a", "b", "c", "d", "e"})
	v.Adopt(map[string]layout.Rect{"body": {W: 10, H: 2}, "body:content": {W: 10, H: 9}}, "body")
	if v.TotalLines() != 9 {
		t.Fatalf("adopted %d lines, want the host's 9", v.TotalLines())
	}
	v.Adopt(map[string]layout.Rect{}, "body") // hidden: no rect
	if v.TotalLines() != 5 {
		t.Fatalf("hidden viewport reports %d lines, want its own 5", v.TotalLines())
	}
}
