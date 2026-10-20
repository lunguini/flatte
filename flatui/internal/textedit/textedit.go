// Package textedit lays out editable text in terminal cells: soft wrapping
// by grapheme clusters, horizontal windows, selection runs, and the scroll
// that keeps a cursor visible. flatui.Textarea and the multi-line
// layout.Input share it, so the widget and the node cannot render
// differently.
package textedit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

// ByteOffsetForColumn returns the byte offset of the grapheme boundary in line
// whose display column is the greatest not exceeding col (clamped to len).
func ByteOffsetForColumn(line string, col int) int {
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

func HorizontalWindowLines(lines []string, offset, width int) string {
	if width <= 0 && offset <= 0 {
		return strings.Join(lines, "\n")
	}
	windowed := make([]string, len(lines))
	for i, line := range lines {
		windowed[i] = HorizontalWindowLine(line, offset, width)
	}
	return strings.Join(windowed, "\n")
}

func HorizontalWindowLine(line string, offset, width int) string {
	return HorizontalWindowLineWithSelection(line, offset, width, 0, 0, false, func(text string, _ bool) string {
		return text
	})
}

func VisibleWrappedLines(lines []string, width, height, offset int) string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, SoftWrapLine(line, width)...)
	}
	if height <= 0 {
		return strings.Join(wrapped, "\n")
	}
	start := min(max(offset, 0), len(wrapped))
	end := min(start+height, len(wrapped))
	return strings.Join(wrapped[start:end], "\n")
}

func SoftWrapLine(line string, width int) []string {
	return SoftWrapLineWithSelection(line, width, 0, 0, false, func(text string, _ bool) string {
		return text
	})
}

func SoftWrapLineWithSelection(line string, width int, selStart, selEnd int, hasSelection bool, render func(text string, selected bool) string) []string {
	if width <= 0 {
		return []string{RenderLineWithSelection(line, selStart, selEnd, hasSelection, render)}
	}
	if line == "" {
		return []string{render("", hasSelection && selStart == 0 && selEnd == 0)}
	}
	rows := make([]string, 0, max(lipgloss.Width(line)/width, 1))
	state := -1
	rest := line
	at := 0
	rowWidth := 0
	var row strings.Builder
	var segment strings.Builder
	segmentSelected := false
	haveSegment := false
	flushSegment := func() {
		if !haveSegment {
			return
		}
		row.WriteString(render(segment.String(), segmentSelected))
		segment.Reset()
		haveSegment = false
	}
	flushRow := func() {
		flushSegment()
		rows = append(rows, row.String())
		row.Reset()
		rowWidth = 0
	}
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		clusterWidth := lipgloss.Width(cluster)
		if rowWidth > 0 && rowWidth+clusterWidth > width {
			flushRow()
		}
		selected := hasSelection && at >= selStart && at+len(cluster) <= selEnd
		if haveSegment && selected != segmentSelected {
			flushSegment()
		}
		segment.WriteString(cluster)
		segmentSelected = selected
		haveSegment = true
		rowWidth += clusterWidth
		at += len(cluster)
		rest, state = r, st
	}
	flushRow()
	return rows
}

func WrappedCursorCell(line string, col, width int) (x, y int) {
	if width <= 0 {
		return lipgloss.Width(line[:col]), 0
	}
	state := -1
	rest := line[:col]
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		clusterWidth := lipgloss.Width(cluster)
		if x > 0 && x+clusterWidth > width {
			y++
			x = 0
		}
		x += clusterWidth
		rest, state = r, st
	}
	if x == width && col < len(line) {
		return 0, y + 1
	}
	return x, y
}

func VisualCursorRow(lines []string, row, col, width int) int {
	_, wrapY := WrappedCursorCell(lines[row], col, width)
	return VisualRowsBefore(lines, row, width) + wrapY
}

func VisualRowsBefore(lines []string, row, width int) int {
	total := 0
	for i := 0; i < row && i < len(lines); i++ {
		total += len(SoftWrapLine(lines[i], width))
	}
	return total
}

func TotalWrappedRows(lines []string, width int) int {
	return VisualRowsBefore(lines, len(lines), width)
}

func PositionForVisualCell(lines []string, visualRow, col, width int) (row, byteCol int) {
	if len(lines) == 0 {
		return 0, 0
	}
	remaining := max(visualRow, 0)
	for row, line := range lines {
		rows := len(SoftWrapLine(line, width))
		if remaining < rows {
			return row, ByteOffsetForWrappedCell(line, remaining, col, width)
		}
		remaining -= rows
	}
	last := len(lines) - 1
	return last, len(lines[last])
}

func ByteOffsetForWrappedCell(line string, targetRow, col, width int) int {
	if width <= 0 || targetRow <= 0 {
		return ByteOffsetForColumn(line, col)
	}
	if line == "" {
		return 0
	}
	state := -1
	rest := line
	row, display := 0, 0
	at, lastInTarget := 0, 0
	for len(rest) > 0 {
		cluster, r, _, st := uniseg.StepString(rest, state)
		cw := lipgloss.Width(cluster)
		if display > 0 && display+cw > width {
			if row == targetRow {
				return at
			}
			row++
			display = 0
		}
		if row == targetRow {
			if display+cw > col {
				return at
			}
			lastInTarget = at + len(cluster)
		}
		display += cw
		at += len(cluster)
		rest, state = r, st
	}
	if row == targetRow {
		return lastInTarget
	}
	return len(line)
}

func HorizontalWindowLineWithSelection(line string, offset, width int, selStart, selEnd int, hasSelection bool, render func(text string, selected bool) string) string {
	if width <= 0 {
		return RenderLineWithSelection(line, selStart, selEnd, hasSelection, render)
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

func DisplayColumnAtOrAfter(line string, col int) int {
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

func RenderLineWithSelection(line string, selStart, selEnd int, hasSelection bool, render func(text string, selected bool) string) string {
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

// Scroll is a text window's scroll state: the first visible line (without
// wrapping), the first visible display column (without wrapping), and the
// first visible visual row (with wrapping).
type Scroll struct{ Offset, XOffset, WrapOffset int }

// Area is lines shown in a window width×height cells (0: unbounded), soft
// wrapped or not, with the cursor at (Row, Col), Col a byte offset.
type Area struct {
	Lines         []string
	Row, Col      int
	Width, Height int
	Wrap          bool
}

// KeepVisible returns s scrolled so the cursor's row and column are inside
// the window, moving it as little as possible.
func (a Area) KeepVisible(s Scroll) Scroll {
	if a.Wrap {
		s.XOffset = 0
		visualRow := VisualCursorRow(a.Lines, a.Row, a.Col, a.Width)
		if a.Height > 0 {
			if visualRow < s.WrapOffset {
				s.WrapOffset = visualRow
			} else if visualRow >= s.WrapOffset+a.Height {
				s.WrapOffset = visualRow - a.Height + 1
			}
			s.WrapOffset = min(max(s.WrapOffset, 0), max(TotalWrappedRows(a.Lines, a.Width)-a.Height, 0))
		} else {
			s.WrapOffset = 0
		}
		s.Offset = keepRow(s.Offset, a.Row, a.Height, len(a.Lines))
		return s
	}
	s.WrapOffset = 0
	s.Offset = keepRow(s.Offset, a.Row, a.Height, len(a.Lines))
	if a.Width <= 0 {
		s.XOffset = 0
		return s
	}
	line := a.Lines[a.Row]
	cursorCol := lipgloss.Width(line[:a.Col])
	if cursorCol < s.XOffset {
		s.XOffset = cursorCol
	} else if cursorCol >= s.XOffset+a.Width {
		s.XOffset = DisplayColumnAtOrAfter(line, cursorCol-a.Width+1)
	}
	s.XOffset = min(max(s.XOffset, 0), lipgloss.Width(line))
	return s
}

// keepRow is the first visible line keeping row in a window height lines
// tall (0: unbounded, offset unchanged).
func keepRow(offset, row, height, lines int) int {
	if height <= 0 {
		return offset
	}
	if row < offset {
		offset = row
	} else if row >= offset+height {
		offset = row - height + 1
	}
	return min(max(offset, 0), max(lines-height, 0))
}

// CursorCell is the cursor's cell relative to the window scrolled by s.
func (a Area) CursorCell(s Scroll) (x, y int) {
	if len(a.Lines) == 0 {
		return 0, 0
	}
	row := min(max(a.Row, 0), len(a.Lines)-1)
	col := min(max(a.Col, 0), len(a.Lines[row]))
	if a.Wrap {
		wrapX, wrapY := WrappedCursorCell(a.Lines[row], col, a.Width)
		return wrapX, VisualRowsBefore(a.Lines, row, a.Width) + wrapY - s.WrapOffset
	}
	return max(lipgloss.Width(a.Lines[row][:col])-s.XOffset, 0), row - s.Offset
}

// Render returns the window scrolled by s, one row per line. selection
// reports the selected byte range of a line (ok false: none); render styles
// each run of selected or unselected text.
func (a Area) Render(s Scroll, selection func(row int) (start, end int, ok bool), render func(text string, selected bool) string) string {
	if len(a.Lines) == 0 {
		return ""
	}
	if a.Wrap {
		lines := make([]string, 0, len(a.Lines))
		for row, line := range a.Lines {
			selStart, selEnd, selected := selection(row)
			lines = append(lines, SoftWrapLineWithSelection(line, a.Width, selStart, selEnd, selected, render)...)
		}
		if a.Height <= 0 {
			return strings.Join(lines, "\n")
		}
		start := min(max(s.WrapOffset, 0), len(lines))
		end := min(start+a.Height, len(lines))
		return strings.Join(lines[start:end], "\n")
	}
	startRow, endRow := 0, len(a.Lines)
	if a.Height > 0 {
		startRow = min(max(s.Offset, 0), len(a.Lines))
		endRow = min(startRow+a.Height, len(a.Lines))
	}
	lines := make([]string, 0, endRow-startRow)
	for row := startRow; row < endRow; row++ {
		selStart, selEnd, selected := selection(row)
		lines = append(lines, HorizontalWindowLineWithSelection(a.Lines[row], s.XOffset, a.Width, selStart, selEnd, selected, render))
	}
	return strings.Join(lines, "\n")
}
