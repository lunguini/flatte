package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/lunguini/flatte/cmd/internal/brand"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusSearch
	focusDetails
	focusCount
)

// paneGap is the blank-column gutter the body Row leaves between the three
// panes — the layout engine paints nothing there, matching the old two-space
// JoinHorizontal separator.
const paneGap = 2

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
	height   int

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
	s.height = 24
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
	_, centerOuter, rightOuter := columnWidths(width)
	bodyRows := max(min(height-13, 8), 4)
	s.height = max(height, 0)
	s.tree.SetHeight(bodyRows)
	s.table.SetHeight(bodyRows)
	s.setTableColumns(max(centerOuter-6, 20))
	s.details.SetSize(max(rightOuter-6, 14), bodyRows)
	s.progress.SetWidth(max(centerOuter-12, 8))
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

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		handleMouse(s, e)
	}
}

// handleMouse makes the panes clickable: a press focuses the pane under the
// pointer and acts on what it hit — a tree row selects it (again: toggles
// it), a work row selects it, the search field places its caret — and the
// wheel scrolls the pane under it.
func handleMouse(s *State, m flatte.MouseEvent) {
	area, ok := areaOf(m.Target)
	if !ok {
		return
	}
	switch m.Button {
	case flatte.MouseWheelUp, flatte.MouseWheelDown:
		s.focus.Select(int(area))
		delta := 1
		if m.Button == flatte.MouseWheelUp {
			delta = -1
		}
		handleVertical(s, delta)
		return
	case flatte.MouseLeft:
	default:
		return
	}
	if m.Action != flatte.MousePress {
		return
	}
	s.focus.Select(int(area))
	switch {
	case strings.HasPrefix(m.Target, "tree:"):
		id := strings.TrimPrefix(m.Target, "tree:")
		if id == s.tree.CursorID() {
			s.tree.Toggle(id)
			return
		}
		for i, row := range s.tree.VisibleRows() {
			if row.ID == id {
				s.tree.SelectVisible(i)
			}
		}
	case strings.HasPrefix(m.Target, "work:"):
		if i, err := strconv.Atoi(strings.TrimPrefix(m.Target, "work:")); err == nil {
			s.table.Select(i)
			s.syncDetails()
		}
	case m.Target == "search" && m.HasCaret:
		s.search.SetCursor(m.Caret)
	}
}

// areaOf is the focus area a pointer target belongs to: a pane, or a node
// inside one.
func areaOf(target string) (focusArea, bool) {
	switch {
	case target == "left" || strings.HasPrefix(target, "tree:"):
		return focusTree, true
	case target == "center" || target == "search" || strings.HasPrefix(target, "work:"):
		return focusSearch, true
	case target == "right":
		return focusDetails, true
	}
	return 0, false
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyEscape:
		fx.Quit()
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			s.focus.Prev()
		} else {
			s.focus.Next()
		}
	case flatte.KeyUp:
		handleVertical(s, -1)
	case flatte.KeyDown:
		handleVertical(s, 1)
	case flatte.KeyEnter:
		if s.focus.Focused(int(focusTree)) {
			s.tree.Toggle(s.tree.CursorID())
		}
	case flatte.KeyBackspace:
		if s.focus.Focused(int(focusSearch)) {
			s.search.Backspace()
			s.syncResults()
		}
	case flatte.KeyDelete:
		if s.focus.Focused(int(focusSearch)) {
			s.search.Delete()
			s.syncResults()
		}
	case flatte.KeyLeft:
		if s.focus.Focused(int(focusTree)) {
			collapseSelectedTreeRow(s)
		} else if s.focus.Focused(int(focusSearch)) {
			s.search.MoveLeft()
		}
	case flatte.KeyRight:
		if s.focus.Focused(int(focusTree)) {
			expandSelectedTreeRow(s)
		} else if s.focus.Focused(int(focusSearch)) {
			s.search.MoveRight()
		}
	case flatte.KeyCharacter:
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
	case s.focus.Focused(int(focusSearch)):
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

func handleCharacter(s *State, key flatte.KeyEvent) {
	if s.focus.Focused(int(focusSearch)) {
		s.search.Insert(key.Rune)
		s.syncResults()
		return
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
		accent:   brand.Teal,
		good:     lipgloss.Color("114"),
		selected: lipgloss.Color("229"),
	}
}

// View returns the workspace as a Frame.Root tree — header, focus tabs, a
// blank row, the three-pane body Row, a growing spacer that pins the footer
// to the bottom, and the footer — built from widget nodes (Tree.Rows,
// Table.Rows, the search Input, a Meter, the details Viewport), so a window
// draws native components and the search Input places the cursor.
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	p := defaultPalette()
	width := max(ctx.Width, 64)
	leftOuter, centerOuter, rightOuter := columnWidths(width)
	muted := layout.TextStyle{Fg: p.muted}

	header := layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(leftOuter + centerOuter - 2)}, Text: "Flat Workspace", Style: layout.TextStyle{Bold: true, Fg: p.accent}},
		layout.Label{Text: fmt.Sprintf("focus %s | %d visible", focusName(s), len(s.results)), Style: muted},
	}}
	body := layout.Row{
		NodeBase: layout.NodeBase{Gap: paneGap},
		Children: []layout.Node{
			pane("left", leftOuter-2, paneBorderColor(p, s.focus.Focused(int(focusTree))), treeChildren(s, p)),
			pane("center", centerOuter-2, paneBorderColor(p, s.focus.Focused(int(focusSearch))), centerChildren(s, p)),
			pane("right", rightOuter-2, paneBorderColor(p, s.focus.Focused(int(focusDetails))), detailsChildren(s, p)),
		},
	}
	footer := layout.Label{
		NodeBase: layout.NodeBase{H: layout.Fixed(1)},
		Text:     keyGroups(s).ViewWithOptions(flatui.KeyMapOptions{Mode: flatui.KeyMapFull, Width: width}),
		Style:    muted,
	}
	return flatte.Frame{Root: layout.Col{Children: []layout.Node{
		header,
		tabs(s, width),
		layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}},
		body,
		layout.NewSpacer(),
		footer,
	}}}
}

// searchPrefix labels the search field.
const searchPrefix = "search: "

// columnWidths splits the frame into the three responsive body columns (outer
// widths, border included). It is the single source for the split: View feeds
// each width to a Fixed pane in the body Row, and layout() sizes the widgets
// from the same numbers. The panes total less than the frame, so the layout
// engine leaves the surplus columns unpainted (trimmed), reproducing the old
// JoinHorizontal geometry exactly.
func columnWidths(width int) (leftOuter, centerOuter, rightOuter int) {
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

// pane is a rounded border in fg around kids, one column in. Its height is
// pinned to the content so the three panes stay ragged (top-aligned).
func pane(id string, outerW int, fg color.Color, kids []layout.Node) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID:      id,
			W:       layout.Fixed(outerW),
			H:       layout.Fixed(rows(kids) + 2),
			PadLeft: 1,
			Box:     layout.Box{Border: layout.BorderRounded, BorderColor: fg},
		},
		Children: kids,
	}
}

// rows is how many lines kids take: a Col's children's, a fixed height's
// lines, else one.
func rows(kids []layout.Node) int {
	n := 0
	for _, k := range kids {
		if c, ok := k.(layout.Col); ok {
			n += rows(c.Children)
			continue
		}
		if _, h := k.Size(); h.Kind == layout.SizeFixed {
			n += h.Value
			continue
		}
		n++
	}
	return n
}

// section is a pane's bracketed title and the blank row under it.
func section(title string, p palette) []layout.Node {
	return []layout.Node{layout.Label{Text: title, Style: layout.TextStyle{Bold: true, Fg: p.base}}, layout.Label{}}
}

func treeChildren(s *State, p palette) []layout.Node {
	kids := section("[tree]", p)
	if len(s.tree.VisibleRows()) == 0 {
		return kids
	}
	selected := layout.TextStyle{Bold: true, Fg: p.selected}
	rows := s.tree.Rows("tree", func(row flatui.TreeRow, sel bool) layout.Node {
		marker, style := layout.Icon{Text: "  "}, layout.TextStyle{}
		if sel {
			marker, style = layout.Icon{Name: layout.IconChevronRight, Text: "> ", Style: selected}, selected
		}
		icon := layout.Icon{Text: " ", Style: style}
		switch {
		case row.Expandable && row.Expanded:
			icon.Name, icon.Text = layout.IconDisclosureOpen, "▾"
		case row.Expandable:
			icon.Name, icon.Text = layout.IconDisclosureClosed, "▸"
		}
		indent := ""
		if row.Depth > 0 {
			indent = " "
		}
		return layout.Row{Children: []layout.Node{
			marker,
			layout.Label{Text: indent, Style: style},
			icon,
			layout.Label{Text: " " + row.Label, Style: style},
		}}
	})
	return append(kids, rows)
}

func centerChildren(s *State, p palette) []layout.Node {
	focused := s.focus.Focused(int(focusSearch))
	prefix := layout.Label{Text: searchPrefix}
	search := s.search.Layout("search", focused)
	search.Placeholder = "(empty)"
	if focused {
		prefix.Style = layout.TextStyle{Bold: true, Fg: p.selected}
		search.Style = prefix.Style
	}
	kids := append(section("[work]", p)[:1],
		layout.Row{Children: []layout.Node{prefix, search}},
		layout.Label{},
		s.table.HeaderLayout(layout.TextStyle{Bold: true, Fg: p.accent}),
	)
	if len(s.results) == 0 {
		kids = append(kids, layout.Label{Text: "no matching work", Style: layout.TextStyle{Fg: p.muted}})
	} else {
		kids = append(kids, s.table.Rows("work", func(cells []string, selected bool) layout.Node {
			style := layout.TextStyle{Fg: p.base}
			if selected {
				style = layout.TextStyle{Bold: true, Fg: p.selected}
			}
			return s.table.Cells(cells, style)
		}))
	}
	return append(kids, layout.Label{}, progressNode(s.progress, p))
}

// progressNode is the progress bar in the workspace's colors: a Meter and
// its bold percentage.
func progressNode(pr flatui.Progress, p palette) layout.Node {
	return layout.Row{Children: []layout.Node{
		layout.Meter{NodeBase: layout.NodeBase{W: layout.Fixed(pr.Width())}, Value: pr.Percent() / 100, Fill: p.good, Track: p.panel},
		layout.Label{Text: fmt.Sprintf("  %3d%%", int(math.Round(pr.Percent()))), Style: layout.TextStyle{Bold: true, Fg: p.base}},
	}}
}

func detailsChildren(s *State, p palette) []layout.Node {
	title := "[details]"
	if s.focus.Focused(int(focusDetails)) {
		title = "[details scroll]"
	}
	kids := section(title, p)
	if s.details.VisibleLines() == 0 {
		return append(kids, layout.Label{Text: "No details"})
	}
	return append(kids, layout.Col{Children: []layout.Node{s.details.Layout()}})
}

// tabs is the focus legend: a rule width columns wide with the three area
// names centered in equal segments, the active one bracketed.
func tabs(s *State, width int) layout.Node {
	labels := []struct {
		name    string
		focused bool
	}{
		{name: "Tree", focused: s.focus.Focused(int(focusTree))},
		{name: "Search", focused: s.focus.Focused(int(focusSearch))},
		{name: "Details", focused: s.focus.Focused(int(focusDetails))},
	}
	row := layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1)}}
	remaining := width
	for i, label := range labels {
		segment := remaining / (len(labels) - i)
		remaining -= segment
		text := label.name
		if label.focused {
			text = "[" + text + "]"
		}
		text = fit(text, segment)
		pad := segment - lipgloss.Width(text)
		row.Children = append(row.Children, layout.Row{Children: []layout.Node{
			rule(pad / 2), layout.Label{Text: text}, rule(pad - pad/2),
		}})
	}
	return row
}

// rule is a horizontal line n columns wide (drawn as a line in a window,
// like any box-drawing cells).
func rule(n int) layout.Node {
	return layout.Text{NodeBase: layout.NodeBase{W: layout.Fixed(n)}, String: strings.Repeat("─", max(n, 0))}
}

func paneBorderColor(p palette, focused bool) color.Color {
	if focused {
		return p.accent
	}
	return p.panel
}

func focusName(s *State) string {
	switch {
	case s.focus.Focused(int(focusSearch)):
		return "search"
	case s.focus.Focused(int(focusDetails)):
		return "details"
	default:
		return "tree"
	}
}

func keyGroups(s *State) flatui.KeyGroups {
	switch {
	case s.focus.Focused(int(focusTree)):
		return flatui.KeyGroups{
			{Title: "nav", Bindings: flatui.KeyMap{
				{Keys: []string{"tab"}, Help: "focus"},
				{Keys: []string{"enter"}, Help: "toggle"},
				{Keys: []string{"left", "right"}, Help: "open/close"},
				{Keys: []string{"up", "down"}, Help: "move"},
			}},
			{Title: "app", Bindings: flatui.KeyMap{
				{Keys: []string{"esc"}, Help: "quit"},
			}},
		}
	case s.focus.Focused(int(focusSearch)):
		return flatui.KeyGroups{
			{Title: "nav", Bindings: flatui.KeyMap{
				{Keys: []string{"tab"}, Help: "focus"},
				{Keys: []string{"up", "down"}, Help: "rows"},
			}},
			{Title: "edit", Bindings: flatui.KeyMap{
				{Keys: []string{"type"}, Help: "search"},
				{Keys: []string{"backspace"}, Help: "edit"},
			}},
			{Title: "app", Bindings: flatui.KeyMap{
				{Keys: []string{"esc"}, Help: "quit"},
			}},
		}
	case s.focus.Focused(int(focusDetails)):
		return flatui.KeyGroups{
			{Title: "nav", Bindings: flatui.KeyMap{
				{Keys: []string{"tab"}, Help: "focus"},
			}},
			{Title: "details", Bindings: flatui.KeyMap{
				{Keys: []string{"j", "k"}, Help: "scroll"},
			}},
			{Title: "app", Bindings: flatui.KeyMap{
				{Keys: []string{"esc"}, Help: "quit"},
			}},
		}
	}
	return flatui.KeyGroups{
		{Title: "nav", Bindings: flatui.KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
		}},
		{Title: "app", Bindings: flatui.KeyMap{
			{Keys: []string{"esc"}, Help: "quit"},
		}},
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

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	app := flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Flat Workspace", Width: 86, Height: 24})
	} else {
		err = flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
