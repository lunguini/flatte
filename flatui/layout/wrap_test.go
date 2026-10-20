package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// frameLines is a composed frame as plain lines without trailing blanks.
func frameLines(root Node, w, h int) []string {
	lines := strings.Split(ansi.Strip(Compose(root, w, h).Frame), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return lines
}

func eqLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	for len(got) > len(want) && got[len(got)-1] == "" {
		got = got[:len(got)-1]
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestLabelWrapsByWordsAtItsWidth(t *testing.T) {
	root := Col{Children: []Node{
		Label{Text: "the quick brown fox jumps", Wrap: true},
		Label{Text: "after"},
	}}
	eqLines(t, frameLines(root, 10, 6), "the quick", "brown fox", "jumps", "after")
	// The wrapped label's rect is its wrapped height.
	_, rects := SolveAndCompose(Col{Children: []Node{Label{NodeBase: NodeBase{ID: "p"}, Text: "the quick brown fox jumps", Wrap: true}}}, 10, 6)
	if rects["p"] != (Rect{W: 10, H: 3}) {
		t.Fatalf("wrapped label rect = %+v, want 10x3", rects["p"])
	}
}

func TestLabelWrapBreaksLongWordsAndKeepsNewlines(t *testing.T) {
	root := Col{Children: []Node{Label{Text: "abcdefghijkl\nhi there", Wrap: true}}}
	eqLines(t, frameLines(root, 5, 6), "abcde", "fghij", "kl", "hi", "there")
}

func TestLabelWrapInsidePaddingAndRows(t *testing.T) {
	// Padding narrows the wrap width; in a Row the label gets the width the
	// Row leaves it, and the Row is as tall as the wrapped text.
	root := Col{Children: []Node{
		Row{Children: []Node{
			Label{NodeBase: NodeBase{W: Fixed(3)}, Text: "->"},
			Label{NodeBase: NodeBase{PadLeft: 1}, Text: "one two three four", Wrap: true},
		}},
		Label{Text: "end"},
	}}
	eqLines(t, frameLines(root, 13, 8), "->  one two", "    three", "    four", "end")
}

func TestScrollContentIsTheWrappedHeight(t *testing.T) {
	text := "aa bb cc dd ee ff" // 6 lines at width 2
	sc := Scroll{NodeBase: NodeBase{H: Fixed(2)}, OffsetY: 99, Child: Label{NodeBase: NodeBase{ID: "c"}, Text: text, Wrap: true}}
	_, rects := SolveAndCompose(Col{Children: []Node{sc}}, 2, 4)
	// Clamped to the last window: content 6 lines, window 2.
	if rects["c"] != (Rect{Y: -4, W: 2, H: 6}) {
		t.Fatalf("scrolled content rect = %+v, want 6 lines offset by 4", rects["c"])
	}
	eqLines(t, frameLines(Col{Children: []Node{sc}}, 2, 2), "ee", "ff")
}

func TestOverlayMeasuresWrappedHeightAtItsWidth(t *testing.T) {
	root := Col{Children: []Node{
		Label{Text: "base"},
		Col{NodeBase: NodeBase{ID: "o", Overlay: true, W: Fixed(5)}, Children: []Node{
			Label{Text: "aaa bbb ccc", Wrap: true},
		}},
	}}
	_, rects := SolveAndCompose(root, 11, 9)
	if r := rects["o"]; r.W != 5 || r.H != 3 {
		t.Fatalf("overlay rect = %+v, want 5 wide and 3 lines tall", r)
	}
	// A proportional width is only known from the frame: half of 20 is 10,
	// where "aaa bbb ccc ddd" takes two lines.
	half := Col{Children: []Node{
		Label{Text: "base"},
		Col{NodeBase: NodeBase{ID: "o", Overlay: true, W: Grow(0.5)}, Children: []Node{
			Label{Text: "aaa bbb ccc ddd", Wrap: true},
		}},
	}}
	_, rects = SolveAndCompose(half, 20, 9)
	if r := rects["o"]; r.W != 10 || r.H != 2 {
		t.Fatalf("half-width overlay rect = %+v, want 10 wide and 2 lines tall", r)
	}
}

func TestLabelWrapSizeWithoutAWidth(t *testing.T) {
	// Alone, a wrapping label claims the width it is given (it grows) and
	// its unwrapped line count.
	w, h := Label{Text: "a b\nc", Wrap: true}.Size()
	if w.Kind != SizeGrow || h != (Size{Kind: SizeContent, Value: 2}) {
		t.Fatalf("Size = %+v × %+v", w, h)
	}
}

// wrapMetric measures text one Column per rune and wraps greedily by words,
// standing in for a GUI's font.
func wrapMetric() Metrics {
	m := testMetrics
	m.MeasureText = func(text string, _ TextStyle, maxWidth float64) (float64, float64) {
		cols := -1
		if maxWidth > 0 {
			cols = int(maxWidth / m.Column)
		}
		lines := wrapCells(text, cols)
		w := 0
		for _, l := range strings.Split(lines, "\n") {
			w = max(w, ansi.StringWidth(l))
		}
		return float64(w) * m.Column, float64(strings.Count(lines, "\n")+1) * m.Line
	}
	return m
}

func TestSceneWrapsInPoints(t *testing.T) {
	m := wrapMetric()
	root := Col{Children: []Node{
		Label{NodeBase: NodeBase{ID: "p"}, Text: "the quick brown fox jumps", Wrap: true},
		Label{NodeBase: NodeBase{ID: "after"}, Text: "after"},
	}}
	s := LayoutScene(root, 10*m.Column, 10*m.Line, m)
	if p := s.Rects["p"]; p.H != 3*m.Line {
		t.Fatalf("wrapped label height %v, want 3 lines (%v)", p.H, 3*m.Line)
	}
	if a := s.Rects["after"]; a.Y != 3*m.Line {
		t.Fatalf("next label at %v, want after 3 lines", a.Y)
	}
	for _, op := range s.Ops {
		if t2, ok := op.(TextOp); ok && t2.Text == "the quick brown fox jumps" && !t2.Wrap {
			t.Fatal("wrapped label's TextOp does not say Wrap")
		}
	}
}
