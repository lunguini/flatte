package flatui

import (
	"strings"
	"testing"
)

func TestTextareaInsertAndNewlineSplitsLines(t *testing.T) {
	var ta Textarea
	ta.Insert('a')
	ta.Insert('b')
	ta.InsertNewline()
	ta.Insert('c')
	ta.Insert('d')
	if ta.Value() != "ab\ncd" {
		t.Fatalf("Value() = %q, want \"ab\\ncd\"", ta.Value())
	}
	if ta.Row() != 1 || ta.Col() != 2 {
		t.Fatalf("cursor = (%d,%d), want (1,2)", ta.Row(), ta.Col())
	}
}

func TestTextareaInsertNewlineSplitsMidLine(t *testing.T) {
	var ta Textarea
	ta.SetValue("abcd")
	ta.MoveRight()
	ta.MoveRight() // between b and c
	ta.InsertNewline()
	if ta.Value() != "ab\ncd" {
		t.Fatalf("Value() = %q, want \"ab\\ncd\"", ta.Value())
	}
	if ta.Row() != 1 || ta.Col() != 0 {
		t.Fatalf("cursor = (%d,%d), want (1,0)", ta.Row(), ta.Col())
	}
}

func TestTextareaBackspaceMergesWithPreviousLine(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab\ncd")
	ta.MoveDown() // row 1, col 0
	ta.Backspace()
	if ta.Value() != "abcd" {
		t.Fatalf("Value() = %q, want abcd", ta.Value())
	}
	if ta.Row() != 0 || ta.Col() != 2 {
		t.Fatalf("cursor = (%d,%d), want (0,2)", ta.Row(), ta.Col())
	}
}

func TestTextareaDeleteMergesNextLine(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab\ncd")
	ta.MoveRight()
	ta.MoveRight() // end of "ab"
	ta.Delete()
	if ta.Value() != "abcd" {
		t.Fatalf("Value() = %q, want abcd", ta.Value())
	}
}

func TestTextareaHorizontalMoveWrapsAcrossLines(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab\ncd")
	ta.MoveDown() // row 1, col 0
	ta.MoveLeft() // wraps to end of previous line
	if ta.Row() != 0 || ta.Col() != 2 {
		t.Fatalf("MoveLeft wrap = (%d,%d), want (0,2)", ta.Row(), ta.Col())
	}
	ta.MoveRight() // wraps to start of next line
	if ta.Row() != 1 || ta.Col() != 0 {
		t.Fatalf("MoveRight wrap = (%d,%d), want (1,0)", ta.Row(), ta.Col())
	}
}

func TestTextareaVerticalMovePreservesGoalColumn(t *testing.T) {
	var ta Textarea
	ta.SetValue("hello\nhi\nworld")
	for range 5 {
		ta.MoveRight() // row 0, col 5 (end of "hello"); goal column = 5
	}
	ta.MoveDown() // "hi" is shorter; col clamps to 2 but goal stays 5
	if ta.Col() != 2 {
		t.Fatalf("col on short line = %d, want 2", ta.Col())
	}
	ta.MoveDown() // "world" is long enough; goal column 5 is restored
	if ta.Col() != 5 {
		t.Fatalf("col restored to goal = %d, want 5", ta.Col())
	}
}

func TestTextareaMovesByWordWithinAndAcrossLines(t *testing.T) {
	var ta Textarea
	ta.SetValue("hello, world\nnext line")
	for range len("hello, world") {
		ta.MoveRight()
	}

	ta.MoveWordLeft()
	if ta.Row() != 0 || ta.Col() != len("hello, ") {
		t.Fatalf("MoveWordLeft = (%d,%d), want start of world", ta.Row(), ta.Col())
	}
	ta.MoveWordRight()
	if ta.Row() != 0 || ta.Col() != len("hello, world") {
		t.Fatalf("MoveWordRight = (%d,%d), want end of world", ta.Row(), ta.Col())
	}
	ta.MoveRight() // row 1, col 0
	ta.MoveWordLeft()
	if ta.Row() != 0 || ta.Col() != len("hello, ") {
		t.Fatalf("MoveWordLeft across line = (%d,%d), want previous word start", ta.Row(), ta.Col())
	}
	ta.MoveWordRight()
	if ta.Row() != 0 || ta.Col() != len("hello, world") {
		t.Fatalf("MoveWordRight after cross-line left = (%d,%d), want previous line word end", ta.Row(), ta.Col())
	}
	ta.MoveWordRight()
	if ta.Row() != 1 || ta.Col() != len("next") {
		t.Fatalf("MoveWordRight across line = (%d,%d), want end of next", ta.Row(), ta.Col())
	}
}

func TestTextareaBackspaceRemovesWholeGraphemeCluster(t *testing.T) {
	var ta Textarea
	ta.SetValue("ábc") // a + combining acute = one cluster (3 bytes)
	ta.MoveRight()      // past the whole cluster
	if ta.Col() != 3 {
		t.Fatalf("MoveRight col = %d, want 3 (past cluster)", ta.Col())
	}
	ta.Backspace()
	if ta.Value() != "bc" {
		t.Fatalf("Value() = %q, want bc", ta.Value())
	}
}

func TestTextareaDeletesByWordWithinLine(t *testing.T) {
	var ta Textarea
	ta.SetValue("hello, world café")
	for range len("hello, world") {
		ta.MoveRight()
	}

	ta.DeleteWordLeft()
	if ta.Value() != "hello,  café" {
		t.Fatalf("DeleteWordLeft value = %q, want %q", ta.Value(), "hello,  café")
	}
	if ta.Row() != 0 || ta.Col() != len("hello, ") {
		t.Fatalf("DeleteWordLeft cursor = (%d,%d), want (0,%d)", ta.Row(), ta.Col(), len("hello, "))
	}

	ta.DeleteWordRight()
	if ta.Value() != "hello, " {
		t.Fatalf("DeleteWordRight value = %q, want %q", ta.Value(), "hello, ")
	}
	if ta.Row() != 0 || ta.Col() != len("hello, ") {
		t.Fatalf("DeleteWordRight cursor = (%d,%d), want (0,%d)", ta.Row(), ta.Col(), len("hello, "))
	}
}

func TestTextareaDeletesByWordAcrossLines(t *testing.T) {
	var ta Textarea
	ta.SetValue("hello world\nnext line")
	ta.MoveDown() // row 1, col 0

	ta.DeleteWordLeft()
	if ta.Value() != "hello next line" {
		t.Fatalf("DeleteWordLeft across line value = %q, want %q", ta.Value(), "hello next line")
	}
	if ta.Row() != 0 || ta.Col() != len("hello ") {
		t.Fatalf("DeleteWordLeft across line cursor = (%d,%d), want (0,%d)", ta.Row(), ta.Col(), len("hello "))
	}

	ta.SetValue("hello\nnext line")
	for range len("hello") {
		ta.MoveRight()
	}
	ta.DeleteWordRight()
	if ta.Value() != "hello line" {
		t.Fatalf("DeleteWordRight across line value = %q, want %q", ta.Value(), "hello line")
	}
	if ta.Row() != 0 || ta.Col() != len("hello") {
		t.Fatalf("DeleteWordRight across line cursor = (%d,%d), want (0,%d)", ta.Row(), ta.Col(), len("hello"))
	}
}

func TestTextareaSelectingMoveReportsRangeAndReplacesSelection(t *testing.T) {
	var ta Textarea
	ta.SetValue("abcdef")
	for range 2 {
		ta.MoveRight()
	}

	ta.MoveRightSelecting()
	ta.MoveRightSelecting()
	rng, ok := ta.Selection()
	if !ok || rng.Start != (TextPosition{Row: 0, Col: 2}) || rng.End != (TextPosition{Row: 0, Col: 4}) {
		t.Fatalf("Selection() = (%+v,%v), want row 0 cols 2..4", rng, ok)
	}
	if got := ta.SelectedText(); got != "cd" {
		t.Fatalf("SelectedText() = %q, want %q", got, "cd")
	}

	ta.Insert('X')
	if ta.Value() != "abXef" {
		t.Fatalf("Value after replacing selection = %q, want %q", ta.Value(), "abXef")
	}
	if ta.Row() != 0 || ta.Col() != 3 {
		t.Fatalf("cursor after replacing selection = (%d,%d), want (0,3)", ta.Row(), ta.Col())
	}
	if _, ok := ta.Selection(); ok {
		t.Fatal("selection still active after insert")
	}
}

func TestTextareaSelectionDeletesAcrossLines(t *testing.T) {
	var ta Textarea
	ta.SetValue("hello\nworld")
	for range 5 {
		ta.MoveRight()
	}

	ta.MoveRightSelecting() // selects the newline
	ta.MoveRightSelecting()
	ta.MoveRightSelecting() // selects "\nwo"
	if got := ta.SelectedText(); got != "\nwo" {
		t.Fatalf("SelectedText() = %q, want %q", got, "\nwo")
	}

	ta.Backspace()
	if ta.Value() != "hellorld" {
		t.Fatalf("Value after deleting multiline selection = %q, want %q", ta.Value(), "hellorld")
	}
	if ta.Row() != 0 || ta.Col() != 5 {
		t.Fatalf("cursor after deleting multiline selection = (%d,%d), want (0,5)", ta.Row(), ta.Col())
	}
}

func TestTextareaViewWithSelectionUsesRenderCallback(t *testing.T) {
	var ta Textarea
	ta.SetValue("abcdef")
	ta.SetSize(4, 3)
	ta.MoveRight()
	for range 3 {
		ta.MoveRightSelecting()
	}

	got := ta.ViewWithSelection(func(text string, selected bool) string {
		if selected {
			return "[" + text + "]"
		}
		return text
	})
	if got != "[bcd]e" {
		t.Fatalf("ViewWithSelection() = %q, want %q", got, "[bcd]e")
	}
}

func TestTextareaVerticalScrollKeepsCursorVisible(t *testing.T) {
	var ta Textarea
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = string(rune('0' + i))
	}
	ta.SetValue(strings.Join(lines, "\n"))
	ta.SetSize(20, 3)
	for range 5 {
		ta.MoveDown() // row 5
	}
	if ta.Offset() != 3 { // 5 - 3 + 1
		t.Fatalf("Offset() = %d, want 3", ta.Offset())
	}
}

func TestTextareaHorizontallyScrollsLongLineToKeepCursorVisible(t *testing.T) {
	var ta Textarea
	ta.SetValue("abcdefghij")
	ta.SetSize(4, 3)
	if got := ta.View(); got != "abcd" {
		t.Fatalf("initial View() = %q, want %q", got, "abcd")
	}

	for range 6 {
		ta.MoveRight()
	}
	if got := ta.View(); got != "defg" {
		t.Fatalf("View() after moving right = %q, want %q", got, "defg")
	}
	x, y := ta.CursorCell()
	if x != 3 || y != 0 {
		t.Fatalf("CursorCell() after moving right = (%d,%d), want (3,0)", x, y)
	}
}

func TestTextareaHorizontalScrollMovesBackLeft(t *testing.T) {
	var ta Textarea
	ta.SetValue("abcdefghij")
	ta.SetSize(4, 3)
	for range 6 {
		ta.MoveRight()
	}
	for range 5 {
		ta.MoveLeft()
	}
	if got := ta.View(); got != "bcde" {
		t.Fatalf("View() after moving back left = %q, want %q", got, "bcde")
	}
	x, y := ta.CursorCell()
	if x != 0 || y != 0 {
		t.Fatalf("CursorCell() after moving back left = (%d,%d), want (0,0)", x, y)
	}
}

func TestTextareaHorizontalWindowDoesNotSplitWideGrapheme(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab界cd")
	ta.SetSize(4, 3)
	for range 5 {
		ta.MoveRight()
	}
	if got := ta.View(); got != "cd" {
		t.Fatalf("View() after wide scroll = %q, want %q", got, "cd")
	}
	x, y := ta.CursorCell()
	if x != 2 || y != 0 {
		t.Fatalf("CursorCell() after wide scroll = (%d,%d), want (2,0)", x, y)
	}
}

func TestTextareaCursorCell(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab\ncd")
	ta.SetSize(20, 5)
	ta.MoveDown()
	ta.MoveRight() // row 1, col 1
	x, y := ta.CursorCell()
	if x != 1 || y != 1 {
		t.Fatalf("CursorCell() = (%d,%d), want (1,1)", x, y)
	}
}

func TestTextareaViewWindowsToHeight(t *testing.T) {
	var ta Textarea
	ta.SetValue("0\n1\n2\n3\n4")
	ta.SetSize(10, 2)
	if got := ta.View(); got != "0\n1" {
		t.Fatalf("View() = %q, want \"0\\n1\"", got)
	}
	for range 3 {
		ta.MoveDown() // row 3, offset 2
	}
	if got := ta.View(); got != "2\n3" {
		t.Fatalf("View() after scroll = %q, want \"2\\n3\"", got)
	}
}

func TestTextareaValueRoundTrips(t *testing.T) {
	var ta Textarea
	ta.SetValue("x\ny\nz")
	if ta.Value() != "x\ny\nz" {
		t.Fatalf("Value() = %q, want round-trip", ta.Value())
	}
}
