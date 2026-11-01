package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

func sampleTable() Table {
	var tb Table
	tb.SetColumns([]Column{{Title: "Name", Width: 6}, {Title: "Age", Width: 3}})
	tb.SetRows([][]string{{"Ann", "30"}, {"Bob", "9"}})
	tb.SetHeight(5)
	return tb
}

func TestTableHeaderAndRowsAlignToColumnWidths(t *testing.T) {
	tb := sampleTable()
	// total width = 6 + 1 (separator) + 3 = 10
	if w := lipgloss.Width(tb.Header()); w != 10 {
		t.Fatalf("Header width = %d, want 10\n%q", w, tb.Header())
	}
	if !strings.HasPrefix(tb.Header(), "Name") {
		t.Fatalf("Header = %q, want to start with Name", tb.Header())
	}
	lines := strings.Split(tb.View(nil), "\n")
	if len(lines) != 2 {
		t.Fatalf("View has %d body lines, want 2", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 10 {
			t.Fatalf("row %d width = %d, want 10\n%q", i, w, line)
		}
	}
	if !strings.HasPrefix(lines[0], "Ann") || !strings.Contains(lines[0], "30") {
		t.Fatalf("row 0 = %q, want Ann ... 30", lines[0])
	}
}

func TestTableTruncatesOverWideCells(t *testing.T) {
	var tb Table
	tb.SetColumns([]Column{{Title: "C", Width: 3}})
	tb.SetRows([][]string{{"abcdef"}})
	tb.SetHeight(5)
	line := tb.View(nil)
	if w := lipgloss.Width(line); w != 3 {
		t.Fatalf("over-wide cell width = %d, want 3 (truncated)\n%q", w, line)
	}
}

func TestTableSelectionScrollsAndReportsRow(t *testing.T) {
	var tb Table
	tb.SetColumns([]Column{{Title: "N", Width: 4}})
	rows := make([][]string, 10)
	for i := range rows {
		rows[i] = []string{string(rune('0' + i))}
	}
	tb.SetRows(rows)
	tb.SetHeight(3)
	for range 5 {
		tb.MoveDown()
	}
	if tb.Cursor() != 5 {
		t.Fatalf("Cursor() = %d, want 5", tb.Cursor())
	}
	if got := tb.SelectedRow(); len(got) != 1 || got[0] != "5" {
		t.Fatalf("SelectedRow() = %v, want [5]", got)
	}
	// keep-visible scroll comes from the embedded List
	body := strings.Split(tb.View(nil), "\n")
	if len(body) != 3 {
		t.Fatalf("body has %d rows, want 3 (height)", len(body))
	}
	if !strings.HasPrefix(body[len(body)-1], "5") {
		t.Fatalf("last visible row = %q, want the selected row 5", body[len(body)-1])
	}
}

func TestTableRenderCallbackMarksSelectedRow(t *testing.T) {
	tb := sampleTable()
	body := tb.View(func(text string, selected bool) string {
		if selected {
			return "> " + text
		}
		return "  " + text
	})
	lines := strings.Split(body, "\n")
	if !strings.HasPrefix(lines[0], "> ") {
		t.Fatalf("selected row 0 = %q, want '> ' prefix", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Fatalf("unselected row 1 = %q, want '  ' prefix", lines[1])
	}
}

func TestTableEmptyView(t *testing.T) {
	var tb Table
	tb.SetColumns([]Column{{Title: "X", Width: 3}})
	tb.SetHeight(3)
	if got := tb.View(nil); got != "" {
		t.Fatalf("empty View() = %q, want empty", got)
	}
}

func TestTableViewWithStyle(t *testing.T) {
	tb := sampleTable()
	gotHeader := tb.HeaderWithStyle(TableStyle{
		Header: lipgloss.NewStyle().Bold(true),
	})
	if !strings.Contains(gotHeader, "\x1b[") || ansi.Strip(gotHeader) != tb.Header() {
		t.Fatalf("HeaderWithStyle() = %q", gotHeader)
	}

	gotBody := tb.ViewWithStyle(TableStyle{
		Row:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Active: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	})
	if strings.Count(gotBody, "\x1b[") < 2 {
		t.Fatalf("ViewWithStyle() missing styled rows: %q", gotBody)
	}
	if ansi.Strip(gotBody) != tb.View(nil) {
		t.Fatalf("stripped ViewWithStyle() = %q, want %q", ansi.Strip(gotBody), tb.View(nil))
	}
}

// TestTableNodesMatchTheStringViews: the node forms of header and rows
// compose to Header and View's text (cells padded, truncated, one column
// apart), with the rows ID'd and the selected one marked like List.Rows.
func TestTableNodesMatchTheStringViews(t *testing.T) {
	tb := sampleTable()
	tb.SetRows([][]string{{"Ann", "30"}, {"Bobbinsworth", "9"}, {"Cy"}}) // over-wide and short rows
	tb.Select(1)
	header, _ := layout.SolveAndCompose(tb.HeaderLayout(layout.TextStyle{}), 10, 1)
	if header != tb.Header() {
		t.Fatalf("HeaderLayout composed %q, want Header %q", header, tb.Header())
	}
	var gotCells [][]string
	rows := tb.Rows("t", func(cells []string, selected bool) layout.Node {
		gotCells = append(gotCells, cells)
		return tb.Cells(cells, layout.TextStyle{})
	})
	body, rects := layout.SolveAndCompose(rows, 10, 3)
	// Composed frames drop trailing blank cells; compare the visible text.
	trim := func(s string) string {
		lines := strings.Split(s, "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " ")
		}
		return strings.Join(lines, "\n")
	}
	if want := tb.View(nil); trim(flatest.CleanFrame(body)) != trim(want) {
		t.Fatalf("Rows composed %q, want View %q", body, want)
	}
	if len(gotCells) != 3 || gotCells[1][0] != "Bobbinsworth" {
		t.Fatalf("row callback got %q, want each row's own cells", gotCells)
	}
	if rows.Children[1].(layout.Row).Box.Selected != true || rects["t:2"] != (layout.Rect{Y: 2, W: 10, H: 1}) {
		t.Fatalf("selected/IDs wrong: %+v, t:2 = %+v", rows.Children[1], rects["t:2"])
	}
}
