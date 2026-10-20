package layout

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMeterTerminalBar(t *testing.T) {
	for _, c := range []struct {
		value float64
		w     int
		want  string
	}{
		{0, 10, "░░░░░░░░░░"},
		{1, 10, "██████████"},
		{0.63, 10, "██████░░░░"}, // rounds like flatui.Progress
		{0.65, 10, "███████░░░"}, // .5 rounds up
		{-1, 4, "░░░░"},          // clamped
		{2, 4, "████"},           // clamped
		{math.NaN(), 4, "░░░░"},  // NaN is empty
		{0.5, 1, "█"},            // one cell
	} {
		got := ansi.Strip(Compose(Meter{Value: c.value}, c.w, 1).Frame)
		if got != c.want {
			t.Errorf("Meter %v at %d = %q, want %q", c.value, c.w, got, c.want)
		}
	}
}

func TestMeterColorsAndSize(t *testing.T) {
	fill, track := lipgloss.Color("2"), lipgloss.Color("240")
	m := Meter{Value: 0.5, Fill: fill, Track: track}
	got := m.Render(Rect{W: 4, H: 1})
	want := lipgloss.NewStyle().Foreground(fill).Render("██") + lipgloss.NewStyle().Foreground(track).Render("░░")
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
	// It fills the width like an Input, one line tall; Fixed widths pin it.
	w, h := m.Size()
	if w.Kind != SizeGrow || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Size = %+v × %+v", w, h)
	}
	m.W = Fixed(6)
	if w, _ := m.Size(); w != Fixed(6) {
		t.Fatalf("fixed Size = %+v", w)
	}
	if s := m.Render(Rect{W: 0, H: 1}); s != "" {
		t.Fatalf("zero-width Render = %q", s)
	}
}

func TestSceneMeter(t *testing.T) {
	root := Row{Children: []Node{Meter{NodeBase: NodeBase{ID: "m"}, Value: 0.25, Fill: lipgloss.Color("2")}, Label{Text: "  25%"}}}
	s := LayoutScene(root, 200, 100, testMetrics)
	var op *MeterOp
	for _, o := range s.Ops {
		if m, ok := o.(MeterOp); ok {
			op = &m
		}
	}
	if op == nil || op.Value != 0.25 || op.Rect != s.Rects["m"] || op.Fill == nil || op.Track != nil {
		t.Fatalf("meter op = %+v (rect %+v)", op, s.Rects["m"])
	}
	if !strings.Contains(Compose(root, 20, 1).Frame, "25%") {
		t.Fatal("label lost")
	}
}
