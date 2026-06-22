package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusSearch
	focusDetails
)

type State struct {
	focus   flatui.FocusRing
	tree    flatui.Tree
	search  flatui.TextField
	details flatui.Viewport
}

func NewState() *State {
	s := &State{tree: flatui.NewTree(treeNodes())}
	s.focus.SetCount(3)
	s.tree.Toggle("workspace")
	s.layout(72, 18)
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

func (s *State) layout(width, height int) {
	_, _, detailsWidth := paneSizes(width)
	bodyRows := max(flatui.CardBodyHeight(height, 5), 5)
	bodyHeight := max(bodyRows-2, 1)
	s.tree.SetHeight(bodyHeight)
	s.details.SetSize(detailsWidth, bodyHeight)
	s.syncDetails()
}

func paneSizes(width int) (left, right, detailsWidth int) {
	bodyWidth := max(flatui.CardBodyWidth(width), 40)
	left = max(bodyWidth*2/5, 20)
	right = max(bodyWidth-left-2, 20)
	if left+right+2 > bodyWidth {
		left = max(bodyWidth-right-2, 12)
	}
	return left, right, max(right-4, 1)
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(ev.Width, ev.Height)
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
	switch key.Key {
	case flatte.KeyDown:
		s.tree.MoveDown()
	case flatte.KeyUp:
		s.tree.MoveUp()
	case flatte.KeyEnter:
		s.tree.Toggle(s.tree.CursorID())
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j':
			s.tree.MoveDown()
		case 'k':
			s.tree.MoveUp()
		case ' ':
			s.tree.Toggle(s.tree.CursorID())
		case 'q':
			fx.Quit()
		}
	}
	s.syncDetails()
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
	id := s.tree.CursorID()
	for _, row := range s.tree.VisibleRows() {
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

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	leftWidth, rightWidth, _ := paneSizes(ctx.Width)
	treePanel := panelStyle().Width(leftWidth).Render(strings.Join([]string{
		sectionTitle("tree", s.focus.Focused(int(focusTree))),
		"",
		s.tree.View(renderTreeRow),
	}, "\n"))
	detailsPanel := panelStyle().Width(rightWidth).Render(strings.Join([]string{
		sectionTitle("details", s.focus.Focused(int(focusDetails))),
		"",
		s.details.View(),
	}, "\n"))
	body := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, "  ", detailsPanel)
	searchLine := sectionTitle("search", s.focus.Focused(int(focusSearch))) + ": " + s.search.Value
	footer := flatui.Subtle(keyMap(s).View())
	lines := []string{
		flatui.Title("Flat Tree"),
		searchLine,
		"",
		body,
		"",
		footer,
	}
	frame := flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
	if s.focus.Focused(int(focusSearch)) {
		x, y := flatui.CardOrigin()
		frame.Cursor = &flatte.Cursor{
			X: x + lipgloss.Width(sectionTitle("search", true)+": ") + s.search.CursorColumn(),
			Y: y + 1,
		}
	}
	return frame
}

func keyMap(s *State) flatui.KeyMap {
	return flatui.KeyMap{
		{Keys: []string{"tab"}, Help: "focus"},
		{Keys: []string{"enter", "space"}, Help: "toggle", Disabled: !s.focus.Focused(int(focusTree))},
		{Keys: []string{"j", "k"}, Help: "move"},
		{Keys: []string{"q"}, Help: "quit", Disabled: s.focus.Focused(int(focusSearch))},
	}
}

func renderTreeRow(row flatui.TreeRow, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}
	toggle := "  "
	if row.Expandable && row.Expanded {
		toggle = "v "
	} else if row.Expandable {
		toggle = "> "
	}
	return cursor + strings.Repeat("  ", row.Depth) + toggle + row.Label
}

func sectionTitle(label string, focused bool) string {
	if focused {
		return activeStyle().Render("[" + label + "]")
	}
	return mutedStyle().Render(" " + label + " ")
}

func panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
}

func activeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
}

func mutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
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
