package flatui

import (
	"strconv"
	"strings"

	"github.com/lunguini/flatte/flatui/layout"
)

// List is selection-and-scroll state over a flat sequence of items. The app
// owns it (like TextField and Viewport): no goroutines, no key policy. It holds
// no item data and imposes no appearance — the app supplies the item count,
// drives the cursor, and renders each visible row through a callback, so the
// selection marker and styling stay app policy, never the framework's.
// Moving the selection keeps it inside the visible window; Scroll (a wheel)
// moves the window alone, and may leave the selection out of view until it
// next moves.
type List struct {
	count  int
	cursor int
	offset int
	height int

	// RenderRow renders one row for Layout(); set by the app. It receives the
	// item index and whether it is the selected row.
	RenderRow func(index int, selected bool) string
}

// Layout renders the visible rows via RenderRow as a single Text leaf. If
// RenderRow is unset, it renders nothing.
func (l List) Layout() layout.Node {
	if l.RenderRow == nil {
		return layout.Text{}
	}
	return layout.Text{String: l.View(l.RenderRow)}
}

// Rows returns the visible rows as nodes, for a Frame.Root: row is called
// for each visible item index with whether it is the selected row, and each
// result is wrapped in a Row with ID "id:index" (so MouseEvent.Target names
// the clicked row) whose Box is marked Selected for the selected one (a GUI
// highlights it; the terminal shows only what row draws). The Col is the
// app's: give it an ID or a size (usually H: layout.Grow(1), sizing the list
// from its solved height) as needed.
func (l List) Rows(id string, row func(index int, selected bool) layout.Node) layout.Col {
	var col layout.Col
	if l.height <= 0 || l.count == 0 {
		return col
	}
	end := min(l.offset+l.height, l.count)
	col.Children = make([]layout.Node, 0, end-l.offset)
	for i := l.offset; i < end; i++ {
		selected := i == l.cursor
		col.Children = append(col.Children, layout.Row{
			NodeBase: layout.NodeBase{ID: id + ":" + strconv.Itoa(i), Box: layout.Box{Selected: selected}},
			Children: []layout.Node{row(i, selected)},
		})
	}
	return col
}

// SetCount sets the number of selectable items, clamping the cursor and
// scroll; a visible selection stays visible.
func (l *List) SetCount(n int) {
	shown := l.cursorShown()
	l.count = max(n, 0)
	l.clamp(shown)
}

// SetHeight sets the number of visible rows, clamped to the supported widget
// dimension range; a visible selection stays visible.
func (l *List) SetHeight(h int) {
	shown := l.cursorShown()
	l.height = clampWidgetDimension(h)
	l.clamp(shown)
}

// MoveUp / MoveDown move the selection by one, keeping it visible.
func (l *List) MoveUp()   { l.cursor--; l.clamp(true) }
func (l *List) MoveDown() { l.cursor++; l.clamp(true) }

// Select moves the selection to index i (clamped), keeping it visible.
func (l *List) Select(i int) { l.cursor = i; l.clamp(true) }

// Scroll moves the visible window by rows (negative: up), clamped, leaving
// the selection where it is — for a mouse wheel.
func (l *List) Scroll(rows int) {
	l.offset += rows
	l.clamp(false)
}

// Cursor is the selected index (0 when empty).
func (l List) Cursor() int { return l.cursor }

// Count is the number of items.
func (l List) Count() int { return l.count }

// Offset is the index of the first visible row (for tests and indicators).
func (l List) Offset() int { return l.offset }

// Height returns the number of visible rows the list was sized to via SetHeight.
func (l List) Height() int { return l.height }

// View renders the visible rows: render is called for each visible item index
// with whether it is the selected row, and the results are joined by newlines.
// The app decides each row's appearance (marker, styling, truncation).
func (l List) View(render func(index int, selected bool) string) string {
	if l.height <= 0 || l.count == 0 {
		return ""
	}
	end := min(l.offset+l.height, l.count)
	rows := make([]string, 0, end-l.offset)
	for i := l.offset; i < end; i++ {
		rows = append(rows, render(i, i == l.cursor))
	}
	return strings.Join(rows, "\n")
}

// ScrollTo shows the window starting at row offset (clamped), keeping the
// cursor inside it: a cursor scrolled out moves to the nearest visible row.
// For a scroll bar press or drag (layout.ScrollBar.OffsetAt).
func (l *List) ScrollTo(offset int) {
	if l.height <= 0 {
		return
	}
	l.offset = min(max(offset, 0), max(l.count-l.height, 0))
	l.cursor = min(max(l.cursor, l.offset), l.offset+l.height-1)
	l.clamp(true)
}

// cursorShown reports the selection inside the window (or no window yet).
func (l List) cursorShown() bool {
	return l.height <= 0 || l.cursor >= l.offset && l.cursor < l.offset+l.height
}

// clamp keeps the cursor and the window in range, and with reveal the
// window on the cursor.
func (l *List) clamp(reveal bool) {
	l.cursor = min(max(l.cursor, 0), max(l.count-1, 0))
	if reveal && l.height > 0 {
		if l.cursor < l.offset {
			l.offset = l.cursor
		} else if l.cursor >= l.offset+l.height {
			l.offset = l.cursor - l.height + 1
		}
	}
	l.offset = min(max(l.offset, 0), max(l.count-l.height, 0))
}
