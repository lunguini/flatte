package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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

	// rects is the geometry the single per-frame SolveAndCompose records for
	// every ID'd node. View adopts it to place the hardware cursor from the
	// search line's solved rect — no re-parsing of the rendered frame.
	rects map[string]layout.Rect
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
	}
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
		accent:   lipgloss.Color("117"),
		good:     lipgloss.Color("114"),
		selected: lipgloss.Color("229"),
	}
}

type styles struct {
	title    lipgloss.Style
	subtle   lipgloss.Style
	section  lipgloss.Style
	selected lipgloss.Style
	table    flatui.TableStyle
	progress flatui.ProgressStyle
}

func newStyles(p palette) styles {
	base := lipgloss.NewStyle()
	return styles{
		title:    base.Bold(true).Foreground(p.accent),
		subtle:   base.Foreground(p.muted),
		section:  base.Bold(true).Foreground(p.base),
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

// View builds one layout tree and resolves it with a single SolveAndCompose.
// The frame is a Col — header, focus tabs, a blank rule, the three-pane body
// Row, a growing spacer that pins the footer to the bottom, and the footer.
// Panes size themselves from the responsive column widths; the search line
// carries an ID so the cursor is placed from its solved rect, and the tab strip
// is a leaf that fills its own rect width. No manual coordinate math survives.
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	p := defaultPalette()
	st := newStyles(p)
	width := max(ctx.Width, 64)
	height := s.height
	if height <= 0 {
		height = 24
	}
	leftOuter, centerOuter, rightOuter := columnWidths(width)

	header := layout.Text{
		NodeBase: layout.NodeBase{H: layout.Fixed(1)},
		String: lipgloss.JoinHorizontal(lipgloss.Top,
			st.title.Width(leftOuter+centerOuter-2).Render("Flat Workspace"),
			st.subtle.Render(fmt.Sprintf("focus %s | %d visible", focusName(s), len(s.results))),
		),
	}

	body := layout.Row{
		NodeBase: layout.NodeBase{Gap: paneGap},
		Children: []layout.Node{
			treePane(s, st, p, leftOuter),
			centerPane(s, st, p, centerOuter),
			detailsPane(s, st, p, rightOuter),
		},
	}

	footer := layout.Text{
		NodeBase: layout.NodeBase{H: layout.Fixed(1)},
		String:   st.subtle.Render(keyGroups(s).ViewWithOptions(flatui.KeyMapOptions{Mode: flatui.KeyMapFull, Width: width})),
	}

	root := layout.Col{Children: []layout.Node{
		header,
		tabsNode{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, state: s},
		layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}},
		body,
		layout.NewSpacer(),
		footer,
	}}

	content, rects := layout.SolveAndCompose(root, width, height)
	s.rects = rects

	frame := flatte.Frame{Content: content}
	if s.focus.Focused(int(focusSearch)) {
		if r, ok := rects["search"]; ok {
			frame.Cursor = &flatte.Cursor{
				X: r.X + lipgloss.Width(" "+searchPrefix) + s.search.CursorColumn(),
				Y: r.Y,
			}
		}
	}
	return frame
}

// searchPrefix is the fixed text before the editable value on the search line.
// The cursor is placed at its solved rect plus this prefix (and the one-column
// pane padding baked into the line) plus the field's cursor column.
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

// pane builds a bordered Col whose Chrome paints a rounded rule in fg. Its
// height is pinned to the content so the three panes stay ragged (top-aligned)
// the way the independent lipgloss blocks used to.
func pane(id string, outerW int, fg color.Color, kids []layout.Node) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID:       id,
			W:        layout.Fixed(outerW),
			H:        layout.Fixed(len(kids) + 2),
			Bordered: true,
			Chrome:   borderChrome(fg),
		},
		Children: kids,
	}
}

// line is one interior row of a pane: a fixed-height Text with the pane's
// one-column left padding baked in (the border inset supplies the rest).
func line(id, content string) layout.Node {
	return layout.Text{
		NodeBase: layout.NodeBase{ID: id, H: layout.Fixed(1)},
		String:   " " + content,
	}
}

func treePane(s *State, st styles, p palette, leftOuter int) layout.Node {
	var kids []layout.Node
	for _, ln := range treeLines(s, st, leftOuter-4) {
		kids = append(kids, line("", ln))
	}
	return pane("left", leftOuter-2, paneBorderColor(p, s.focus.Focused(int(focusTree))), kids)
}

func centerPane(s *State, st styles, p palette, centerOuter int) layout.Node {
	return pane("center", centerOuter-2, paneBorderColor(p, s.focus.Focused(int(focusSearch))), centerChildren(s, st, centerOuter-6))
}

func detailsPane(s *State, st styles, p palette, rightOuter int) layout.Node {
	var kids []layout.Node
	for _, ln := range detailsLines(s, st, rightOuter-6) {
		kids = append(kids, line("", ln))
	}
	return pane("right", rightOuter-2, paneBorderColor(p, s.focus.Focused(int(focusDetails))), kids)
}

func treeLines(s *State, st styles, width int) []string {
	lines := []string{st.section.Render("[tree]"), ""}
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
		lines = append(lines, strings.Split(view, "\n")...)
	}
	return lines
}

func centerChildren(s *State, st styles, width int) []layout.Node {
	searchLine := searchPrefix + s.search.Value
	if s.search.Value == "" {
		searchLine = searchPrefix + "(empty)"
	}
	if s.focus.Focused(int(focusSearch)) {
		searchLine = st.selected.Render(searchLine)
	}
	kids := []layout.Node{
		line("", st.section.Render("[work]")),
		line("search", fit(searchLine, width)),
		line("", ""),
		line("", st.table.Header.Render(s.table.Header())),
	}
	body := s.table.View(func(row string, selected bool) string {
		row = fit(row, width)
		if selected {
			return st.table.Active.Render(row)
		}
		return st.table.Row.Render(row)
	})
	if body == "" {
		kids = append(kids, line("", st.subtle.Render("no matching work")))
	} else {
		for _, row := range strings.Split(body, "\n") {
			kids = append(kids, line("", row))
		}
	}
	kids = append(kids, line("", ""), line("", s.progress.ViewWithStyle(st.progress)))
	return kids
}

func detailsLines(s *State, st styles, width int) []string {
	title := "[details]"
	if s.focus.Focused(int(focusDetails)) {
		title = "[details scroll]"
	}
	lines := []string{st.section.Render(title), ""}
	view := s.details.View()
	if view == "" {
		view = "No details"
	}
	for _, l := range strings.Split(view, "\n") {
		lines = append(lines, fit(l, width))
	}
	return lines
}

// tabsNode is the focus legend: a full-width dashed rule with the three area
// names centered across it, the active one bracketed. It is a leaf, so it draws
// itself to whatever width the solver hands it (the frame width) — the segment
// split is a rendering detail, not layout the parent has to pre-compute.
type tabsNode struct {
	layout.NodeBase
	state *State
}

func (t tabsNode) Render(r layout.Rect) string {
	labels := []struct {
		name    string
		focused bool
	}{
		{name: "Tree", focused: t.state.focus.Focused(int(focusTree))},
		{name: "Search", focused: t.state.focus.Focused(int(focusSearch))},
		{name: "Details", focused: t.state.focus.Focused(int(focusDetails))},
	}
	parts := make([]string, 0, len(labels))
	remaining := r.W
	for i, label := range labels {
		segmentWidth := remaining / (len(labels) - i)
		remaining -= segmentWidth
		text := label.name
		if label.focused {
			text = "[" + text + "]"
		}
		parts = append(parts, centerFill(text, segmentWidth))
	}
	return strings.Join(parts, "")
}

// centerFill centers text in width columns over a horizontal rule of dashes.
func centerFill(text string, width int) string {
	if width <= 0 {
		return ""
	}
	text = fit(text, width)
	pad := width - lipgloss.Width(text)
	left := pad / 2
	right := pad - left
	return strings.Repeat("─", left) + text + strings.Repeat("─", right)
}

// borderChrome paints a rounded pane border in fg, leaving the interior for the
// pane's children to draw over (flat-game's convention, without a title rule).
func borderChrome(fg color.Color) func(layout.Rect) string {
	return func(r layout.Rect) string {
		if r.W < 2 || r.H < 2 {
			return ""
		}
		b := lipgloss.NewStyle().Foreground(fg)
		rows := make([]string, r.H)
		rows[0] = b.Render("╭" + strings.Repeat("─", r.W-2) + "╮")
		side := b.Render("│") + strings.Repeat(" ", r.W-2) + b.Render("│")
		for i := 1; i < r.H-1; i++ {
			rows[i] = side
		}
		rows[r.H-1] = b.Render("╰" + strings.Repeat("─", r.W-2) + "╯")
		return strings.Join(rows, "\n")
	}
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
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
