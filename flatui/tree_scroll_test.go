package flatui

import (
	"strconv"
	"testing"
)

func flatTree(n int) Tree {
	nodes := make([]TreeNode, n)
	for i := range nodes {
		nodes[i] = TreeNode{ID: strconv.Itoa(i), Label: strconv.Itoa(i)}
	}
	nodes[0].Children = []TreeNode{{ID: "0/a", Label: "a"}, {ID: "0/b", Label: "b"}}
	return NewTree(nodes)
}

// The wheel scrolls the window and leaves the cursor; moving the cursor
// brings its row back into view.
func TestTreeScrollKeepsTheCursor(t *testing.T) {
	tr := flatTree(30)
	tr.SetHeight(10)
	tr.MoveDown() // cursor on row 1
	tr.Scroll(5)
	if tr.Offset() != 5 || tr.CursorID() != "1" {
		t.Fatalf("offset %d cursor %q, want 5 and 1", tr.Offset(), tr.CursorID())
	}
	tr.Scroll(100)
	if tr.Offset() != 20 {
		t.Fatalf("scrolled past the end: offset %d", tr.Offset())
	}
	tr.Scroll(-100)
	if tr.Offset() != 0 {
		t.Fatalf("scrolled past the start: offset %d", tr.Offset())
	}
	tr.Scroll(15)
	// A hidden cursor stays hidden through resizes, data and expansion.
	tr.SetHeight(8)
	tr.SetNodes(flatTree(30).nodes)
	tr.Toggle("0")
	if tr.Offset() != 15+2 && tr.Offset() != 15 {
		t.Fatalf("hidden cursor pulled the window: offset %d", tr.Offset())
	}
	if tr.CursorID() != "1" {
		t.Fatalf("cursor moved to %q", tr.CursorID())
	}
	tr.MoveDown()
	if c := tr.cursor; c < tr.Offset() || c >= tr.Offset()+8 {
		t.Fatalf("moving the cursor did not reveal it: cursor %d offset %d", c, tr.Offset())
	}
	// A visible cursor stays visible when the window shrinks.
	tr.SetHeight(2)
	if c := tr.cursor; c < tr.Offset() || c >= tr.Offset()+2 {
		t.Fatalf("visible cursor lost on resize: cursor %d offset %d", c, tr.Offset())
	}
}

// Folding keeps the cursor on its item: rows opening above it do not move
// it, and folding away the folder it is in leaves it on that folder.
func TestTreeToggleKeepsTheCursorItem(t *testing.T) {
	tr := flatTree(5)
	tr.SetHeight(10)
	tr.MoveDown()
	tr.MoveDown() // "2"
	tr.Toggle("0")
	if tr.CursorID() != "2" {
		t.Fatalf("expanding above moved the cursor to %q", tr.CursorID())
	}
	tr.SelectVisible(2) // "0/b"
	if tr.CursorID() != "0/b" {
		t.Fatalf("setup: cursor %q", tr.CursorID())
	}
	tr.Toggle("0")
	if tr.CursorID() != "0" {
		t.Fatalf("folding the cursor's folder left it on %q", tr.CursorID())
	}
}
