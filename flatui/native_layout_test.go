package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

// visibleText is a composed frame's text as View writes it: no escape
// sequences and no trailing blanks (composition drops them).
func visibleText(s string) string {
	lines := strings.Split(flatest.CleanFrame(s), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func TestProgressLayoutIsAMeterAndItsLabel(t *testing.T) {
	for _, pct := range []float64{0, 37, 63, 100} {
		p := NewProgress(20)
		p.SetPercent(pct)
		got, _ := layout.SolveAndCompose(p.Layout(), 26, 1)
		if visibleText(got) != visibleText(p.View()) {
			t.Fatalf("%v%%: Layout %q, want View %q", pct, visibleText(got), p.View())
		}
	}
	row, ok := NewProgress(10).Layout().(layout.Row)
	if !ok || len(row.Children) != 2 {
		t.Fatalf("Layout = %#v, want a Row of a Meter and its label", NewProgress(10).Layout())
	}
	if m, ok := row.Children[0].(layout.Meter); !ok || m.W != layout.Fixed(10) {
		t.Fatalf("bar = %#v, want a 10-column Meter", row.Children[0])
	}
	// A zero-width bar is only the label, as View is.
	zero := NewProgress(0)
	zero.SetPercent(50)
	if got, _ := layout.SolveAndCompose(zero.Layout(), 10, 1); visibleText(got) != " 50%" {
		t.Fatalf("zero-width Layout = %q", got)
	}
}

func TestViewportLayoutScrollsItsLines(t *testing.T) {
	var lines []string
	for i := range 12 {
		lines = append(lines, strings.Repeat("word ", i%4+1)+"end")
	}
	for _, c := range []struct{ w, h, off int }{{30, 5, 0}, {30, 5, 4}, {8, 5, 7}, {30, 20, 0}, {0, 3, 2}} {
		var v Viewport
		v.SetLines(lines)
		v.SetSize(c.w, c.h)
		v.ScrollToLine(c.off)
		frameW := c.w
		if frameW == 0 {
			frameW = 40
		}
		got, _ := layout.SolveAndCompose(v.Layout(), frameW, v.VisibleLines())
		if visibleText(got) != visibleText(v.View()) {
			t.Fatalf("%+v: Layout\n%s\nwant View\n%s", c, visibleText(got), v.View())
		}
	}
	// Plain lines are a Label (a window draws them natively); styled ones
	// stay a Text of cells.
	var v Viewport
	v.SetLines([]string{"plain"})
	v.SetSize(10, 2)
	if _, ok := v.Layout().(layout.Scroll).Child.(layout.Label); !ok {
		t.Fatalf("plain viewport child = %#v, want a Label", v.Layout())
	}
	v.SetLines([]string{lipgloss.NewStyle().Bold(true).Render("styled")})
	if _, ok := v.Layout().(layout.Scroll).Child.(layout.Text); !ok {
		t.Fatalf("styled viewport child = %#v, want a Text", v.Layout())
	}
	var empty Viewport
	if got, _ := layout.SolveAndCompose(empty.Layout(), 10, 2); visibleText(got) != "" {
		t.Fatalf("empty viewport composed %q", got)
	}
}
