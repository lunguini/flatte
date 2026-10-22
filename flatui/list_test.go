package flatui

import (
	"fmt"
	"testing"

	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

// marker renders an item as "> i" when selected, "  i" otherwise — a stand-in
// for whatever appearance an app would choose.
func marker(i int, selected bool) string {
	if selected {
		return fmt.Sprintf("> %d", i)
	}
	return fmt.Sprintf("  %d", i)
}

func TestListStartsAtFirstItem(t *testing.T) {
	var l List
	l.SetCount(5)
	if l.Cursor() != 0 {
		t.Fatalf("Cursor() = %d, want 0", l.Cursor())
	}
	if l.Count() != 5 {
		t.Fatalf("Count() = %d, want 5", l.Count())
	}
}

func TestListMoveClampsToBounds(t *testing.T) {
	var l List
	l.SetCount(3)
	l.MoveUp()
	if l.Cursor() != 0 {
		t.Fatalf("MoveUp at top: Cursor() = %d, want 0", l.Cursor())
	}
	for range 5 {
		l.MoveDown()
	}
	if l.Cursor() != 2 {
		t.Fatalf("MoveDown past end: Cursor() = %d, want 2", l.Cursor())
	}
}

func TestListSelectClamps(t *testing.T) {
	var l List
	l.SetCount(4)
	l.Select(100)
	if l.Cursor() != 3 {
		t.Fatalf("Select(100): Cursor() = %d, want 3", l.Cursor())
	}
	l.Select(-5)
	if l.Cursor() != 0 {
		t.Fatalf("Select(-5): Cursor() = %d, want 0", l.Cursor())
	}
}

func TestListKeepsCursorVisibleScrollingDown(t *testing.T) {
	var l List
	l.SetCount(10)
	l.SetHeight(3)
	for range 4 { // cursor 0 -> 4
		l.MoveDown()
	}
	if l.Cursor() != 4 {
		t.Fatalf("Cursor() = %d, want 4", l.Cursor())
	}
	if l.Offset() != 2 { // window [2,5) keeps row 4 visible
		t.Fatalf("Offset() = %d, want 2", l.Offset())
	}
}

func TestListKeepsCursorVisibleScrollingUp(t *testing.T) {
	var l List
	l.SetCount(10)
	l.SetHeight(3)
	l.Select(9) // offset 7, window [7,10)
	if l.Offset() != 7 {
		t.Fatalf("after Select(9) Offset() = %d, want 7", l.Offset())
	}
	l.Select(1) // jump up; window must follow to keep row 1 visible
	if l.Offset() != 1 {
		t.Fatalf("after Select(1) Offset() = %d, want 1", l.Offset())
	}
}

func TestListShrinkClampsCursorAndOffset(t *testing.T) {
	var l List
	l.SetCount(10)
	l.SetHeight(3)
	l.Select(9) // cursor 9, offset 7
	l.SetCount(2)
	if l.Cursor() != 1 {
		t.Fatalf("after shrink Cursor() = %d, want 1", l.Cursor())
	}
	if l.Offset() != 0 {
		t.Fatalf("after shrink Offset() = %d, want 0", l.Offset())
	}
}

func TestListViewRendersVisibleWindowWithSelection(t *testing.T) {
	var l List
	l.SetCount(5)
	l.SetHeight(2)
	if got := l.View(marker); got != "> 0\n  1" {
		t.Fatalf("View() = %q, want \"> 0\\n  1\"", got)
	}
	l.Select(4) // offset 3, window [3,5)
	if got := l.View(marker); got != "  3\n> 4" {
		t.Fatalf("View() after Select(4) = %q, want \"  3\\n> 4\"", got)
	}
}

func TestListEmptyView(t *testing.T) {
	var l List
	l.SetHeight(3) // no items
	if got := l.View(marker); got != "" {
		t.Fatalf("empty View() = %q, want empty", got)
	}
}

// TestListRowsAreTheVisibleWindowAsNodes: Rows is View as nodes — the same
// window of items, each row ID'd for hit-testing and the selected one marked
// for a GUI's highlight — and composes to View's text.
func TestListRowsAreTheVisibleWindowAsNodes(t *testing.T) {
	var l List
	l.SetCount(10)
	l.SetHeight(3)
	l.Select(4) // window 2..4
	rows := l.Rows("items", func(i int, selected bool) layout.Node {
		return layout.Label{Text: marker(i, selected)}
	})
	if rows.ID != "" || len(rows.Children) != 3 {
		t.Fatalf("Rows = %+v, want 3 rows in an unnamed Col (the app names it)", rows)
	}
	for k, child := range rows.Children {
		row, ok := child.(layout.Row)
		i := l.Offset() + k
		if !ok || row.ID != fmt.Sprintf("items:%d", i) || row.Box.Selected != (i == l.Cursor()) {
			t.Fatalf("row %d = %+v, want ID items:%d selected=%v", k, child, i, i == l.Cursor())
		}
	}
	got, rects := layout.SolveAndCompose(rows, 5, 3)
	if want := l.View(marker); flatest.CleanFrame(got) != want {
		t.Fatalf("Rows composed %q, want View's %q", got, want)
	}
	if r := rects["items:3"]; r != (layout.Rect{Y: 1, W: 5, H: 1}) {
		t.Fatalf("items:3 rect = %+v", r)
	}
}

func TestListRowsEmpty(t *testing.T) {
	var l List
	l.SetHeight(3)
	called := false
	rows := l.Rows("x", func(int, bool) layout.Node { called = true; return layout.Label{} })
	if len(rows.Children) != 0 || called {
		t.Fatalf("empty list rows = %+v (row func called: %v)", rows, called)
	}
}

func TestListScrollTo(t *testing.T) {
	var l List
	l.SetCount(20)
	l.SetHeight(5)
	l.Select(2)
	l.ScrollTo(10)
	if l.Offset() != 10 || l.Cursor() != 10 {
		t.Fatalf("offset %d cursor %d, want 10 and the cursor pulled to 10", l.Offset(), l.Cursor())
	}
	l.Select(13)
	l.ScrollTo(99)
	if l.Offset() != 15 || l.Cursor() != 15 {
		t.Fatalf("offset %d cursor %d, want clamped to 15", l.Offset(), l.Cursor())
	}
	l.ScrollTo(12) // cursor 15 still visible: stays
	if l.Offset() != 12 || l.Cursor() != 15 {
		t.Fatalf("offset %d cursor %d, want 12 and 15", l.Offset(), l.Cursor())
	}
	l.ScrollTo(-4)
	if l.Offset() != 0 || l.Cursor() != 4 {
		t.Fatalf("offset %d cursor %d, want 0 and 4", l.Offset(), l.Cursor())
	}
}

func TestListScrollLeavesTheSelection(t *testing.T) {
	var l List
	l.SetCount(20)
	l.SetHeight(5)
	l.Select(2)
	l.Scroll(3)
	if l.Offset() != 3 || l.Cursor() != 2 {
		t.Fatalf("offset %d cursor %d, want 3 and 2", l.Offset(), l.Cursor())
	}
	// Resizing and relisting (as every layout pass does) keep the scroll.
	l.SetHeight(5)
	l.SetCount(20)
	if l.Offset() != 3 {
		t.Fatalf("offset %d after SetHeight/SetCount, want 3", l.Offset())
	}
	if rows := l.Rows("l", func(int, bool) layout.Node { return layout.Text{} }); rows.Children[0].(layout.Row).ID != "l:3" {
		t.Fatalf("first row %q", rows.Children[0].(layout.Row).ID)
	}
	l.Scroll(99)
	if l.Offset() != 15 || l.Cursor() != 2 {
		t.Fatalf("offset %d cursor %d, want clamped to 15 and 2", l.Offset(), l.Cursor())
	}
	l.Scroll(-99)
	if l.Offset() != 0 {
		t.Fatalf("offset %d, want 0", l.Offset())
	}
	// The next move brings the selection back into view.
	l.Scroll(10)
	l.MoveDown()
	if l.Cursor() != 3 || l.Offset() != 3 {
		t.Fatalf("offset %d cursor %d, want the window back on 3", l.Offset(), l.Cursor())
	}
}

func TestListSetHeightKeepsAVisibleSelectionVisible(t *testing.T) {
	var l List
	l.SetCount(20)
	l.SetHeight(10)
	l.Select(8)
	l.SetHeight(4)
	if l.Offset() != 5 || l.Cursor() != 8 {
		t.Fatalf("offset %d cursor %d, want the window moved onto 8", l.Offset(), l.Cursor())
	}
	l.SetCount(3) // the cursor clamps to the last item, shown
	if l.Cursor() != 2 || l.Offset() != 0 {
		t.Fatalf("offset %d cursor %d after shrinking", l.Offset(), l.Cursor())
	}
}
