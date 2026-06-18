package flatui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

type TextPosition struct {
	Row int
	Col int
}

type TextRange struct {
	Start TextPosition
	End   TextPosition
}

// Textarea is a multi-line editable text buffer. The app owns it (like
// TextField): no goroutines, no key policy. Content is kept as logical lines;
// the cursor is a (row, byte-col) pair that the movement and edit methods keep
// on a grapheme-cluster boundary, so multi-rune clusters are never split.
// Vertical movement preserves a display "goal column"; the visible window
// scrolls to keep the cursor row in view. Width, when positive, horizontally
// scrolls long lines to keep the cursor cell visible. There is no soft-wrapping
// yet.
type Textarea struct {
	lines   []string
	row     int // cursor line
	col     int // cursor byte offset within lines[row]
	goalCol int // desired display column, preserved across vertical moves
	width   int // visible columns (0 = no horizontal window)
	height  int // visible rows (0 = show all)
	offset  int // index of the first visible line
	xOffset int // display-cell offset of the first visible column

	selectionActive bool
	anchorRow       int
	anchorCol       int
}

// SetValue replaces the content (split on newlines) and resets the cursor.
func (t *Textarea) SetValue(s string) {
	t.lines = strings.Split(s, "\n")
	t.row, t.col, t.goalCol, t.offset, t.xOffset = 0, 0, 0, 0, 0
	t.ClearSelection()
	t.ensure()
}

// Value returns the content joined by newlines.
func (t Textarea) Value() string {
	if len(t.lines) == 0 {
		return ""
	}
	return strings.Join(t.lines, "\n")
}

// SetSize sets the visible window. Width caps the columns View emits and drives
// horizontal scrolling; height caps the rows View emits and drives vertical
// scrolling.
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

// Selection returns the selected logical range, normalized as [Start,End). Col
// values are byte offsets into their rows. The range is false when no non-empty
// selection is active.
func (t Textarea) Selection() (TextRange, bool) {
	if !t.selectionActive || len(t.lines) == 0 {
		return TextRange{}, false
	}
	cursor := t.clampedPosition(TextPosition{Row: t.row, Col: t.col})
	anchor := t.clampedPosition(TextPosition{Row: t.anchorRow, Col: t.anchorCol})
	if compareTextPosition(cursor, anchor) == 0 {
		return TextRange{}, false
	}
	if compareTextPosition(anchor, cursor) < 0 {
		return TextRange{Start: anchor, End: cursor}, true
	}
	return TextRange{Start: cursor, End: anchor}, true
}

func (t Textarea) SelectedText() string {
	rng, ok := t.Selection()
	if !ok {
		return ""
	}
	if rng.Start.Row == rng.End.Row {
		line := t.lines[rng.Start.Row]
		return line[rng.Start.Col:rng.End.Col]
	}
	parts := make([]string, 0, rng.End.Row-rng.Start.Row+1)
	parts = append(parts, t.lines[rng.Start.Row][rng.Start.Col:])
	for row := rng.Start.Row + 1; row < rng.End.Row; row++ {
		parts = append(parts, t.lines[row])
	}
	parts = append(parts, t.lines[rng.End.Row][:rng.End.Col])
	return strings.Join(parts, "\n")
}

func (t *Textarea) ClearSelection() {
	t.selectionActive = false
	t.anchorRow, t.anchorCol = 0, 0
}

func (t *Textarea) Insert(r rune) {
	t.ensure()
	t.deleteSelection()
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
	t.deleteSelection()
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
	if t.deleteSelection() {
		return
	}
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
	if t.deleteSelection() {
		return
	}
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
	t.moveLeft()
	t.ClearSelection()
}

func (t *Textarea) MoveRight() {
	t.moveRight()
	t.ClearSelection()
}

func (t *Textarea) MoveLeftSelecting() {
	t.startSelection()
	t.moveLeft()
}

func (t *Textarea) MoveRightSelecting() {
	t.startSelection()
	t.moveRight()
}

func (t *Textarea) moveLeft() {
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

func (t *Textarea) moveRight() {
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
	t.moveWordLeft()
	t.ClearSelection()
}

func (t *Textarea) MoveWordRight() {
	t.moveWordRight()
	t.ClearSelection()
}

func (t *Textarea) MoveWordLeftSelecting() {
	t.startSelection()
	t.moveWordLeft()
}

func (t *Textarea) MoveWordRightSelecting() {
	t.startSelection()
	t.moveWordRight()
}

func (t *Textarea) moveWordLeft() {
	t.ensure()
	if t.col == 0 && t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
	t.col = prevWordBoundary(t.lines[t.row], t.col)
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) moveWordRight() {
	t.ensure()
	if t.col >= len(t.lines[t.row]) && t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
	t.col = nextWordBoundary(t.lines[t.row], t.col)
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) DeleteWordLeft() {
	t.ensure()
	if t.deleteSelection() {
		return
	}
	if t.col > 0 {
		line := t.lines[t.row]
		start := prevWordBoundary(line, t.col)
		t.lines[t.row] = line[:start] + line[t.col:]
		t.col = start
	} else if t.row > 0 {
		prev := t.lines[t.row-1]
		start := prevWordBoundary(prev, len(prev))
		merged := prev[:start] + t.lines[t.row]
		t.lines = removeLineMerging(t.lines, t.row-1, merged)
		t.row--
		t.col = start
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) DeleteWordRight() {
	t.ensure()
	if t.deleteSelection() {
		return
	}
	line := t.lines[t.row]
	if t.col < len(line) {
		end := nextWordBoundary(line, t.col)
		t.lines[t.row] = line[:t.col] + line[end:]
	} else if t.row < len(t.lines)-1 {
		next := t.lines[t.row+1]
		end := nextWordBoundary(next, 0)
		merged := line + next[end:]
		t.lines = removeLineMerging(t.lines, t.row, merged)
	}
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) MoveUp() {
	t.moveUp()
	t.ClearSelection()
}

func (t *Textarea) MoveDown() {
	t.moveDown()
	t.ClearSelection()
}

func (t *Textarea) MoveUpSelecting() {
	t.startSelection()
	t.moveUp()
}

func (t *Textarea) MoveDownSelecting() {
	t.startSelection()
	t.moveDown()
}

func (t *Textarea) moveUp() {
	t.ensure()
	if t.row > 0 {
		t.row--
		t.col = byteOffsetForColumn(t.lines[t.row], t.goalCol)
	}
	t.keepVisible()
}

func (t *Textarea) moveDown() {
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
	return max(lipgloss.Width(t.lines[row][:col])-t.xOffset, 0), row - t.offset
}

// View returns the visible lines joined by newlines (windowed to height).
func (t Textarea) View() string {
	if len(t.lines) == 0 {
		return ""
	}
	if t.height <= 0 {
		return horizontalWindowLines(t.lines, t.xOffset, t.width)
	}
	end := min(t.offset+t.height, len(t.lines))
	return horizontalWindowLines(t.lines[t.offset:end], t.xOffset, t.width)
}

// ViewWithSelection renders the visible textarea window and calls render for
// selected and unselected runs. The callback owns style policy.
func (t Textarea) ViewWithSelection(render func(text string, selected bool) string) string {
	if render == nil {
		return t.View()
	}
	if len(t.lines) == 0 {
		return ""
	}
	startRow, endRow := 0, len(t.lines)
	if t.height > 0 {
		startRow = t.offset
		endRow = min(t.offset+t.height, len(t.lines))
	}
	rng, hasSelection := t.Selection()
	lines := make([]string, 0, endRow-startRow)
	for row := startRow; row < endRow; row++ {
		selStart, selEnd, selected := selectionColumnsForLine(t.lines[row], row, rng, hasSelection)
		lines = append(lines, horizontalWindowLineWithSelection(t.lines[row], t.xOffset, t.width, selStart, selEnd, selected, render))
	}
	return strings.Join(lines, "\n")
}

// syncGoal records the cursor's current display column as the goal for
// subsequent vertical moves.
func (t *Textarea) syncGoal() {
	t.goalCol = lipgloss.Width(t.lines[t.row][:t.col])
}

// keepVisible scrolls the window so the cursor row and column stay inside it.
func (t *Textarea) keepVisible() {
	if t.height > 0 {
		if t.row < t.offset {
			t.offset = t.row
		} else if t.row >= t.offset+t.height {
			t.offset = t.row - t.height + 1
		}
		t.offset = min(max(t.offset, 0), max(len(t.lines)-t.height, 0))
	}

	if t.width <= 0 {
		t.xOffset = 0
		return
	}
	line := t.lines[t.row]
	cursorCol := lipgloss.Width(line[:t.col])
	if cursorCol < t.xOffset {
		t.xOffset = cursorCol
	} else if cursorCol >= t.xOffset+t.width {
		t.xOffset = displayColumnAtOrAfter(line, cursorCol-t.width+1)
	}
	t.xOffset = min(max(t.xOffset, 0), lipgloss.Width(line))
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

func (t *Textarea) startSelection() {
	t.ensure()
	if !t.selectionActive {
		t.anchorRow = t.row
		t.anchorCol = t.col
		t.selectionActive = true
	}
}

func (t *Textarea) deleteSelection() bool {
	rng, ok := t.Selection()
	if !ok {
		return false
	}
	if rng.Start.Row == rng.End.Row {
		line := t.lines[rng.Start.Row]
		t.lines[rng.Start.Row] = line[:rng.Start.Col] + line[rng.End.Col:]
	} else {
		merged := t.lines[rng.Start.Row][:rng.Start.Col] + t.lines[rng.End.Row][rng.End.Col:]
		next := make([]string, 0, len(t.lines)-(rng.End.Row-rng.Start.Row))
		next = append(next, t.lines[:rng.Start.Row]...)
		next = append(next, merged)
		next = append(next, t.lines[rng.End.Row+1:]...)
		t.lines = next
	}
	t.row = rng.Start.Row
	t.col = rng.Start.Col
	t.ClearSelection()
	t.syncGoal()
	t.keepVisible()
	return true
}

func (t Textarea) clampedPosition(pos TextPosition) TextPosition {
	if len(t.lines) == 0 {
		return TextPosition{}
	}
	row := min(max(pos.Row, 0), len(t.lines)-1)
	col := min(max(pos.Col, 0), len(t.lines[row]))
	for col > 0 && col < len(t.lines[row]) && !utf8.RuneStart(t.lines[row][col]) {
		col--
	}
	return TextPosition{Row: row, Col: col}
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

func horizontalWindowLines(lines []string, offset, width int) string {
	if width <= 0 && offset <= 0 {
		return strings.Join(lines, "\n")
	}
	windowed := make([]string, len(lines))
	for i, line := range lines {
		windowed[i] = horizontalWindowLine(line, offset, width)
	}
	return strings.Join(windowed, "\n")
}

func horizontalWindowLine(line string, offset, width int) string {
	return horizontalWindowLineWithSelection(line, offset, width, 0, 0, false, func(text string, _ bool) string {
		return text
	})
}

func horizontalWindowLineWithSelection(line string, offset, width int, selStart, selEnd int, hasSelection bool, render func(text string, selected bool) string) string {
	if width <= 0 {
		return renderLineWithSelection(line, selStart, selEnd, hasSelection, render)
	}
	right := offset + width
	state := -1
	rest := line
	display := 0
	var out strings.Builder
	var segment strings.Builder
	segmentSelected := false
	haveSegment := false
	flush := func() {
		if !haveSegment {
			return
		}
		out.WriteString(render(segment.String(), segmentSelected))
		segment.Reset()
		haveSegment = false
	}
	at := 0
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		clusterWidth := lipgloss.Width(cluster)
		nextDisplay := display + clusterWidth
		if display >= offset && nextDisplay <= right {
			selected := hasSelection && at >= selStart && at+len(cluster) <= selEnd
			if haveSegment && selected != segmentSelected {
				flush()
			}
			segment.WriteString(cluster)
			segmentSelected = selected
			haveSegment = true
		}
		at += len(cluster)
		display = nextDisplay
		rest, state = r, st
	}
	flush()
	return out.String()
}

func displayColumnAtOrAfter(line string, col int) int {
	if col <= 0 {
		return 0
	}
	state := -1
	rest := line
	display := 0
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		if display >= col {
			return display
		}
		display += lipgloss.Width(cluster)
		rest, state = r, st
	}
	return display
}

func renderLineWithSelection(line string, selStart, selEnd int, hasSelection bool, render func(text string, selected bool) string) string {
	if !hasSelection {
		return render(line, false)
	}
	var out strings.Builder
	if selStart > 0 {
		out.WriteString(render(line[:selStart], false))
	}
	if selStart < selEnd {
		out.WriteString(render(line[selStart:selEnd], true))
	}
	if selEnd < len(line) {
		out.WriteString(render(line[selEnd:], false))
	}
	return out.String()
}

func selectionColumnsForLine(line string, row int, rng TextRange, hasSelection bool) (start, end int, selected bool) {
	if !hasSelection || row < rng.Start.Row || row > rng.End.Row {
		return 0, 0, false
	}
	switch {
	case rng.Start.Row == rng.End.Row:
		return rng.Start.Col, rng.End.Col, rng.Start.Col != rng.End.Col
	case row == rng.Start.Row:
		return rng.Start.Col, len(line), rng.Start.Col != len(line)
	case row == rng.End.Row:
		return 0, rng.End.Col, rng.End.Col != 0
	default:
		return 0, len(line), len(line) != 0
	}
}

func compareTextPosition(a, b TextPosition) int {
	if a.Row < b.Row {
		return -1
	}
	if a.Row > b.Row {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
}
