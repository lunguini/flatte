package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/cmd/internal/brand"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusSearch
	focusDetails
)

type State struct {
	focus flatui.FocusRing
	// tree is the whole tree as the user browses it; results is the tree of
	// search matches, shown instead while the query is non-empty, so
	// clearing the search returns to the tree exactly as it was.
	tree      flatui.Tree
	results   flatui.Tree
	searching bool
	rows      int // the tree pane's height, for both trees
	search    flatui.TextField
	details   flatui.Viewport
}

// activeTree is the tree on screen: the results while searching.
func (s *State) activeTree() *flatui.Tree {
	if s.searching {
		return &s.results
	}
	return &s.tree
}

func NewState() *State {
	s := &State{tree: flatui.NewTree(treeNodes())}
	s.focus.SetCount(3)
	s.tree.Toggle("workspace")
	// Until the first layout reports the panes' sizes.
	s.rows = 5
	s.tree.SetHeight(s.rows)
	s.details.SetSize(20, 5)
	s.syncDetails()
	return s
}

func treeNodes() []flatui.TreeNode {
	return []flatui.TreeNode{
		{ID: "workspace", Label: "workspace", Children: []flatui.TreeNode{
			{ID: "flatui", Label: "flatui", Children: []flatui.TreeNode{
				{ID: "tree", Label: "tree.go"},
				{ID: "viewport", Label: "viewport.go"},
				{ID: "textfield", Label: "textfield.go"},
			}},
			{ID: "cmd", Label: "cmd", Children: []flatui.TreeNode{
				{ID: "flat-tree", Label: "flat-tree"},
				{ID: "flat-style", Label: "flat-style"},
			}},
			{ID: "docs", Label: ".docs"},
		}},
	}
}

// The panes' bodies are ID'd nodes: their solved sizes size the tree and the
// details viewport.
const (
	treeBodyID    = "tree.rows"
	detailsBodyID = "details.body"
)

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := ev.Rects[treeBodyID]; ok {
			s.rows = max(r.H, 1)
			s.tree.SetHeight(s.rows)
			s.results.SetHeight(s.rows)
		}
		if r, ok := ev.Rects[detailsBodyID]; ok {
			s.details.SetSize(max(r.W, 1), max(r.H, 1))
			s.syncDetails()
		}
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if key.Key == flatte.KeyTab {
		if key.Mod.Contains(flatte.ModShift) {
			s.focus.Prev()
		} else {
			s.focus.Next()
		}
		return
	}

	switch focusArea(s.focus.Index()) {
	case focusTree:
		handleTreeKey(s, key, fx)
	case focusSearch:
		handleSearchKey(s, key, fx)
	case focusDetails:
		handleDetailsKey(s, key, fx)
	}
}

func handleTreeKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	tree := s.activeTree()
	switch key.Key {
	case flatte.KeyDown:
		tree.MoveDown()
	case flatte.KeyUp:
		tree.MoveUp()
	case flatte.KeyEnter:
		tree.Toggle(tree.CursorID())
	case flatte.KeyLeft:
		collapseOrParent(tree)
	case flatte.KeyRight:
		expandOrChild(tree)
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j':
			tree.MoveDown()
		case 'k':
			tree.MoveUp()
		case 'h':
			collapseOrParent(tree)
		case 'l':
			expandOrChild(tree)
		case ' ':
			tree.Toggle(tree.CursorID())
		case 'q':
			fx.Quit()
		}
	}
	s.syncDetails()
}

// cursorRow returns the visible rows and the cursor's index among them.
func cursorRow(t *flatui.Tree) ([]flatui.TreeRow, int) {
	rows, id := t.VisibleRows(), t.CursorID()
	for i, r := range rows {
		if r.ID == id {
			return rows, i
		}
	}
	return rows, -1
}

// expandOrChild opens a closed node, or steps into an open one's first
// child; on a leaf it does nothing.
func expandOrChild(t *flatui.Tree) {
	rows, i := cursorRow(t)
	if i < 0 || !rows[i].Expandable {
		return
	}
	if !rows[i].Expanded {
		t.Toggle(rows[i].ID)
		return
	}
	t.SelectVisible(i + 1) // an open node's first child is the next row
}

// collapseOrParent closes an open node, or steps out to the parent of a
// closed node or leaf; at a closed root it does nothing.
func collapseOrParent(t *flatui.Tree) {
	rows, i := cursorRow(t)
	if i < 0 {
		return
	}
	if rows[i].Expanded {
		t.Toggle(rows[i].ID)
		return
	}
	for p := i - 1; p >= 0; p-- { // the parent is the nearest shallower row above
		if rows[p].Depth < rows[i].Depth {
			t.SelectVisible(p)
			return
		}
	}
}

func handleSearchKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyLeft:
		s.search.MoveLeft()
	case flatte.KeyRight:
		s.search.MoveRight()
	case flatte.KeyBackspace:
		s.search.Backspace()
	case flatte.KeyDelete:
		s.search.Delete()
	case flatte.KeyCharacter:
		s.search.Insert(key.Rune)
	}
	s.syncSearch()
	s.syncDetails()
}

func handleDetailsKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		s.details.LineDown(1)
	case flatte.KeyUp:
		s.details.LineUp(1)
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j':
			s.details.LineDown(1)
		case 'k':
			s.details.LineUp(1)
		case 'q':
			fx.Quit()
		}
	}
}

// syncSearch rebuilds the results for the current query.
func (s *State) syncSearch() {
	query := strings.ToLower(strings.TrimSpace(s.search.Value))
	s.searching = query != ""
	if !s.searching {
		return
	}
	matches := filterNodes(treeNodes(), query)
	s.results = flatui.NewTree(matches)
	s.results.SetHeight(s.rows)
	expandAll(&s.results, matches)
	for i, row := range s.results.VisibleRows() {
		if strings.Contains(strings.ToLower(row.Label), query) {
			s.results.SelectVisible(i)
			break
		}
	}
}

// filterNodes keeps the nodes whose label contains query, each with its
// whole subtree, and the ancestors of deeper matches.
func filterNodes(nodes []flatui.TreeNode, query string) []flatui.TreeNode {
	var out []flatui.TreeNode
	for _, n := range nodes {
		if strings.Contains(strings.ToLower(n.Label), query) {
			out = append(out, n)
		} else if kept := filterNodes(n.Children, query); len(kept) > 0 {
			out = append(out, flatui.TreeNode{ID: n.ID, Label: n.Label, Children: kept})
		}
	}
	return out
}

func expandAll(t *flatui.Tree, nodes []flatui.TreeNode) {
	for _, n := range nodes {
		if len(n.Children) > 0 {
			t.Toggle(n.ID)
			expandAll(t, n.Children)
		}
	}
}

func (s *State) syncDetails() {
	row := s.selectedRow()
	lines := []string{
		"Selected: " + row.Label,
		"ID: " + row.ID,
		fmt.Sprintf("Depth: %d", row.Depth),
		"",
		"Search: " + searchValue(s.search.Value),
		"",
		"Tree state stays app-owned.",
		"Tab moves focus between sections.",
	}
	s.details.SetWrappedContent(strings.Join(lines, "\n"))
}

func (s *State) selectedRow() flatui.TreeRow {
	id := s.activeTree().CursorID()
	for _, row := range s.activeTree().VisibleRows() {
		if row.ID == id {
			return row
		}
	}
	return flatui.TreeRow{Label: "(none)"}
}

func searchValue(value string) string {
	if value == "" {
		return "(empty)"
	}
	return value
}

// View lays out the search field over the tree and details panels. The
// search Input places the cursor while focused.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	var rows layout.Node = layout.Col{NodeBase: layout.NodeBase{ID: treeBodyID, H: layout.Grow(1)}, Children: []layout.Node{
		layout.Label{Text: "no matches", Style: layout.TextStyle{Fg: lipgloss.Color("245")}},
	}}
	if len(s.activeTree().VisibleRows()) > 0 {
		col := s.activeTree().Rows("node", treeRow)
		col.ID, col.H = treeBodyID, layout.Grow(1)
		rows = col
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Tree", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Row{Children: []layout.Node{
			sectionTitle("search", s.focus.Focused(int(focusSearch))),
			layout.Label{Text: ": "},
			s.search.Layout("search", s.focus.Focused(int(focusSearch))),
		}},
		layout.Label{},
		layout.Row{NodeBase: layout.NodeBase{Gap: 2, H: layout.Grow(1)}, Children: []layout.Node{
			panel(2, sectionTitle("tree", s.focus.Focused(int(focusTree))), rows),
			panel(3, sectionTitle("details", s.focus.Focused(int(focusDetails))),
				layout.Col{NodeBase: layout.NodeBase{ID: detailsBodyID, H: layout.Grow(1)}, Children: []layout.Node{
					s.details.Layout(),
				}}),
		}},
		layout.Label{},
		layout.Label{Text: keyMap(s).View(), Style: layout.TextStyle{Role: layout.TextSubtle}},
	)}
}

// panel is a bordered section: its title, a blank row, and body, taking
// weight shares of the row.
func panel(weight float64, title, body layout.Node) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{
			W: layout.Grow(weight), PadLeft: 1, PadRight: 1,
			Box: layout.Box{Border: layout.BorderRounded, BorderColor: lipgloss.Color("240")},
		},
		Children: []layout.Node{title, layout.Label{}, body},
	}
}

func keyMap(s *State) flatui.KeyMap {
	return flatui.KeyMap{
		{Keys: []string{"tab"}, Help: "focus"},
		{Keys: []string{"enter", "space"}, Help: "toggle", Disabled: !s.focus.Focused(int(focusTree))},
		{Keys: []string{"←", "→"}, Help: "fold", Disabled: !s.focus.Focused(int(focusTree))},
		{Keys: []string{"j", "k"}, Help: "move"},
		{Keys: []string{"q"}, Help: "quit", Disabled: s.focus.Focused(int(focusSearch))},
	}
}

// treeRow is one node: the cursor marker (a chevron in a window, "> " in
// the terminal), the indentation, the disclosure toggle (a triangle, or
// "v "/"> "), and the label.
func treeRow(row flatui.TreeRow, selected bool) layout.Node {
	cursor := layout.Icon{Text: "  "}
	if selected {
		cursor = layout.Icon{Name: layout.IconChevronRight, Text: "> "}
	}
	toggle := layout.Icon{Text: "  "}
	if row.Expandable && row.Expanded {
		toggle = layout.Icon{Name: layout.IconDisclosureOpen, Text: "v "}
	} else if row.Expandable {
		toggle = layout.Icon{Name: layout.IconDisclosureClosed, Text: "> "}
	}
	return layout.Row{Children: []layout.Node{
		cursor,
		layout.Row{NodeBase: layout.NodeBase{PadLeft: 2 * row.Depth}, Children: []layout.Node{toggle, layout.Label{Text: row.Label}}},
	}}
}

func sectionTitle(label string, focused bool) layout.Node {
	if focused {
		return layout.Label{Text: "[" + label + "]", Style: layout.TextStyle{Bold: true, Fg: brand.Teal}}
	}
	return layout.Label{Text: " " + label + " ", Style: layout.TextStyle{Fg: lipgloss.Color("245")}}
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
