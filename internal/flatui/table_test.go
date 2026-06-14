package flatui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
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
