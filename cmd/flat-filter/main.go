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

type Item struct {
	Title string
	Kind  string
}

func items() []Item {
	return []Item{
		{Title: "API gateway", Kind: "service"},
		{Title: "Auth flow", Kind: "security"},
		{Title: "Build cache", Kind: "tooling"},
		{Title: "Deploy queue", Kind: "ops"},
		{Title: "Docs index", Kind: "knowledge"},
		{Title: "Error budget", Kind: "reliability"},
		{Title: "Feature flags", Kind: "runtime"},
		{Title: "Graph report", Kind: "analytics"},
		{Title: "Health check", Kind: "ops"},
		{Title: "Inbox triage", Kind: "support"},
		{Title: "API schema", Kind: "contract"},
		{Title: "API client", Kind: "sdk"},
		{Title: "Release notes", Kind: "docs"},
		{Title: "Release checklist", Kind: "ops"},
	}
}

type State struct {
	items           []Item
	query           flatui.TextField
	list            flatui.List
	pages           flatui.Paginator
	filteredIndexes []int
}

func NewState() *State {
	s := &State{items: items()}
	s.pages.SetPageSize(5)
	s.list.SetHeight(5)
	s.syncFiltered()
	return s
}

// listID names the result list; its solved height is the page size.
const listID = "results"

// layout sizes a page to the rows the frame gave the list.
func (s *State) layout(rows int) {
	rows = max(rows, 1)
	s.pages.SetPageSize(rows)
	s.list.SetHeight(rows)
	s.syncFiltered()
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := e.Rects[listID]; ok && r.H != s.list.Height() {
			s.layout(r.H)
		}
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyEscape:
		fx.Quit()
	case flatte.KeyUp:
		s.list.MoveUp()
	case flatte.KeyDown:
		s.list.MoveDown()
	case flatte.KeyLeft:
		s.pages.PrevPage()
		s.list.Select(0)
		s.syncCurrentPage()
	case flatte.KeyRight:
		s.pages.NextPage()
		s.list.Select(0)
		s.syncCurrentPage()
	case flatte.KeyBackspace:
		s.query.Backspace()
		s.syncFiltered()
	case flatte.KeyDelete:
		s.query.Delete()
		s.syncFiltered()
	case flatte.KeyCharacter:
		s.query.Insert(key.Rune)
		s.syncFiltered()
	}
}

func (s *State) syncFiltered() {
	query := strings.ToLower(strings.TrimSpace(s.query.Value))
	s.filteredIndexes = s.filteredIndexes[:0]
	for i, item := range s.items {
		haystack := strings.ToLower(item.Title + " " + item.Kind)
		if query == "" || strings.Contains(haystack, query) {
			s.filteredIndexes = append(s.filteredIndexes, i)
		}
	}
	s.pages.SetTotal(len(s.filteredIndexes))
	s.syncCurrentPage()
}

func (s *State) syncCurrentPage() {
	first, last := s.pages.Range()
	if first > len(s.filteredIndexes) {
		first = len(s.filteredIndexes)
	}
	if last > len(s.filteredIndexes) {
		last = len(s.filteredIndexes)
	}
	s.list.SetCount(last - first)
}

func (s *State) currentPageIndexes() []int {
	first, last := s.pages.Range()
	if first > len(s.filteredIndexes) {
		first = len(s.filteredIndexes)
	}
	if last > len(s.filteredIndexes) {
		last = len(s.filteredIndexes)
	}
	return s.filteredIndexes[first:last]
}

// renderRow is one result: the selection marker (a chevron in a window,
// "> " in the terminal), the title in an 18-column field, and the kind.
func (s *State) renderRow(i int, selected bool) layout.Node {
	page := s.currentPageIndexes()
	if i < 0 || i >= len(page) {
		return layout.Label{}
	}
	item := s.items[page[i]]
	marker, style := layout.Icon{Text: "  ", Style: itemStyle}, itemStyle
	if selected {
		marker, style = layout.Icon{Name: layout.IconChevronRight, Text: "> ", Style: activeStyle}, activeStyle
	}
	return layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		layout.Row{Children: []layout.Node{marker, layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(18)}, Text: item.Title, Style: style}}},
		layout.Label{Text: item.Kind, Style: subtleStyle},
	}}
}

var (
	indent      = layout.NodeBase{PadLeft: 2}
	subtle      = layout.TextStyle{Role: layout.TextSubtle}
	itemStyle   = layout.TextStyle{Fg: lipgloss.Color("252")}
	activeStyle = layout.TextStyle{Fg: brand.Teal, Bold: true}
	subtleStyle = layout.TextStyle{Fg: lipgloss.Color("244")}
)

// View shows the query field, the current page of results, and the pager.
// The query Input places the cursor.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	summary := fmt.Sprintf("%d of %d results", len(s.filteredIndexes), len(s.items))
	query := s.query.Layout("filter.query", true)
	query.Placeholder = "(empty)"
	var results layout.Node = layout.Col{NodeBase: layout.NodeBase{ID: listID, H: layout.Grow(1)}, Children: []layout.Node{
		layout.Label{NodeBase: indent, Text: "no matches", Style: subtle},
	}}
	if s.list.Count() > 0 {
		rows := s.list.Rows(listID, s.renderRow)
		rows.ID, rows.H = listID, layout.Grow(1)
		results = rows
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Filter", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Row{NodeBase: indent, Children: []layout.Node{layout.Label{Text: "query: "}, query}},
		layout.Label{NodeBase: indent, Text: summary, Style: subtle},
		layout.Label{},
		results,
		layout.Label{},
		layout.Label{NodeBase: indent, Text: s.pages.View(), Style: subtle},
		layout.Label{Text: keyMap(s).View(), Style: subtle},
	)}
}

func keyMap(s *State) flatui.KeyMap {
	return flatui.KeyMap{
		{Keys: []string{"type"}, Help: "filter"},
		{Keys: []string{"up", "down"}, Help: "move", Disabled: s.list.Count() == 0},
		{Keys: []string{"left", "right"}, Help: "page", Disabled: s.pages.Pages() <= 1},
		{Keys: []string{"backspace"}, Help: "clear", Disabled: s.query.Value == ""},
		{Keys: []string{"esc"}, Help: "quit"},
	}
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
