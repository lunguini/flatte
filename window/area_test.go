package window

import "testing"

func TestAreaRows(t *testing.T) {
	text := "the quick brown\n\nabcdefgh"
	rows := areaRows(text, 9, true, runeWidth)
	var got []string
	for _, r := range rows {
		got = append(got, text[r.start:r.end])
	}
	want := []string{"the quick", "brown", "", "abcdefgh"}
	if len(got) != len(want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows %q, want %q", got, want)
		}
	}
	if rows[0].lineEnd || !rows[1].lineEnd || !rows[2].lineEnd {
		t.Fatalf("line ends wrong: %+v", rows)
	}
	// A long word broken by runes: its rows share their boundary.
	long := areaRows("abcdefghij", 4, true, runeWidth)
	if len(long) != 3 || long[0].end != 4 || long[1].start != 4 || long[2].end != 10 {
		t.Fatalf("broken word rows %+v", long)
	}
	// Unwrapped: one row per line.
	if n := len(areaRows(text, 3, false, runeWidth)); n != 3 {
		t.Fatalf("unwrapped rows %d, want 3", n)
	}
}

func TestCaretRow(t *testing.T) {
	text := "the quick brown\nx"
	rows := areaRows(text, 9, true, runeWidth)
	for _, c := range []struct{ at, row int }{
		{0, 0}, {9, 0}, // after "quick", on the dropped space: still row 0
		{10, 1}, {15, 1}, // "brown" and the end of its line
		{16, 2}, {17, 2},
	} {
		if got := caretRow(rows, c.at); got != c.row {
			t.Errorf("caret %d on row %d, want %d", c.at, got, c.row)
		}
	}
	// At a shared boundary of a broken word, the caret starts the next row.
	long := areaRows("abcdefghij", 4, true, runeWidth)
	if got := caretRow(long, 4); got != 1 {
		t.Errorf("caret at a broken word's boundary on row %d, want 1", got)
	}
	if got := caretRow(long, 10); got != 2 {
		t.Errorf("caret at the end on row %d, want 2", got)
	}
}

func TestFirstRow(t *testing.T) {
	for _, c := range []struct{ hint, caret, visible, total, want int }{
		{0, 0, 3, 10, 0},
		{0, 5, 3, 10, 3},  // caret below: scroll so it is the last row
		{6, 2, 3, 10, 2},  // caret above: it becomes the first row
		{4, 5, 3, 10, 4},  // in view: the hint stays
		{9, 9, 3, 10, 7},  // never past the end
		{0, 0, 0, 1, 0},   // no room still shows the caret's row
		{-3, 0, 3, 10, 0}, // never before the start
	} {
		if got := firstRow(c.hint, c.caret, c.visible, c.total); got != c.want {
			t.Errorf("firstRow(%d, %d, %d, %d) = %d, want %d", c.hint, c.caret, c.visible, c.total, got, c.want)
		}
	}
}
