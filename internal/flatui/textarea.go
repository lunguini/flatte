package flatui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
)

// Textarea is a multi-line editable text buffer. The app owns it (like
// TextField): no goroutines, no key policy. Content is kept as logical lines;
// the cursor is a (row, byte-col) pair that the movement and edit methods keep
// on a grapheme-cluster boundary, so multi-rune clusters are never split.
// Vertical movement preserves a display "goal column"; the visible window
// scrolls to keep the cursor row in view. There is no soft-wrapping yet —
// lines longer than the width are the app's concern (a card or the terminal
// clips them); horizontal scrolling is a future addition.
type Textarea struct {
	lines   []string
	row     int // cursor line
	col     int // cursor byte offset within lines[row]
	goalCol int // desired display column, preserved across vertical moves
	width   int // reserved for future horizontal handling
	height  int // visible rows (0 = show all)
	offset  int // index of the first visible line
}

// SetValue replaces the content (split on newlines) and resets the cursor.
func (t *Textarea) SetValue(s string) {
	t.lines = strings.Split(s, "\n")
	t.row, t.col, t.goalCol, t.offset = 0, 0, 0, 0
	t.ensure()
}

// Value returns the content joined by newlines.
func (t Textarea) Value() string {
	if len(t.lines) == 0 {
		return ""
	}
	return strings.Join(t.lines, "\n")
}

// SetSize sets the visible window. Width is reserved (no wrapping yet); height
// caps the rows View emits and drives vertical scrolling.
func (t *Textarea) SetSize(width, height int) {
	t.width, t.height = width, height
	t.ensure()
	t.keepVisible()
}

// Row and Col expose the cursor position (line index, byte offset) for tests.
func (t Textarea) Row() int { return t.row }
func (t Textarea) Col() int { return t.col }

// Offset is the index of the first visible line.
func (t Textarea) Offset() int { return t.offset }

func (t *Textarea) Insert(r rune) {
	t.ensure()
	s := string(r)
	line := t.lines[t.row]
	t.lines[t.row] = line[:t.col] + s + line[t.col:]
	t.col += len(s)
	t.syncGoal()
	t.keepVisible()
}

// InsertNewline splits the current line at the cursor.
func (t *Textarea) InsertNewline() {
	t.ensure()
	line := t.lines[t.row]
	left, right := line[:t.col], line[t.col:]
	next := make([]string, 0, len(t.lines)+1)
	next = append(next, t.lines[:t.row]...)
	next = append(next, left, right)
	next = append(next, t.lines[t.row+1:]...)
	t.lines = next
	t.row++
	t.col = 0
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) Backspace() {
	t.ensure()
	if t.col > 0 {
		line := t.lines[t.row]
		start := prevGraphemeBoundary(line, t.col)
		t.lines[t.row] = line[:start] + line[t.col:]
		t.col = start
	} else if t.row > 0 {
		prev := t.lines[t.row-1]
		merged := prev + t.lines[t.row]
		t.lines = removeLineMerging(t.lines, t.row-1, merged)
		t.row--
		t.col = len(prev)
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) Delete() {
	t.ensure()
	line := t.lines[t.row]
	if t.col < len(line) {
		end := nextGraphemeBoundary(line, t.col)
		t.lines[t.row] = line[:t.col] + line[end:]
	} else if t.row < len(t.lines)-1 {
		merged := line + t.lines[t.row+1]
		t.lines = removeLineMerging(t.lines, t.row, merged)
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveLeft() {
	t.ensure()
	if t.col > 0 {
		t.col = prevGraphemeBoundary(t.lines[t.row], t.col)
	} else if t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveRight() {
	t.ensure()
	if t.col < len(t.lines[t.row]) {
		t.col = nextGraphemeBoundary(t.lines[t.row], t.col)
	} else if t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveWordLeft() {
	t.ensure()
	if t.col == 0 && t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
	t.col = prevWordBoundary(t.lines[t.row], t.col)
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveWordRight() {
	t.ensure()
	if t.col >= len(t.lines[t.row]) && t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
	t.col = nextWordBoundary(t.lines[t.row], t.col)
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveUp() {
	t.ensure()
	if t.row > 0 {
		t.row--
		t.col = byteOffsetForColumn(t.lines[t.row], t.goalCol)
	}
	t.keepVisible()
}

func (t *Textarea) MoveDown() {
	t.ensure()
	if t.row < len(t.lines)-1 {
		t.row++
		t.col = byteOffsetForColumn(t.lines[t.row], t.goalCol)
	}
	t.keepVisible()
}

// CursorCell is the cursor's display position relative to the visible window:
// x in display cells from the left of its line, y in rows from the top of the
// window. Pair with a frame origin to place Frame.Cursor.
func (t Textarea) CursorCell() (x, y int) {
	if len(t.lines) == 0 {
		return 0, 0
	}
	row := min(max(t.row, 0), len(t.lines)-1)
	col := min(max(t.col, 0), len(t.lines[row]))
	return lipgloss.Width(t.lines[row][:col]), row - t.offset
}

// View returns the visible lines joined by newlines (windowed to height).
func (t Textarea) View() string {
	if len(t.lines) == 0 {
		return ""
	}
	if t.height <= 0 {
		return strings.Join(t.lines, "\n")
	}
	end := min(t.offset+t.height, len(t.lines))
	return strings.Join(t.lines[t.offset:end], "\n")
}

// syncGoal records the cursor's current display column as the goal for
// subsequent vertical moves.
func (t *Textarea) syncGoal() {
	t.goalCol = lipgloss.Width(t.lines[t.row][:t.col])
}

// keepVisible scrolls the window so the cursor row stays inside it.
func (t *Textarea) keepVisible() {
	if t.height <= 0 {
		return
	}
	if t.row < t.offset {
		t.offset = t.row
	} else if t.row >= t.offset+t.height {
		t.offset = t.row - t.height + 1
	}
	t.offset = min(max(t.offset, 0), max(len(t.lines)-t.height, 0))
}

// ensure guarantees at least one line and a cursor on a valid rune boundary.
func (t *Textarea) ensure() {
	if len(t.lines) == 0 {
		t.lines = []string{""}
	}
	t.row = min(max(t.row, 0), len(t.lines)-1)
	t.col = min(max(t.col, 0), len(t.lines[t.row]))
	for t.col > 0 && t.col < len(t.lines[t.row]) && !utf8.RuneStart(t.lines[t.row][t.col]) {
		t.col--
	}
}

// removeLineMerging replaces lines[at] and lines[at+1] with a single merged
// line, returning the new slice.
func removeLineMerging(lines []string, at int, merged string) []string {
	next := make([]string, 0, len(lines)-1)
	next = append(next, lines[:at]...)
	next = append(next, merged)
	next = append(next, lines[at+2:]...)
	return next
}

// byteOffsetForColumn returns the byte offset of the grapheme boundary in line
// whose display column is the greatest not exceeding col (clamped to len).
func byteOffsetForColumn(line string, col int) int {
	state := -1
	rest := line
	at, width := 0, 0
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		cw := lipgloss.Width(cluster)
		if width+cw > col {
			break
		}
		at += len(cluster)
		width += cw
		rest, state = r, st
	}
	return at
}
