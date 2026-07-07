package main

import (
	"context"
	"fmt"
	"html"
	"image/color"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type landingTab int

const (
	tabOverview landingTab = iota
	tabComponents
	tabArchitecture
)

var landingTabs = []flatui.TabItem{
	{ID: "overview", Label: "Overview"},
	{ID: "components", Label: "Components"},
	{ID: "architecture", Label: "Architecture"},
}

type showcaseItem struct {
	Name    string
	Kind    string
	Summary string
	Detail  string
}

var showcase = []showcaseItem{
	{Name: "TextField", Kind: "input", Summary: "Grapheme-correct single-line input with app-owned focus policy.", Detail: "TextField stores plain Go state: Value, Cursor, and optional selection. Apps decide when keys edit it, where the cursor lands, and how the field is styled."},
	{Name: "Textarea", Kind: "input", Summary: "Multiline editing, soft-wrap, selection, and real cursor placement.", Detail: "Textarea shows the retained-widget pattern: the app owns the editor state, while the widget supplies text movement, wrapping, selection, and cursor-cell helpers."},
	{Name: "List", Kind: "selection", Summary: "Selection and scroll state over app-owned rows.", Detail: "List has no item data and no key policy. The app sets the count, moves the cursor, and renders each visible row with its own styling."},
	{Name: "Table", Kind: "data", Summary: "Aligned columns backed by List selection and keep-visible scrolling.", Detail: "Table composes reusable list state with column metadata. It keeps the terminal output deterministic while leaving row appearance to the application."},
	{Name: "Tree", Kind: "navigation", Summary: "Expandable hierarchy without a router or framework-owned focus.", Detail: "Tree owns expansion and cursor state only. Apps still decide how Tab, Enter, arrows, mouse, and details panes interact."},
	{Name: "Viewport", Kind: "scrolling", Summary: "Scrollable content window with hard-wrap and clipping support.", Detail: "Viewport is the supported way to keep body content scrollable while chrome stays pinned. Mouse wheel and keyboard paging are still app bindings."},
	{Name: "Progress", Kind: "feedback", Summary: "Styled progress bars with deterministic text output.", Detail: "Progress is a pure widget state. Resize handlers set widths, async ticks update percentages, and tests snapshot stable output."},
	{Name: "Spinner", Kind: "feedback", Summary: "Every-driven animation without goroutine-per-component state.", Detail: "Spinner pairs naturally with flatte.Every: one named update advances state, and View renders the current frame."},
	{Name: "Timer", Kind: "time", Summary: "Countdown and stopwatch helpers driven by fake-clock-testable updates.", Detail: "Timer and Stopwatch demonstrate async that remains deterministic under flatest.Driver and fake clock advancement."},
	{Name: "TabBar", Kind: "navigation", Summary: "Layout-aware tabs with rect-backed hit testing.", Detail: "TabBar emits layout IDs for the strip and each tab, so mouse hits use solved geometry instead of duplicated coordinate math."},
	{Name: "Layout", Kind: "composition", Summary: "One solve-and-compose pass for geometry, clipping, chrome, and overlays.", Detail: "flatui/layout is the default app composition vocabulary: build Node trees, call SolveAndCompose, and let rects and pixels come from the same walk."},
}

type State struct {
	activeTab landingTab
	searching bool
	search    flatui.TextField
	catalog   flatui.List
	detail    flatui.Viewport
	filtered  []int
	width     int
	height    int
	rects     map[string]layout.Rect
}

func NewState() *State {
	s := &State{
		width:  92,
		height: 28,
		rects:  map[string]layout.Rect{},
	}
	s.catalog.SetHeight(8)
	s.syncFiltered()
	s.syncDetail(40, 8)
	return s
}

func (s *State) layout(width, height int) {
	s.width = max(width, 56)
	s.height = max(height, 18)
	s.catalog.SetHeight(max(s.height-14, 4))
	s.syncDetail(max(s.width/3, 24), max(s.height-14, 4))
}

func (s *State) syncFiltered() {
	query := strings.ToLower(strings.TrimSpace(s.search.Value))
	s.filtered = s.filtered[:0]
	for i, item := range showcase {
		haystack := strings.ToLower(item.Name + " " + item.Kind + " " + item.Summary)
		if query == "" || strings.Contains(haystack, query) {
			s.filtered = append(s.filtered, i)
		}
	}
	s.catalog.SetCount(len(s.filtered))
}

func (s *State) selectedItem() showcaseItem {
	if len(s.filtered) == 0 {
		return showcaseItem{Name: "No match", Kind: "search", Summary: "No components match the current filter.", Detail: "Clear or edit the search query to return to the full component catalog."}
	}
	return showcase[s.filtered[s.catalog.Cursor()]]
}

func (s *State) syncDetail(width, height int) {
	item := s.selectedItem()
	s.detail.SetSize(max(width, 12), max(height, 1))
	s.detail.SetWrappedContent(item.Detail)
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(ev.Width, ev.Height)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	case flatte.MouseEvent:
		handleMouse(s, ev)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if s.searching {
		handleSearchKey(s, key, fx)
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		fx.Quit()
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			s.prevTab()
		} else {
			s.nextTab()
		}
	case flatte.KeyLeft:
		s.prevTab()
	case flatte.KeyRight:
		s.nextTab()
	case flatte.KeyUp:
		s.catalog.MoveUp()
		s.syncDetailFromRects()
	case flatte.KeyDown:
		s.catalog.MoveDown()
		s.syncDetailFromRects()
	case flatte.KeyPageUp:
		s.detail.PageUp()
	case flatte.KeyPageDown:
		s.detail.PageDown()
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'q', 'Q':
			fx.Quit()
		case '/', 's', 'S':
			s.activeTab = tabComponents
			s.searching = true
		case 'j', 'J':
			s.catalog.MoveDown()
			s.syncDetailFromRects()
		case 'k', 'K':
			s.catalog.MoveUp()
			s.syncDetailFromRects()
		case 'd', 'D':
			s.detail.HalfPageDown()
		case 'u', 'U':
			s.detail.HalfPageUp()
		}
	}
}

func handleSearchKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyEscape:
		s.searching = false
	case flatte.KeyEnter:
		s.searching = false
	case flatte.KeyBackspace:
		s.search.Backspace()
		s.syncFiltered()
		s.syncDetailFromRects()
	case flatte.KeyDelete:
		s.search.Delete()
		s.syncFiltered()
		s.syncDetailFromRects()
	case flatte.KeyLeft:
		s.search.MoveLeft()
	case flatte.KeyRight:
		s.search.MoveRight()
	case flatte.KeyCharacter:
		if key.Rune == 'q' && key.Mod.Contains(flatte.ModCtrl) {
			fx.Quit()
			return
		}
		s.search.Insert(key.Rune)
		s.syncFiltered()
		s.syncDetailFromRects()
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	if m.Action != flatte.MousePress {
		return
	}
	st := newStyles(defaultPalette())
	if i, ok := tabBar(s, st).HitTest(s.rects, m.X, m.Y); ok {
		s.activeTab = landingTab(i)
		s.searching = false
		return
	}
	if r, ok := s.rects["catalog-list"]; ok && r.Contains(m.X, m.Y) {
		s.activeTab = tabComponents
		if m.Button == flatte.MouseWheelDown {
			s.catalog.MoveDown()
		} else if m.Button == flatte.MouseWheelUp {
			s.catalog.MoveUp()
		} else {
			row := m.Y - r.Y - 3
			if row >= 0 {
				s.catalog.Select(s.catalog.Offset() + row)
			}
		}
		s.syncDetailFromRects()
	}
	if r, ok := s.rects["detail-pane"]; ok && r.Contains(m.X, m.Y) {
		if m.Button == flatte.MouseWheelDown {
			s.detail.LineDown(3)
		} else if m.Button == flatte.MouseWheelUp {
			s.detail.LineUp(3)
		}
	}
}

func (s *State) nextTab() { s.activeTab = (s.activeTab + 1) % landingTab(len(landingTabs)) }
func (s *State) prevTab() {
	s.activeTab = (s.activeTab - 1 + landingTab(len(landingTabs))) % landingTab(len(landingTabs))
}

func (s *State) syncDetailFromRects() {
	width, height := max(s.width/3, 24), max(s.height-14, 4)
	if r, ok := s.rects["detail-pane"]; ok {
		width = max(r.W-4, 12)
		height = max(r.H-4, 1)
	}
	s.syncDetail(width, height)
}

type palette struct {
	base     color.Color
	muted    color.Color
	panel    color.Color
	accent   color.Color
	accent2  color.Color
	selected color.Color
}

func defaultPalette() palette {
	return palette{
		base:     lipgloss.Color("252"),
		muted:    lipgloss.Color("245"),
		panel:    lipgloss.Color("239"),
		accent:   lipgloss.Color("117"),
		accent2:  lipgloss.Color("151"),
		selected: lipgloss.Color("229"),
	}
}

type styles struct {
	logo     lipgloss.Style
	title    lipgloss.Style
	lead     lipgloss.Style
	body     lipgloss.Style
	subtle   lipgloss.Style
	section  lipgloss.Style
	selected lipgloss.Style
	accent   lipgloss.Style
}

func newStyles(p palette) styles {
	base := lipgloss.NewStyle()
	return styles{
		logo:     base.Bold(true).Foreground(p.accent),
		title:    base.Bold(true).Foreground(p.base),
		lead:     base.Foreground(p.accent2),
		body:     base.Foreground(p.base),
		subtle:   base.Foreground(p.muted),
		section:  base.Bold(true).Foreground(p.accent2),
		selected: base.Bold(true).Foreground(p.selected),
		accent:   base.Foreground(p.accent),
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	st := newStyles(defaultPalette())
	width := max(ctx.Width, s.width)
	width = max(width, 56)
	height := max(s.height, 24)
	content, rects := layout.SolveAndCompose(landingTree(s, st), width, height)
	s.rects = rects
	s.syncDetailFromRects()
	return flatte.Frame{Content: trimRightLines(content), Title: "Flatte"}
}

type landingBlock struct {
	layout.NodeBase
	RenderContent func(layout.Rect, int) string
}

func (b landingBlock) Size() (layout.Size, layout.Size) { return b.NodeBase.Size() }

func (b landingBlock) Render(r layout.Rect) string {
	width := max(innerWidth(b.NodeBase, r.W), 1)
	content := ""
	if b.RenderContent != nil {
		content = b.RenderContent(r, width)
	}
	return layout.Text{NodeBase: b.NodeBase, String: content}.Render(r)
}

func innerWidth(base layout.NodeBase, width int) int {
	inset := base.PadLeft + base.PadRight + base.Pad*2
	if base.Bordered {
		inset += 2
	}
	return width - inset
}

func landingTree(s *State, st styles) layout.Node {
	panel := func(id string, weight float64, render func(layout.Rect, int) string) landingBlock {
		return landingBlock{
			NodeBase:      layout.NodeBase{ID: id, W: layout.Grow(weight), H: layout.Grow(1), Bordered: true, PadLeft: 1, PadRight: 1},
			RenderContent: render,
		}
	}
	return layout.Col{
		NodeBase: layout.NodeBase{ID: "page", Gap: 1, PadRight: 2, PadLeft: 2},
		Children: []layout.Node{
			landingBlock{NodeBase: layout.NodeBase{ID: "header", H: layout.Fixed(3)}, RenderContent: func(_ layout.Rect, width int) string {
				return headerView(st, width)
			}},
			tabBar(s, st).Layout(),
			layout.Row{
				NodeBase: layout.NodeBase{ID: "showcase", H: layout.Grow(1), Gap: 2},
				Children: []layout.Node{
					panel("catalog-list", 2, func(r layout.Rect, width int) string {
						s.catalog.SetHeight(max(r.H-5, 1))
						return catalogView(s, st, width)
					}),
					panel("detail-pane", 3, func(r layout.Rect, width int) string {
						s.syncDetail(width, max(r.H-4, 1))
						return detailView(s, st, width)
					}),
					panel("feature-pane", 2, func(_ layout.Rect, width int) string {
						return featureView(s, st, width)
					}),
				},
			},
			landingBlock{NodeBase: layout.NodeBase{ID: "footer", H: layout.Fixed(1)}, RenderContent: func(_ layout.Rect, width int) string {
				return footerView(s, st, width)
			}},
		},
	}
}

func tabBar(s *State, st styles) *flatui.TabBar {
	bar := flatui.NewTabBar(landingTabs...).
		WithID("tabs").
		WithGlyphs(flatui.TabGlyphsSafe).
		WithColors(defaultPalette().accent, defaultPalette().panel, nil)
	bar.SetActive(int(s.activeTab))
	_ = st
	return bar
}

func (t *State) activeTabName() string {
	if int(t.activeTab) < len(landingTabs) {
		return landingTabs[t.activeTab].Label
	}
	return ""
}

func headerView(st styles, width int) string {
	title := st.logo.Render("Flatte")
	claim := st.title.Render("Build TUIs like Go programs")
	lead := st.lead.Render("Direct mutation. Full-frame views. Deterministic tests. Browser WASM.")
	return strings.Join([]string{title + "  " + claim, lead, st.accent.Render(strings.Repeat("─", max(min(width, 76), 1)))}, "\n")
}

func catalogView(s *State, st styles, width int) string {
	query := s.search.Value
	if query == "" {
		query = "type / to search"
	}
	if s.searching {
		query = "search: " + s.search.Value + "▌"
	} else {
		query = "search: " + query
	}
	rows := []string{st.section.Render("Component catalog"), st.subtle.Render(truncate(query, width))}
	list := s.catalog.View(func(index int, selected bool) string {
		item := showcase[s.filtered[index]]
		prefix := "  "
		style := st.body
		if selected {
			prefix = "▸ "
			style = st.selected
		}
		return style.Render(truncate(prefix+item.Name, width))
	})
	if list != "" {
		rows = append(rows, strings.Split(list, "\n")...)
	}
	if len(s.filtered) == 0 {
		rows = append(rows, st.subtle.Render("No matches"))
	}
	return strings.Join(rows, "\n")
}

func detailView(s *State, st styles, width int) string {
	item := s.selectedItem()
	rows := []string{
		st.section.Render(item.Name),
		st.accent.Render(strings.ToUpper(item.Kind)),
		st.body.Render(wrapText(item.Summary, width)),
		"",
		s.detail.View(),
	}
	return strings.Join(rows, "\n")
}

func featureView(s *State, st styles, width int) string {
	switch s.activeTab {
	case tabOverview:
		return overviewView(st, width)
	case tabArchitecture:
		return architectureView(st, width)
	default:
		return componentsView(s, st, width)
	}
}

func overviewView(st styles, width int) string {
	rows := []string{
		st.section.Render("Why Flatte"),
		st.body.Render(wrapText("A Go-native TUI loop: mutate one state struct, render one frame, test the same flow without a terminal.", width)),
		"",
		st.selected.Render("TinyGo first"),
		st.subtle.Render(wrapText("The same app core builds to terminal and browser WASM.", width)),
	}
	return strings.Join(rows, "\n")
}

func componentsView(s *State, st styles, width int) string {
	item := s.selectedItem()
	rows := []string{
		st.section.Render("Live widget surface"),
		st.body.Render(wrapText("Use / to filter, j/k or arrows to select, mouse wheel to scroll detail.", width)),
		"",
		st.selected.Render(item.Name),
		st.subtle.Render(wrapText(item.Summary, width)),
	}
	return strings.Join(rows, "\n")
}

func architectureView(st styles, width int) string {
	rows := []string{
		st.section.Render("Loop shape"),
		st.body.Render("State"),
		st.accent.Render("  ↓ Handle(event) mutates"),
		st.body.Render("View(state)"),
		st.accent.Render("  ↓"),
		st.body.Render("Frame"),
		"",
		st.subtle.Render(wrapText("Async returns named StateUpdates; layout solves one node tree into pixels and rects.", width)),
	}
	return strings.Join(rows, "\n")
}

func footerView(s *State, st styles, width int) string {
	mode := "tab " + s.activeTabName()
	if s.searching {
		mode = "searching"
	}
	text := mode + "  / search  tab switch  j/k move  wheel scroll  q quit"
	if lipgloss.Width(text) > width {
		text = "/ search  tab switch  j/k move  q quit"
	}
	return st.subtle.Render(truncate(text, width))
}

func architectureCopy() string {
	return "State is the single source of truth. Handle mutates it directly in response to terminal events. View is pure: state in, Frame out. Layout nodes solve geometry and render into one composed cell buffer, so hit-test rects and pixels come from the same pass."
}

func cssPixels(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "px")
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

type sgrState struct {
	bold bool
	fg   string
}

func ansiToHTML(s string) string {
	var out strings.Builder
	state := sgrState{}
	open := false
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			end := i + 2
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end < len(s) {
				if open {
					out.WriteString("</span>")
					open = false
				}
				state = applySGR(state, s[i+2:end])
				if state.bold || state.fg != "" {
					out.WriteString(`<span style="`)
					if state.bold {
						out.WriteString("font-weight:700;")
					}
					if state.fg != "" {
						out.WriteString("color:")
						out.WriteString(state.fg)
						out.WriteByte(';')
					}
					out.WriteString(`">`)
					open = true
				}
				i = end + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			out.WriteRune(r)
			i++
			continue
		}
		out.WriteString(html.EscapeString(string(r)))
		i += size
	}
	if open {
		out.WriteString("</span>")
	}
	return out.String()
}

func applySGR(state sgrState, params string) sgrState {
	if params == "" {
		return sgrState{}
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch code {
		case 0:
			state = sgrState{}
		case 1:
			state.bold = true
		case 22:
			state.bold = false
		case 39:
			state.fg = ""
		case 38:
			if i+2 < len(parts) && parts[i+1] == "5" {
				n, err := strconv.Atoi(parts[i+2])
				if err == nil {
					state.fg = xterm256(n)
				}
				i += 2
			}
		}
	}
	return state
}

func xterm256(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	base := []string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}
	if n < len(base) {
		return base[n]
	}
	if n >= 232 {
		level := 8 + (n-232)*10
		return rgb(level, level, level)
	}
	n -= 16
	return rgb(cubeLevel(n/36), cubeLevel((n/6)%6), cubeLevel(n%6))
}

func cubeLevel(n int) int {
	if n == 0 {
		return 0
	}
	return 55 + n*40
}

func rgb(r, g, b int) string { return "#" + hex2(r) + hex2(g) + hex2(b) }

func hex2(n int) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[n>>4], digits[n&15]})
}

func wrapText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var lines []string
	words := strings.Fields(s)
	line := ""
	for _, word := range words {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if lipgloss.Width(next) > width && line != "" {
			lines = append(lines, line)
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	out := ""
	for _, r := range s {
		next := out + string(r)
		if lipgloss.Width(next) > width {
			break
		}
		out = next
	}
	return out
}

func trimRightLines(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func runTerminal() error {
	return flatte.Run(context.Background(), flatte.App[State]{State: NewState(), Handle: Handle, View: View}, flatte.WithMouse(flatte.MouseModeCellMotion))
}

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
