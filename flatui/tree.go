package flatui

import (
	"slices"
	"strings"

	"github.com/lunguini/flatte/flatui/layout"
)

// TreeNode is one node in an app-owned tree.
type TreeNode struct {
	ID       string
	Label    string
	Children []TreeNode
}

// TreeRow is a visible tree row after expansion state has been applied.
type TreeRow struct {
	ID         string
	Label      string
	Depth      int
	Expandable bool
	Expanded   bool
}

// Tree is expandable tree state. The app owns it and decides key policy,
// styling, and node data; Tree only tracks expansion, cursor, and scroll.
type Tree struct {
	nodes    []TreeNode
	expanded map[string]bool
	cursor   int
	offset   int
	height   int
}

// NewTree returns a tree with the given nodes.
func NewTree(nodes []TreeNode) Tree {
	return Tree{nodes: cloneTreeNodes(nodes), expanded: map[string]bool{}}
}

// SetHeight sets the number of visible rows, clamped to the supported widget
// dimension range; a visible cursor stays visible.
func (t *Tree) SetHeight(h int) {
	shown := t.cursorShown()
	t.height = clampWidgetDimension(h)
	t.clamp(shown)
}

// SetNodes replaces the tree data, preserving expansion for IDs that still
// exist and still have children. A visible cursor stays visible.
func (t *Tree) SetNodes(nodes []TreeNode) {
	shown := t.cursorShown()
	t.nodes = cloneTreeNodes(nodes)
	expandable := map[string]bool{}
	collectExpandable(expandable, t.nodes)
	nextExpanded := map[string]bool{}
	for id, expanded := range t.expanded {
		if expanded && expandable[id] {
			nextExpanded[id] = true
		}
	}
	t.expanded = nextExpanded
	t.clamp(shown)
}

// Toggle expands or collapses the node with id. Unknown and leaf IDs are
// no-op. A visible cursor stays visible.
func (t *Tree) Toggle(id string) {
	if !treeHasChildren(t.nodes, id) {
		return
	}
	shown, selected := t.cursorShown(), t.CursorID()
	t.ensureExpanded()
	if t.expanded[id] {
		delete(t.expanded, id)
	} else {
		t.expanded[id] = true
	}
	// The cursor stays on its item as rows open or close above it; an item
	// folded away leaves it on the folder.
	rows := t.VisibleRows()
	for i, r := range rows {
		if r.ID == selected {
			t.cursor = i
			break
		}
		if r.ID == id && !t.expanded[id] && treeContains(t.nodes, id, selected) {
			t.cursor = i
			break
		}
	}
	t.clamp(shown)
}

// treeContains reports whether the node with id has a descendant with
// the ID want.
func treeContains(nodes []TreeNode, id, want string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return treeHas(n.Children, want)
		}
		if treeContains(n.Children, id, want) {
			return true
		}
	}
	return false
}

func treeHas(nodes []TreeNode, want string) bool {
	for _, n := range nodes {
		if n.ID == want || treeHas(n.Children, want) {
			return true
		}
	}
	return false
}

// MoveUp / MoveDown move the cursor by one visible row, keeping it visible.
func (t *Tree) MoveUp()   { t.cursor--; t.clamp(true) }
func (t *Tree) MoveDown() { t.cursor++; t.clamp(true) }

// SelectVisible moves the cursor to the visible row index i, keeping it
// visible.
func (t *Tree) SelectVisible(i int) {
	t.cursor = i
	t.clamp(true)
}

// Scroll moves the visible window by rows (negative: up), clamped, leaving
// the cursor where it is — for a mouse wheel. The cursor's row may scroll
// out of view; moving the cursor brings it back.
func (t *Tree) Scroll(rows int) {
	t.offset += rows
	t.clamp(false)
}

// CursorID is the ID of the selected visible row, or "" when the tree is empty.
func (t Tree) CursorID() string {
	rows := t.VisibleRows()
	if len(rows) == 0 {
		return ""
	}
	cursor := min(max(t.cursor, 0), len(rows)-1)
	return rows[cursor].ID
}

// Offset is the index of the first visible row.
func (t Tree) Offset() int { return t.offset }

// VisibleRows returns all rows visible under the current expansion state.
func (t Tree) VisibleRows() []TreeRow {
	rows := []TreeRow{}
	t.appendVisibleRows(&rows, t.nodes, 0)
	return rows
}

// View renders the visible window of rows. The app decides each row's marker,
// indentation, and styling through render.
func (t Tree) View(render func(row TreeRow, selected bool) string) string {
	rows, cursor := t.window()
	out := make([]string, 0, len(rows))
	for i, row := range rows {
		if render != nil {
			out = append(out, render(row, i == cursor))
		} else {
			out = append(out, row.Label)
		}
	}
	return strings.Join(out, "\n")
}

// Rows returns the visible window of rows as nodes, for a Frame.Root: row is
// called for each visible row with whether it is the cursor row, and each
// result is wrapped in a Row with ID "id:<node ID>" (stable as the tree
// expands, so MouseEvent.Target names the clicked node) whose Box is marked
// Selected for the cursor row. The Col is the app's to name and size.
func (t Tree) Rows(id string, row func(row TreeRow, selected bool) layout.Node) layout.Col {
	rows, cursor := t.window()
	var col layout.Col
	for i, r := range rows {
		col.Children = append(col.Children, layout.Row{
			NodeBase: layout.NodeBase{ID: id + ":" + r.ID, Box: layout.Box{Selected: i == cursor, Interactive: true, Round: true}},
			Children: []layout.Node{row(r, i == cursor)},
		})
	}
	return col
}

// Pinned is the folders to keep at the top of the window (sticky folders):
// the ancestors, outermost first, of the rows at the top of the window
// whose own rows have scrolled above it. An app draws them over the
// window's first len(Pinned()) rows, which they hide. One row of the window
// is always left to the tree's own rows; none before a height is set.
func (t Tree) Pinned() []TreeRow {
	rows := t.VisibleRows()
	if t.height <= 0 || len(rows) == 0 {
		return nil
	}
	off := min(max(t.offset, 0), max(len(rows)-t.height, 0))
	most := min(t.height-1, len(rows)-off-1)
	// above is the ancestors of the row n below the window's top that are
	// above that row's slot, outermost first.
	above := func(n int) []TreeRow {
		i := off + n
		var out []TreeRow
		for depth, j := rows[i].Depth, i-1; depth > 0 && j >= 0; j-- {
			if rows[j].Depth == depth-1 {
				out = append(out, rows[j])
				depth--
			}
		}
		slices.Reverse(out)
		return out
	}
	n := min(len(above(0)), most)
	if n == 0 {
		return nil
	}
	// Grow while the row below the pins has more folders above it, then
	// give way to a folder's end: the pins are the outermost n of the
	// folders of the row they sit on.
	for n < most {
		if f := len(above(n)); f > n {
			n = min(f, most)
			continue
		}
		break
	}
	for n > 0 && len(above(n)) < n {
		n--
	}
	if n == 0 {
		return nil
	}
	return above(n)[:n]
}

// window returns the visible rows and the cursor's index among them (-1
// when it is outside the window).
func (t Tree) window() ([]TreeRow, int) {
	rows := t.VisibleRows()
	if t.height <= 0 || len(rows) == 0 {
		return nil, -1
	}
	offset := min(max(t.offset, 0), max(len(rows)-t.height, 0))
	cursor := min(max(t.cursor, 0), len(rows)-1)
	end := min(offset+t.height, len(rows))
	return rows[offset:end], cursor - offset
}

func (t Tree) appendVisibleRows(rows *[]TreeRow, nodes []TreeNode, depth int) {
	for _, node := range nodes {
		expandable := len(node.Children) > 0
		expanded := expandable && t.expanded[node.ID]
		*rows = append(*rows, TreeRow{
			ID:         node.ID,
			Label:      node.Label,
			Depth:      depth,
			Expandable: expandable,
			Expanded:   expanded,
		})
		if expanded {
			t.appendVisibleRows(rows, node.Children, depth+1)
		}
	}
}

// cursorShown reports whether the cursor's row is in the window (always,
// before a height is set).
func (t Tree) cursorShown() bool {
	return t.height <= 0 || t.cursor >= t.offset && t.cursor < t.offset+t.height
}

// clamp keeps the cursor and the window in range, and with reveal the
// window on the cursor.
func (t *Tree) clamp(reveal bool) {
	rows := t.VisibleRows()
	t.cursor = min(max(t.cursor, 0), max(len(rows)-1, 0))
	if reveal && t.height > 0 {
		if t.cursor < t.offset {
			t.offset = t.cursor
		} else if t.cursor >= t.offset+t.height {
			t.offset = t.cursor - t.height + 1
		}
	}
	t.offset = min(max(t.offset, 0), max(len(rows)-t.height, 0))
}

func (t *Tree) ensureExpanded() {
	if t.expanded == nil {
		t.expanded = map[string]bool{}
	}
}

func cloneTreeNodes(nodes []TreeNode) []TreeNode {
	out := make([]TreeNode, len(nodes))
	for i, node := range nodes {
		out[i] = TreeNode{
			ID:       node.ID,
			Label:    node.Label,
			Children: cloneTreeNodes(node.Children),
		}
	}
	return out
}

func collectExpandable(out map[string]bool, nodes []TreeNode) {
	for _, node := range nodes {
		if len(node.Children) > 0 {
			out[node.ID] = true
			collectExpandable(out, node.Children)
		}
	}
}

func treeHasChildren(nodes []TreeNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return len(node.Children) > 0
		}
		if treeHasChildren(node.Children, id) {
			return true
		}
	}
	return false
}
