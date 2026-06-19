package flatui

import (
	"strings"
	"testing"
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
