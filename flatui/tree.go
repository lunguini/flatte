package flatui

import (
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
// dimension range.
func (t *Tree) SetHeight(h int) {
	t.height = clampWidgetDimension(h)
	t.clamp()
}

// SetNodes replaces the tree data, preserving expansion for IDs that still
// exist and still have children.
func (t *Tree) SetNodes(nodes []TreeNode) {
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
	t.clamp()
}

// Toggle expands or collapses the node with id. Unknown and leaf IDs are no-op.
func (t *Tree) Toggle(id string) {
	if !treeHasChildren(t.nodes, id) {
		return
	}
	t.ensureExpanded()
	if t.expanded[id] {
		delete(t.expanded, id)
	} else {
		t.expanded[id] = true
	}
	t.clamp()
}

// MoveUp / MoveDown move the cursor by one visible row, keeping it visible.
func (t *Tree) MoveUp()   { t.cursor--; t.clamp() }
func (t *Tree) MoveDown() { t.cursor++; t.clamp() }

// SelectVisible moves the cursor to the visible row index i.
func (t *Tree) SelectVisible(i int) {
	t.cursor = i
	t.clamp()
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
			NodeBase: layout.NodeBase{ID: id + ":" + r.ID, Box: layout.Box{Selected: i == cursor}},
			Children: []layout.Node{row(r, i == cursor)},
		})
	}
	return col
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

func (t *Tree) clamp() {
	rows := t.VisibleRows()
	t.cursor = min(max(t.cursor, 0), max(len(rows)-1, 0))
	if t.height > 0 {
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
