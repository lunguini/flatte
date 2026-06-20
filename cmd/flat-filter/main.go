package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
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

func (s *State) layout(height int) {
	const pinnedRows = 7 // title, query, summary, blanks, page line, footer
	rows := max(flatui.CardBodyHeight(height, pinnedRows), 1)
	s.pages.SetPageSize(rows)
	s.list.SetHeight(rows)
	s.syncFiltered()
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch e := ev.(type) {
	case flat.ResizeEvent:
		s.layout(e.Height)
	case flat.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	switch key.Key {
	case flat.KeyEscape:
		fx.Quit()
	case flat.KeyUp:
		s.list.MoveUp()
	case flat.KeyDown:
		s.list.MoveDown()
	case flat.KeyLeft:
		s.pages.PrevPage()
		s.list.Select(0)
		s.syncCurrentPage()
	case flat.KeyRight:
		s.pages.NextPage()
		s.list.Select(0)
		s.syncCurrentPage()
	case flat.KeyBackspace:
		s.query.Backspace()
		s.syncFiltered()
	case flat.KeyDelete:
		s.query.Delete()
		s.syncFiltered()
	case flat.KeyCharacter:
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

func (s *State) renderRow(i int, selected bool) string {
	page := s.currentPageIndexes()
	if i < 0 || i >= len(page) {
		return ""
	}
	item := s.items[page[i]]
	marker := "  "
	style := itemStyle()
	if selected {
		marker = "> "
		style = activeStyle()
	}
	return style.Render(fmt.Sprintf("%s%-18s %s", marker, item.Title, subtleStyle().Render(item.Kind)))
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	query := s.query.Value
	if query == "" {
		query = "(empty)"
	}
	summary := fmt.Sprintf("%d of %d results", len(s.filteredIndexes), len(s.items))
	pageLine := s.pages.View()

	lines := []string{
		flatui.Title("Flat Filter"),
		"  query: " + query,
		flatui.Subtle("  " + summary),
		"",
	}
	view := s.list.View(s.renderRow)
	if view == "" {
		lines = append(lines, flatui.Subtle("  no matches"))
	} else {
		lines = append(lines, strings.Split(view, "\n")...)
	}
	lines = append(lines, "", flatui.Subtle("  "+pageLine), flatui.Subtle(keyMap(s).View()))

	frame := flat.Frame{Content: flatui.Card(lines, ctx.Width)}
	originX, originY := flatui.CardOrigin()
	frame.Cursor = &flat.Cursor{
		X: originX + lipgloss.Width("  query: ") + s.query.CursorColumn(),
		Y: originY + 1,
	}
	return frame
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

func itemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
}

func activeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
}

func subtleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
