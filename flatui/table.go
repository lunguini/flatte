package flatui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Column is a table column: a header title and a fixed display width. Cells are
// padded to the width and truncated when wider.
type Column struct {
	Title string
	Width int
}

// Table is a columnar, row-selectable, vertically-scrolling grid. The app owns
// it (like the other flatui widgets): no goroutines, no key policy. Row
// selection and keep-visible scrolling are delegated to an embedded List;
// Table adds column alignment. Header and body are rendered separately so the
// app controls placement and styling; row appearance (selection marker/style)
// is the app's callback, so there is no policy in core.
type Table struct {
	cols []Column
	rows [][]string
	list List
}

// SetColumns sets the column titles and widths.
func (t *Table) SetColumns(cols []Column) {
	t.cols = append([]Column(nil), cols...)
}

// SetRows sets the row data and updates the selection bounds.
func (t *Table) SetRows(rows [][]string) {
	t.rows = rows
	t.list.SetCount(len(rows))
}

// SetHeight sets the number of visible body rows (excluding the header).
func (t *Table) SetHeight(h int) { t.list.SetHeight(h) }

// MoveUp / MoveDown / Select move the selected row, keeping it visible.
func (t *Table) MoveUp()      { t.list.MoveUp() }
func (t *Table) MoveDown()    { t.list.MoveDown() }
func (t *Table) Select(i int) { t.list.Select(i) }

// Cursor is the selected row index.
func (t Table) Cursor() int { return t.list.Cursor() }

// SelectedRow returns the selected row's cells (nil when there are no rows).
func (t Table) SelectedRow() []string {
	if len(t.rows) == 0 {
		return nil
	}
	return t.rows[t.list.Cursor()]
}

// Header returns the aligned header row.
func (t Table) Header() string {
	titles := make([]string, len(t.cols))
	for i, c := range t.cols {
		titles[i] = c.Title
	}
	return t.alignRow(titles)
}

func (t Table) HeaderWithStyle(style TableStyle) string {
	return style.Header.Render(t.Header())
}

// View returns the visible body rows, each aligned to the columns and passed
// through renderRow with whether it is the selected row. renderRow may be nil
// to render the aligned rows verbatim.
func (t Table) View(renderRow func(text string, selected bool) string) string {
	return t.list.View(func(i int, selected bool) string {
		text := t.alignRow(t.rows[i])
		if renderRow != nil {
			return renderRow(text, selected)
		}
		return text
	})
}

func (t Table) ViewWithStyle(style TableStyle) string {
	return t.View(func(text string, selected bool) string {
		if selected {
			return style.Active.Render(text)
		}
		return style.Row.Render(text)
	})
}

// alignRow pads/truncates cells to their column widths, joined by a space.
func (t Table) alignRow(cells []string) string {
	parts := make([]string, len(t.cols))
	for i, c := range t.cols {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = padOrTruncate(cell, c.Width)
	}
	return strings.Join(parts, " ")
}

// padOrTruncate fits s to exactly w display cells (ANSI- and width-aware).
func padOrTruncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	width := lipgloss.Width(s)
	if width > w {
		return ansi.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-width)
}
