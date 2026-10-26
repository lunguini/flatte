package flatui

import (
	"github.com/lunguini/flatte/flatui/layout"
	"testing"
)

func TestStandardItemRowsHaveInteractivePresentation(t *testing.T) {
	var list List
	list.SetCount(2)
	list.SetHeight(2)
	table := sampleTable()
	tree := NewTree([]TreeNode{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}})
	tree.SetHeight(2)
	for _, rows := range []layout.Col{
		list.Rows("list", func(int, bool) layout.Node { return layout.Label{Text: "item"} }),
		table.Rows("table", func([]string, bool) layout.Node { return layout.Label{Text: "item"} }),
		tree.Rows("tree", func(TreeRow, bool) layout.Node { return layout.Label{Text: "item"} }),
	} {
		for _, node := range rows.Children {
			item := node.(layout.Row)
			if !item.Box.Interactive || !item.Box.Round {
				t.Fatalf("no default item presentation: %+v", item)
			}
		}
	}
	if list.Cursor() != 0 || table.Cursor() != 0 {
		t.Fatal("render changed selection")
	}
}
