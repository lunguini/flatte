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
