package flatui

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestTreeVisibleRowsAndToggle(t *testing.T) {
	tr := NewTree([]TreeNode{
		{ID: "root", Label: "root", Children: []TreeNode{
			{ID: "api", Label: "api", Children: []TreeNode{{ID: "routes", Label: "routes"}}},
			{ID: "ui", Label: "ui"},
		}},
	})
	tr.SetHeight(10)
	tr.Toggle("root")
	if got := labels(tr.VisibleRows()); strings.Join(got, ",") != "root,api,ui" {
		t.Fatalf("visible after root expand = %v", got)
	}
	tr.Toggle("api")
	if got := labels(tr.VisibleRows()); strings.Join(got, ",") != "root,api,routes,ui" {
		t.Fatalf("visible after api expand = %v", got)
	}
}

func TestTreeCursorKeepsVisible(t *testing.T) {
	tr := NewTree([]TreeNode{
		{ID: "root", Label: "root", Children: []TreeNode{
			{ID: "a", Label: "a"},
			{ID: "b", Label: "b"},
			{ID: "c", Label: "c"},
		}},
	})
	tr.Toggle("root")
	tr.SetHeight(2)
	tr.MoveDown()
	tr.MoveDown()
	if tr.CursorID() != "b" {
		t.Fatalf("CursorID() = %q, want b", tr.CursorID())
	}
	if tr.Offset() != 1 {
		t.Fatalf("Offset() = %d, want 1", tr.Offset())
	}
}

func labels(rows []TreeRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Label
	}
	return out
}

// TestTreeRowsAreTheVisibleWindowAsNodes: Rows is View as nodes, each row
// ID'd by its node's ID (stable as the tree expands) and the cursor row
// marked Selected.
func TestTreeRowsAreTheVisibleWindowAsNodes(t *testing.T) {
	tr := NewTree([]TreeNode{
		{ID: "a", Label: "a", Children: []TreeNode{{ID: "a1", Label: "a1"}, {ID: "a2", Label: "a2"}}},
		{ID: "b", Label: "b"},
	})
	tr.Toggle("a")
	tr.SetHeight(2)
	tr.MoveDown()
	tr.MoveDown() // cursor on a2; window a1..a2
	render := func(row TreeRow, selected bool) string {
		if selected {
			return "> " + row.Label
		}
		return "  " + row.Label
	}
	rows := tr.Rows("t", func(row TreeRow, selected bool) layout.Node {
		return layout.Label{Text: render(row, selected)}
	})
	if len(rows.Children) != 2 {
		t.Fatalf("Rows = %+v, want the 2 visible rows", rows)
	}
	for k, want := range []struct {
		id  string
		sel bool
	}{{"t:a1", false}, {"t:a2", true}} {
		r := rows.Children[k].(layout.Row)
		if r.ID != want.id || r.Box.Selected != want.sel {
			t.Fatalf("row %d = %q selected=%v, want %q %v", k, r.ID, r.Box.Selected, want.id, want.sel)
		}
	}
	got, _ := layout.SolveAndCompose(rows, 6, 2)
	if want := tr.View(render); flatest.CleanFrame(got) != want {
		t.Fatalf("Rows composed %q, want View %q", got, want)
	}
}
