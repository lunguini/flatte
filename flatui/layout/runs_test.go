package layout

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLabelRunsRender(t *testing.T) {
	red := TextStyle{Fg: lipgloss.Color("1")}
	l := Label{Style: TextStyle{Italic: true}, Runs: []Run{
		{Text: "plain "},
		{Text: "red", Style: red},
		{Text: " tail"},
	}}
	if w, h := l.Size(); w != (Size{Kind: SizeContent, Value: 14}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Size = %+v %+v", w, h)
	}
	got := l.Render(Rect{W: 14, H: 1})
	italic := TextStyle{Italic: true}
	want := italic.Render("plain ") + red.Render("red") + italic.Render(" tail")
	if got != want {
		t.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
	// Text is ignored once there are runs.
	l.Text = "ignored"
	if l.Render(Rect{W: 14, H: 1}) != want {
		t.Error("Text leaked into a runs label")
	}
}

func TestLabelRunsWrap(t *testing.T) {
	bold := TextStyle{Bold: true}
	l := Label{Wrap: true, Runs: []Run{
		{Text: "one two "},
		{Text: "three four", Style: bold},
		{Text: " five\nsix"},
	}}
	r := Rect{W: 9, H: 4}
	got := l.Render(r)
	lines := strings.Split(ansi.Strip(got), "\n")
	want := []string{"one two  ", "three    ", "four five", "six      "}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("wrapped = %q, want %q", lines, want)
	}
	// Each line's styled pieces stay on their line: the bold run is
	// rendered per line, never across a break.
	raw := strings.Split(got, "\n")
	if !strings.Contains(raw[1], bold.Render("three")) || !strings.Contains(raw[2], bold.Render("four")) {
		t.Errorf("bold not per line:\n%q", raw)
	}
	if strings.Contains(raw[0], "\x1b[1m") || strings.Contains(raw[3], "\x1b[1m") {
		t.Errorf("bold leaked:\n%q", raw)
	}
	// The wrapped height counts the runs' text.
	if h := l.wrapHeight(9); h != (Size{Kind: SizeContent, Value: 4}) {
		t.Errorf("wrapHeight = %+v", h)
	}
	if rects := Solve(Col{Children: []Node{l, Label{NodeBase: NodeBase{ID: "after"}, Text: "x"}}}, 9, 10); rects["after"].Y != 4 {
		t.Errorf("after at %+v", rects["after"])
	}
}

// renderRuns survives whatever a wrap does to the text: every rune of the
// runs' text comes out in order, in its run's style.
func TestRenderRunsProperty(t *testing.T) {
	runs := []Run{{Text: "aa bb  cc", Style: TextStyle{Bold: true}}, {Text: " dd\nee ff ", Style: TextStyle{Italic: true}}, {Text: "", Style: TextStyle{Underline: true}}, {Text: "gg hhhhhhhhh"}}
	var text strings.Builder
	for _, r := range runs {
		text.WriteString(r.Text)
	}
	for w := 1; w < 16; w++ {
		wrapped := wrapCells(text.String(), w)
		got := ansi.Strip(renderRuns(runs, wrapped))
		if got != wrapped {
			t.Fatalf("w=%d: %q, want %q", w, got, wrapped)
		}
	}
}

func TestSceneRunsOp(t *testing.T) {
	l := Label{Style: TextStyle{Role: TextSubtle}, Runs: []Run{{Text: "a "}, {Text: "b", Style: TextStyle{Bold: true}}}}
	var gotRuns []Run
	m := testMetrics
	m.MeasureRuns = func(runs []Run, _ float64) (float64, float64) {
		gotRuns = runs
		return 30, 22
	}
	s := LayoutScene(Col{Children: []Node{l, Label{NodeBase: NodeBase{ID: "after"}, Text: "x"}}}, 200, 100, m)
	if len(gotRuns) != 2 || gotRuns[0].Style.Role != TextSubtle || !gotRuns[1].Style.Bold || gotRuns[1].Style.Role != TextBody {
		t.Fatalf("MeasureRuns got %+v", gotRuns)
	}
	if s.Rects["after"].Y != 22 {
		t.Errorf("after at %+v", s.Rects["after"])
	}
	op, ok := s.Ops[0].(TextOp)
	if !ok || op.Text != "a b" || len(op.Runs) != 2 || op.Runs[0].Style.Role != TextSubtle {
		t.Errorf("op = %+v", s.Ops[0])
	}
	// Without MeasureRuns, MeasureText measures the plain text.
	s = LayoutScene(Col{Children: []Node{l, Label{NodeBase: NodeBase{ID: "after"}, Text: "x"}}}, 200, 100, testMetrics)
	if s.Rects["after"].Y != 16 {
		t.Errorf("fallback: after at %+v", s.Rects["after"])
	}
}
