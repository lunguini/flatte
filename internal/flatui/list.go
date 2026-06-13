package flatui

import "strings"

// List is selection-and-scroll state over a flat sequence of items. The app
// owns it (like TextField and Viewport): no goroutines, no key policy. It holds
// no item data and imposes no appearance — the app supplies the item count,
// drives the cursor, and renders each visible row through a callback, so the
// selection marker and styling stay app policy, never the framework's. The
// selected row is always kept inside the visible window.
type List struct {
	count  int
	cursor int
	offset int
	height int
}

// SetCount sets the number of selectable items, clamping the cursor and scroll.
func (l *List) SetCount(n int) {
	l.count = max(n, 0)
	l.clamp()
}

// SetHeight sets the number of visible rows.
func (l *List) SetHeight(h int) {
	l.height = h
	l.clamp()
}

// MoveUp / MoveDown move the selection by one, keeping it visible.
func (l *List) MoveUp()   { l.cursor--; l.clamp() }
func (l *List) MoveDown() { l.cursor++; l.clamp() }

// Select moves the selection to index i (clamped), keeping it visible.
func (l *List) Select(i int) { l.cursor = i; l.clamp() }

// Cursor is the selected index (0 when empty).
func (l List) Cursor() int { return l.cursor }

// Count is the number of items.
func (l List) Count() int { return l.count }

// Offset is the index of the first visible row (for tests and indicators).
func (l List) Offset() int { return l.offset }

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

// clamp keeps the cursor in range and the window on the cursor.
func (l *List) clamp() {
	l.cursor = min(max(l.cursor, 0), max(l.count-1, 0))
	if l.height > 0 {
		if l.cursor < l.offset {
			l.offset = l.cursor
		} else if l.cursor >= l.offset+l.height {
			l.offset = l.cursor - l.height + 1
		}
	}
	l.offset = min(max(l.offset, 0), max(l.count-l.height, 0))
}
