package window

import (
	"strings"
)

// areaRow is one visual row of a multi-line field: bytes start..end of its
// text, and whether it ends a line (the next byte, if any, is "\n").
type areaRow struct {
	start, end int
	lineEnd    bool
}

// areaRows splits text into visual rows: at "\n", and, when wrap, by words
// at width as wrapWords wraps them (so a window wraps a text area as it
// wraps a Label).
func areaRows(text string, width float64, wrap bool, measure func(string) float64) []areaRow {
	var rows []areaRow
	at := 0
	for _, line := range strings.Split(text, "\n") {
		if !wrap {
			rows = append(rows, areaRow{start: at, end: at + len(line), lineEnd: true})
			at += len(line) + 1
			continue
		}
		// wrapWords' rows are consecutive substrings of the line, apart
		// from the one space dropped at each word break.
		segs := wrapWords(line, width, measure)
		pos := 0
		for i, seg := range segs {
			start := pos + strings.Index(line[pos:], seg)
			pos = start + len(seg)
			rows = append(rows, areaRow{start: at + start, end: at + pos, lineEnd: i == len(segs)-1})
		}
		at += len(line) + 1
	}
	return rows
}

// caretRow is the row the caret at byte c is on: the row containing it,
// or — at a wrap point shared by two rows — the later one.
func caretRow(rows []areaRow, c int) int {
	for i, r := range rows {
		if c < r.start || c > r.end {
			continue
		}
		if c == r.end && !r.lineEnd && i+1 < len(rows) && rows[i+1].start == c {
			continue
		}
		return i
	}
	return max(len(rows)-1, 0)
}

// firstRow is the first visible row of a window visible rows tall over
// total rows: the hint, moved as little as keeps caret in view.
func firstRow(hint, caret, visible, total int) int {
	visible = max(visible, 1)
	first := min(max(hint, caret-visible+1), caret)
	return max(min(first, total-visible), 0)
}
