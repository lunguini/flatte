package flatui

import (
	"slices"
	"testing"
)

func pinnedIDs(t Tree) []string {
	var ids []string
	for _, r := range t.Pinned() {
		ids = append(ids, r.ID)
	}
	return ids
}

// Pinned is the folders whose rows have scrolled above the window while
// rows inside them are still in it, outermost first, so an app can keep
// them at the top (sticky folders).
func TestTreePinnedAncestors(t *testing.T) {
	tr := NewTree([]TreeNode{
		{ID: "a", Label: "a", Children: []TreeNode{
			{ID: "a/b", Label: "b", Children: []TreeNode{
				{ID: "a/b/1", Label: "1"}, {ID: "a/b/2", Label: "2"}, {ID: "a/b/3", Label: "3"}, {ID: "a/b/4", Label: "4"},
			}},
			{ID: "a/c", Label: "c"},
		}},
		{ID: "d", Label: "d"}, {ID: "e", Label: "e"}, {ID: "f", Label: "f"},
	})
	tr.Toggle("a")
	tr.Toggle("a/b")
	// rows: a, a/b, 1, 2, 3, 4, a/c, d, e, f
	tr.SetHeight(4)
	for _, tc := range []struct {
		offset int
		want   []string
	}{
		{0, nil},                  // a is in view
		{1, []string{"a", "a/b"}}, // a scrolled off; a/b, under the first pin, pins too
		{2, []string{"a", "a/b"}},
		{3, []string{"a", "a/b"}},
		{4, []string{"a"}}, // a/b's last rows: a/c comes up under a
		{5, []string{"a"}},
		{6, nil}, // a/c at the top, then d outside a: nothing to pin
		{7, nil},
	} {
		tr.offset = tc.offset
		if got := pinnedIDs(tr); !slices.Equal(got, tc.want) {
			t.Errorf("offset %d: pinned %q, want %q", tc.offset, got, tc.want)
		}
	}
	// Pinned rows never fill the window: one row is always the tree's own.
	tr.SetHeight(2)
	tr.offset = 4
	if got := pinnedIDs(tr); len(got) > 1 {
		t.Errorf("height 2: pinned %q, want at most one", got)
	}
	tr.SetHeight(0)
	if got := pinnedIDs(tr); got != nil {
		t.Errorf("no height: %q", got)
	}
}
