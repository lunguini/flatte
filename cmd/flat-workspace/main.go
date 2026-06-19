package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusSearch
	focusTable
	focusDetails
	focusCount
)

type WorkItem struct {
	ID       string
	Title    string
	Area     string
	Owner    string
	Status   string
	Progress float64
	Notes    []string
}

var workItems = []WorkItem{
	{ID: "api-gateway", Title: "API gateway", Area: "services", Owner: "platform", Status: "ready", Progress: 68, Notes: []string{"Routes edge.", "Canary next."}},
	{ID: "billing-sync", Title: "Billing sync", Area: "services", Owner: "finance", Status: "watch", Progress: 42, Notes: []string{"Ledger lagging.", "Retry review."}},
	{ID: "api-schema", Title: "API schema", Area: "services", Owner: "platform", Status: "ready", Progress: 76, Notes: []string{"SDK schema.", "Audit done."}},
	{ID: "search-index", Title: "Search index", Area: "operations", Owner: "growth", Status: "blocked", Progress: 55, Notes: []string{"Quota blocked.", "Restart pending."}},
	{ID: "release-train", Title: "Release train", Area: "operations", Owner: "release", Status: "ready", Progress: 88, Notes: []string{"Release notes drafted.", "Smoke pass remains."}},
	{ID: "incident-review", Title: "Incident review", Area: "operations", Owner: "sre", Status: "watch", Progress: 35, Notes: []string{"Timeline assembled.", "Action owners needed."}},
}

type State struct {
	focus    flatui.FocusRing
	tree     flatui.Tree
	search   flatui.TextField
	table    flatui.Table
	details  flatui.Viewport
	progress flatui.Progress

	results []WorkItem
}

func NewState() *State {
	s := &State{
		tree:     flatui.NewTree(workspaceTree()),
		progress: flatui.NewProgress(18),
	}
	s.focus.SetCount(int(focusCount))
	s.tree.Toggle("workspace")
	s.setTableColumns(32)
	s.tree.SetHeight(6)
	s.table.SetHeight(6)
	s.details.SetSize(24, 6)
	s.syncResults()
	return s
}

func workspaceTree() []flatui.TreeNode {
	return []flatui.TreeNode{
		{ID: "workspace", Label: "workspace", Children: []flatui.TreeNode{
			{ID: "services", Label: "services", Children: []flatui.TreeNode{
				{ID: "api-gateway", Label: "API"},
				{ID: "billing-sync", Label: "Billing"},
				{ID: "api-schema", Label: "Schema"},
			}},
			{ID: "operations", Label: "ops", Children: []flatui.TreeNode{
				{ID: "search-index", Label: "Search"},
				{ID: "release-train", Label: "Release"},
				{ID: "incident-review", Label: "Incident"},
			}},
		}},
	}
}

func (s *State) layout(width, height int) {
	_, centerOuter, rightOuter := layoutWidths(width)
	bodyRows := max(min(height-13, 8), 4)
	s.tree.SetHeight(bodyRows)
	s.table.SetHeight(bodyRows)
	s.setTableColumns(max(centerOuter-6, 20))
	s.details.SetSize(max(rightOuter-6, 14), bodyRows)
	s.progress.SetWidth(max(min(width/4, 24), 10))
	s.syncDetails()
}

func (s *State) setTableColumns(contentWidth int) {
	ownerWidth := 8
	if contentWidth < 30 {
		ownerWidth = 7
	}
	stateWidth := 7
	workWidth := min(max(contentWidth-ownerWidth-stateWidth-2, 8), 12)
	s.table.SetColumns([]flatui.Column{
		{Title: "work", Width: workWidth},
		{Title: "owner", Width: ownerWidth},
		{Title: "state", Width: stateWidth},
	})
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch e := ev.(type) {
	case flat.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flat.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	switch key.Key {
	case flat.KeyEscape:
		fx.Quit()
	case flat.KeyTab:
		if key.Mod.Contains(flat.ModShift) {
			s.focus.Prev()
		} else {
			s.focus.Next()
		}
	case flat.KeyUp:
		handleVertical(s, -1)
	case flat.KeyDown:
		handleVertical(s, 1)
	case flat.KeyEnter:
		if s.focus.Focused(int(focusTree)) {
			s.tree.Toggle(s.tree.CursorID())
		}
	case flat.KeyBackspace:
		if s.focus.Focused(int(focusSearch)) {
			s.search.Backspace()
			s.syncResults()
		}
	case flat.KeyDelete:
		if s.focus.Focused(int(focusSearch)) {
			s.search.Delete()
			s.syncResults()
		}
	case flat.KeyLeft:
		if s.focus.Focused(int(focusTree)) {
			collapseSelectedTreeRow(s)
		} else if s.focus.Focused(int(focusSearch)) {
			s.search.MoveLeft()
		}
	case flat.KeyRight:
		if s.focus.Focused(int(focusTree)) {
			expandSelectedTreeRow(s)
		} else if s.focus.Focused(int(focusSearch)) {
			s.search.MoveRight()
		}
	case flat.KeyCharacter:
		handleCharacter(s, key)
	}
}

func selectedTreeRow(s *State) (flatui.TreeRow, bool) {
	id := s.tree.CursorID()
	for _, row := range s.tree.VisibleRows() {
		if row.ID == id {
			return row, true
		}
	}
	return flatui.TreeRow{}, false
}

func expandSelectedTreeRow(s *State) {
	row, ok := selectedTreeRow(s)
	if ok && row.Expandable && !row.Expanded {
		s.tree.Toggle(row.ID)
	}
}

func collapseSelectedTreeRow(s *State) {
	row, ok := selectedTreeRow(s)
	if ok && row.Expandable && row.Expanded {
		s.tree.Toggle(row.ID)
	}
}

func handleVertical(s *State, delta int) {
	switch {
	case s.focus.Focused(int(focusTree)):
		if delta < 0 {
			s.tree.MoveUp()
		} else {
			s.tree.MoveDown()
		}
	case s.focus.Focused(int(focusTable)):
		if delta < 0 {
			s.table.MoveUp()
		} else {
			s.table.MoveDown()
		}
		s.syncDetails()
	case s.focus.Focused(int(focusDetails)):
		if delta < 0 {
			s.details.LineUp(1)
		} else {
			s.details.LineDown(1)
		}
	}
}

func handleCharacter(s *State, key flat.KeyEvent) {
	if s.focus.Focused(int(focusSearch)) {
		s.search.Insert(key.Rune)
		s.syncResults()
		return
	}
	if s.focus.Focused(int(focusTable)) {
		switch key.Rune {
		case 'j', 'J':
			s.table.MoveDown()
			s.syncDetails()
		case 'k', 'K':
			s.table.MoveUp()
			s.syncDetails()
		}
	}
	if s.focus.Focused(int(focusDetails)) {
		switch key.Rune {
		case 'j', 'J':
			s.details.LineDown(1)
		case 'k', 'K':
			s.details.LineUp(1)
		}
	}
}

func (s *State) syncResults() {
	query := strings.ToLower(strings.TrimSpace(s.search.Value))
	s.results = s.results[:0]
	for _, item := range workItems {
		haystack := strings.ToLower(item.Title + " " + item.Area + " " + item.Owner + " " + item.Status)
		if query == "" || strings.Contains(haystack, query) {
			s.results = append(s.results, item)
		}
	}
	rows := make([][]string, len(s.results))
	for i, item := range s.results {
		rows[i] = []string{item.Title, item.Owner, item.Status}
	}
	s.table.SetRows(rows)
	s.syncDetails()
}

func (s *State) selectedResult() WorkItem {
	if len(s.results) == 0 {
		return WorkItem{}
	}
	return s.results[min(max(s.table.Cursor(), 0), len(s.results)-1)]
}

func (s *State) visibleResults() []WorkItem {
	return append([]WorkItem(nil), s.results...)
}

func (s *State) syncDetails() {
	item := s.selectedResult()
	if item.ID == "" {
		s.progress.SetPercent(0)
		s.details.SetContent("No matching work item.\nTry a broader query.")
		return
	}
	s.progress.SetPercent(item.Progress)
	lines := []string{
		item.Title,
		"owner: " + item.Owner,
		"area: " + item.Area,
		"status: " + item.Status,
		fmt.Sprintf("progress: %.0f%%", item.Progress),
		"",
	}
	lines = append(lines, item.Notes...)
	s.details.SetWrappedContent(strings.Join(lines, "\n"))
}

type palette struct {
	base     color.Color
	muted    color.Color
	panel    color.Color
	accent   color.Color
	good     color.Color
	selected color.Color
}

func defaultPalette() palette {
	return palette{
		base:     lipgloss.Color("252"),
		muted:    lipgloss.Color("245"),
		panel:    lipgloss.Color("238"),
		accent:   lipgloss.Color("117"),
		good:     lipgloss.Color("114"),
		selected: lipgloss.Color("229"),
	}
}

type styles struct {
	title    lipgloss.Style
	subtle   lipgloss.Style
	section  lipgloss.Style
	panel    lipgloss.Style
	focused  lipgloss.Style
	selected lipgloss.Style
	table    flatui.TableStyle
	progress flatui.ProgressStyle
}

func newStyles(p palette) styles {
	base := lipgloss.NewStyle()
	return styles{
		title:   base.Bold(true).Foreground(p.accent),
		subtle:  base.Foreground(p.muted),
		section: base.Bold(true).Foreground(p.base),
		panel: base.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.panel).
			Padding(0, 1),
		focused:  base.BorderForeground(p.accent),
		selected: base.Bold(true).Foreground(p.selected),
		table: flatui.TableStyle{
			Header: base.Bold(true).Foreground(p.accent),
			Row:    base.Foreground(p.base),
			Active: base.Bold(true).Foreground(p.selected),
		},
		progress: flatui.ProgressStyle{
			Filled: base.Foreground(p.good),
			Empty:  base.Foreground(p.panel),
			Label:  base.Bold(true).Foreground(p.base),
		},
	}
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	st := newStyles(defaultPalette())
	width := max(ctx.Width, 64)
	leftOuter, centerOuter, rightOuter := layoutWidths(width)

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		st.title.Width(leftOuter+centerOuter-2).Render("Flat Workspace"),
		st.subtle.Render(fmt.Sprintf("focus %s | %d visible", focusName(s), len(s.results))),
	)
	left := panel(st, s.focus.Focused(int(focusTree)), leftOuter, treePanel(s, st, leftOuter-4))
	center := panel(st, s.focus.Focused(int(focusSearch)) || s.focus.Focused(int(focusTable)), centerOuter, centerPanel(s, st, centerOuter-6))
	right := panel(st, s.focus.Focused(int(focusDetails)), rightOuter, detailsPanel(s, st, rightOuter-6))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", center, "  ", right)
	footer := st.subtle.Render(keyMap(s).View())

	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
	frame := flat.Frame{Content: trimRightLines(content)}
	if s.focus.Focused(int(focusSearch)) {
		frame.Cursor = searchCursor(frame.Content, s.search.CursorColumn())
	}
	return frame
}

func focusName(s *State) string {
	switch {
	case s.focus.Focused(int(focusSearch)):
		return "search"
	case s.focus.Focused(int(focusTable)):
		return "work"
	case s.focus.Focused(int(focusDetails)):
		return "details"
	default:
		return "tree"
	}
}

func searchCursor(content string, cursorColumn int) *flat.Cursor {
	const prefix = "search: "
	for y, line := range strings.Split(ansi.Strip(content), "\n") {
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		return &flat.Cursor{
			X: lipgloss.Width(line[:idx+len(prefix)]) + cursorColumn,
			Y: y,
		}
	}
	return nil
}

func layoutWidths(width int) (leftOuter, centerOuter, rightOuter int) {
	width = max(width, 64)
	leftOuter = min(max((width+3)/4, 16), 22)
	rightOuter = min(max(width*28/100, 18), 24)
	centerOuter = width - leftOuter - rightOuter - 4
	if centerOuter < 26 {
		centerOuter = 26
		rightOuter = max(width-leftOuter-centerOuter-4, 16)
	}
	return leftOuter, centerOuter, rightOuter
}

func panel(st styles, focused bool, width int, content string) string {
	style := st.panel.Width(width - 2)
	if focused {
		style = style.Inherit(st.focused)
	}
	return style.Render(content)
}

func treePanel(s *State, st styles, width int) string {
	rows := []string{st.section.Render("[tree]"), ""}
	view := s.tree.View(func(row flatui.TreeRow, selected bool) string {
		icon := " "
		if row.Expandable && row.Expanded {
			icon = "▾"
		} else if row.Expandable {
			icon = "▸"
		}
		indent := ""
		if row.Depth > 0 {
			indent = " "
		}
		text := indent + icon + " " + row.Label
		text = fit(text, max(width-2, 0))
		if selected {
			return st.selected.Render("> " + text)
		}
		return "  " + text
	})
	if view != "" {
		rows = append(rows, strings.Split(view, "\n")...)
	}
	return strings.Join(rows, "\n")
}

func centerPanel(s *State, st styles, width int) string {
	searchLine := "search: " + s.search.Value
	if s.search.Value == "" {
		searchLine = "search: (empty)"
	}
	if s.focus.Focused(int(focusSearch)) {
		searchLine = st.selected.Render(searchLine)
	}
	rows := []string{
		st.section.Render("[work]"),
		fit(searchLine, width),
		"",
		st.table.Header.Render(s.table.Header()),
	}
	body := s.table.View(func(row string, selected bool) string {
		row = fit(row, width)
		if selected {
			return st.table.Active.Render(row)
		}
		return st.table.Row.Render(row)
	})
	if body == "" {
		rows = append(rows, st.subtle.Render("no matching work"))
	} else {
		rows = append(rows, strings.Split(body, "\n")...)
	}
	rows = append(rows, "", s.progress.ViewWithStyle(st.progress))
	return strings.Join(rows, "\n")
}

func detailsPanel(s *State, st styles, width int) string {
	title := "[details]"
	if s.focus.Focused(int(focusDetails)) {
		title = "[details scroll]"
	}
	rows := []string{st.section.Render(title), ""}
	view := s.details.View()
	if view == "" {
		view = "No details"
	}
	for _, line := range strings.Split(view, "\n") {
		rows = append(rows, fit(line, width))
	}
	return strings.Join(rows, "\n")
}

func keyMap(s *State) flatui.KeyMap {
	switch {
	case s.focus.Focused(int(focusTree)):
		return flatui.KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"enter"}, Help: "toggle"},
			{Keys: []string{"left", "right"}, Help: "open/close"},
			{Keys: []string{"up", "down"}, Help: "move"},
			{Keys: []string{"esc"}, Help: "quit"},
		}
	case s.focus.Focused(int(focusSearch)):
		return flatui.KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"type"}, Help: "search"},
			{Keys: []string{"backspace"}, Help: "edit"},
			{Keys: []string{"esc"}, Help: "quit"},
		}
	case s.focus.Focused(int(focusDetails)):
		return flatui.KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"j", "k"}, Help: "scroll"},
			{Keys: []string{"esc"}, Help: "quit"},
		}
	default:
		return flatui.KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"up", "down"}, Help: "rows"},
			{Keys: []string{"esc"}, Help: "quit"},
		}
	}
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}

func trimRightLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
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
