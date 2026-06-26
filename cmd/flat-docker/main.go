package main

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

type screen int

const (
	screenContainers screen = iota
	screenImages
	screenVolumes
)

func (sc screen) Name() string {
	switch sc {
	case screenContainers:
		return "containers"
	case screenImages:
		return "images"
	case screenVolumes:
		return "volumes"
	}
	return "unknown"
}

type State struct {
	width, height int
	screen        screen

	containers containersScreen
	images     *imagesScreen
	volumes    *volumesScreen

	modal      *confirmModel
	command    *commandModel
	cmdHistory []string
	headerTabs *tabBar
}

type confirmModel struct {
	action     string
	targetID   string
	targetName string
}

type commandModel struct {
	input   flatui.TextField
	histIdx int // -1 = editing fresh; >=0 = browsing history
}

func newCommand() *commandModel {
	return &commandModel{histIdx: -1}
}

func (m *commandModel) Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		s.command = nil
	case flatte.KeyEnter:
		line := strings.TrimSpace(m.input.Value)
		if line != "" {
			s.cmdHistory = append(s.cmdHistory, line)
		}
		m.execute(s, fx, line)
		s.command = nil
	case flatte.KeyBackspace:
		m.input.Backspace()
	case flatte.KeyDelete:
		m.input.Delete()
	case flatte.KeyLeft:
		m.input.MoveLeft()
	case flatte.KeyRight:
		m.input.MoveRight()
	case flatte.KeyUp:
		m.browseHistory(s, -1)
	case flatte.KeyDown:
		m.browseHistory(s, 1)
	case flatte.KeyCharacter:
		m.input.Insert(key.Rune)
	}
}

func (m *commandModel) browseHistory(s *State, dir int) {
	if len(s.cmdHistory) == 0 {
		return
	}
	if dir < 0 {
		if m.histIdx == -1 {
			m.histIdx = len(s.cmdHistory) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
	} else {
		if m.histIdx == -1 {
			return
		}
		m.histIdx++
		if m.histIdx >= len(s.cmdHistory) {
			m.histIdx = -1
			m.input.Value = ""
			m.input.SetCursor(0)
			return
		}
	}
	m.input.Value = s.cmdHistory[m.histIdx]
	m.input.SetCursor(len(m.input.Value))
}

func (m *commandModel) execute(s *State, _ flatte.Effects[State], line string) {
	if line == "" {
		s.containers.pushActivity("cmd  (empty)")
		return
	}
	s.containers.pushActivity("cmd  " + line)
	parts := strings.Fields(line)
	cmd, args := parts[0], parts[1:]
	switch cmd {
	case "filter":
		if len(args) > 0 {
			s.containers.filter.Value = strings.Join(args, " ")
			s.containers.filter.SetCursor(len(s.containers.filter.Value))
			s.containers.refreshFilter()
			s.containers.onSelectionChange(s, flatte.Effects[State]{})
			s.containers.pushActivity("→   filter applied: " + s.containers.filter.Value)
		} else {
			s.containers.filter.Value = ""
			s.containers.refreshFilter()
			s.containers.pushActivity("→   filter cleared")
		}
	case "stop":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		ct.Status = "exited"
		s.containers.recomputeDetailWidgets()
		s.containers.pushActivity("→   stopped " + ct.Name)
	case "remove":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		ct.Status = "removed"
		s.containers.recomputeDetailWidgets()
		s.containers.pushActivity("→   removed " + ct.Name)
	case "goto":
		if len(args) == 0 {
			s.containers.pushActivity("→   usage: goto <containers|images|volumes>")
			return
		}
		switch args[0] {
		case "containers", "1":
			s.screen = screenContainers
		case "images", "2":
			s.screen = screenImages
		case "volumes", "3":
			s.screen = screenVolumes
		default:
			s.containers.pushActivity("→   unknown screen: " + args[0])
			return
		}
	case "help":
		s.containers.pushActivity("→   filter <text> | stop | remove | goto <screen> | stream | help")
	case "stream":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		s.containers.injectStreamDemo(ct.ID)
		s.containers.pushActivity("→   streamed 8 demo log lines into " + ct.Name)
	case "q", "quit", "exit":
		s.containers.pushActivity("→   press q (lowercase, no colon) to quit")
	default:
		s.containers.pushActivity("→   unknown command: " + cmd + " (try :help)")
	}
}

func (c *containersScreen) injectStreamDemo(id string) {
	paragraphs := []struct {
		level logLevel
		msg   string
	}{
		{logInfo, "Begin streaming demonstration for " + id + " — this exercises soft-wrap on long ANSI-styled content, the auto-follow-tail behavior, and the level coloring (INFO green, WARN yellow, ERROR red, DEBUG muted)"},
		{logInfo, "Initializing subsystems: configuration loader, connection pool (size 16, timeout 30s), metrics emitter tracing cpu/mem/goroutines, health probe interval=10s"},
		{logDebug, "cache: warmed 248 entries from disk in 18ms; tier=hot size=248 evictions=0 hit_rate=94.2% (rolling 5m window)"},
		{logWarn, "deprecated config key 'storage.driver' detected at /etc/app/config.yaml:42 — will be removed in next major release; migrate to 'storage.backend'"},
		{logInfo, "listening on [::]:8080 (ipv6) and 0.0.0.0:8080 (ipv4); TLS not configured for this listener — proxy termination expected"},
		{logError, "upstream timeout after 30s while fetching user:42 from primary store; retrying with exponential backoff (attempt 1 of 5, next delay=2s)"},
		{logWarn, "retry succeeded but slow: request took 32.4s (threshold=10s); consider tightening the upstream timeout or scaling the primary store"},
		{logInfo, "streaming demonstration complete — 8 paragraphs, mix of INFO/DEBUG/WARN/ERROR, several lines longer than the viewport width to exercise wrap"},
	}
	for i, p := range paragraphs {
		ts := fmt.Sprintf("10:00:%02d", i)
		line := styledLogLine(ts, p.level, p.msg)
		c.appendLiveLog(id, line)
	}
}

func (m *commandModel) View(width int) string {
	prompt := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(":")
	input := m.input.Value
	if input == "" {
		placeholder := lipgloss.NewStyle().Foreground(pal.muted).Render("type a command (try :help)")
		return lipgloss.NewStyle().
			Width(width).
			Background(pal.bg).
			Foreground(pal.text).
			Render(prompt + " " + placeholder)
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(pal.bg).
		Foreground(pal.text).
		Render(prompt + " " + input)
}

func (m *commandModel) cursorFrameX() int {
	// ": " prompt = 2 cells, plus the input's cursor column
	return 2 + m.input.CursorColumn()
}

func (m *confirmModel) Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		s.modal = nil
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'y', 'Y':
			s.applyModalAction()
			s.modal = nil
		case 'n', 'N':
			s.modal = nil
		}
	}
}

func (s *State) applyModalAction() {
	if s.modal == nil {
		return
	}
	for i := range s.containers.containers {
		if s.containers.containers[i].ID == s.modal.targetID {
			switch s.modal.action {
			case "stop":
				s.containers.containers[i].Status = "exited"
			case "remove":
				s.containers.containers[i].Status = "removed"
			}
			break
		}
	}
	s.containers.recomputeDetailWidgets()
}

func (s *State) openConfirm(action string, ct *Container) {
	if ct == nil {
		return
	}
	s.modal = &confirmModel{
		action:     action,
		targetID:   ct.ID,
		targetName: ct.Name,
	}
}

func NewState() *State {
	s := &State{screen: screenContainers}
	s.containers = newContainersScreen()
	s.images = newImagesScreen()
	s.volumes = newVolumesScreen()
	s.headerTabs = newTabBar(
		tabItem{id: "containers", label: "1 containers"},
		tabItem{id: "images", label: "2 images"},
		tabItem{id: "volumes", label: "3 volumes"},
	)
	return s
}
func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	if s.modal != nil {
		s.modal.Handle(s, ev, fx)
		return
	}
	if s.command != nil {
		s.command.Handle(s, ev, fx)
		return
	}

	// Header tab mouse clicks — root-level, above screen dispatch.
	// composeHeader right-aligns the tabs; compute their frame X start.
	if m, ok := ev.(flatte.MouseEvent); ok && m.Action == flatte.MousePress && m.Y == 0 {
		totalTabsW := 0
		for _, item := range s.headerTabs.items {
			totalTabsW += tabLabelWidth(item.label)
		}
		tabStripStart := s.width - totalTabsW
		if tabStripStart < 0 {
			tabStripStart = 0
		}
		if m.X >= tabStripStart {
			if s.headerTabs.HandleMouseAt(m.X - tabStripStart) {
				s.screen = screen(s.headerTabs.Active())
				return
			}
		}
	}

	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.resize(e.Width, e.Height)
		return
	case flatte.KeyEvent:
		if handleGlobalKey(s, e, fx) {
			s.headerTabs.SetActive(int(s.screen))
			return
		}
	}

	switch s.screen {
	case screenContainers:
		s.containers.Handle(s, ev, fx)
	case screenImages:
		s.images.Handle(s, ev, fx)
	case screenVolumes:
		s.volumes.Handle(s, ev, fx)
	}
}

func handleGlobalKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) bool {
	if key.Key != flatte.KeyCharacter {
		return false
	}
	switch key.Rune {
	case '1':
		s.screen = screenContainers
		return true
	case '2':
		s.screen = screenImages
		return true
	case '3':
		s.screen = screenVolumes
		return true
	case 'q', 'Q':
		fx.Quit()
		return true
	}
	return false
}

const (
	chromeRowsTop    = 2 // tab bar + separator
	chromeRowsBottom = 2 // separator + footer (help line) — both anchored to bottom
)

type palette struct {
	accent color.Color
	panel  color.Color
	muted  color.Color
	text   color.Color
	good   color.Color
	bad    color.Color
	bg     color.Color
	tabBg  color.Color // inactive tab fill (slightly lighter than bg)
	dark   color.Color // high-contrast text for accent backgrounds
}

func defaultPalette() palette {
	return palette{
		accent: lipgloss.Color("117"),
		panel:  lipgloss.Color("240"),
		muted:  lipgloss.Color("245"),
		text:   lipgloss.Color("252"),
		good:   lipgloss.Color("114"),
		bad:    lipgloss.Color("203"),
		bg:     lipgloss.Color("236"),
		tabBg:  lipgloss.Color("238"),
		dark:   lipgloss.Color("23"),
	}
}

var pal = defaultPalette()

// glyphSet controls which characters tab edges use. Powerline requires
// the user's terminal font to include U+E0BA/U+E0BC (Nerd Font, etc.);
// Safe uses standard brackets that render in every terminal.
type glyphSet struct {
	left  string
	right string
}

var (
	glyphSafe = glyphSet{"[", "]"}
	glyphPower = glyphSet{powerlineLeftSlant, powerlineRightSlant}
)

// activeGlyphs is the process-wide choice, picked from $FLAT_DOCKER_GLYPHS
// at startup. Default is Powerline because the maintainers want the best
// visuals; users whose terminals can't render the PUA glyphs set
// FLAT_DOCKER_GLYPHS=safe to fall back to brackets.
var activeGlyphs = pickGlyphSet()

func pickGlyphSet() glyphSet {
	switch strings.ToLower(os.Getenv("FLAT_DOCKER_GLYPHS")) {
	case "safe", "ascii", "off", "0", "false":
		return glyphSafe
	default:
		return glyphPower
	}
}

const (
	powerlineLeftSlant  = "\ue0ba"
	powerlineRightSlant = "\ue0bc"
)

// powerlineTab renders a label as a Powerline-style tab: diagonal slant
// edges (filled triangles) on a colored fill, against a surrounding bar
// background. Returns the rendered string and its visible width.
func powerlineTab(label string, active bool) (string, int) {
	var fill, text color.Color
	if active {
		fill = pal.accent
		text = pal.dark
	} else {
		fill = pal.tabBg
		text = pal.muted
	}
	bar := pal.bg
	l := lipgloss.NewStyle().Foreground(fill).Background(bar).Render(activeGlyphs.left)
	b := lipgloss.NewStyle().Foreground(text).Background(fill).Padding(0, 2).Render(label)
	r := lipgloss.NewStyle().Foreground(fill).Background(bar).Render(activeGlyphs.right)
	rendered := l + b + r
	width := lipgloss.Width(rendered)
	return rendered, width
}

const paneBorderRows = 2 // top + bottom border (only when border=true)
const paneBorderCols = 2 // left + right border (only when border=true)
const panePadding = 1    // cells of padding on each side

func paneStyle(width, height int, focused bool, border bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		MaxWidth(width).
		Padding(panePadding, panePadding)
	if border {
		borderFg := pal.panel
		if focused {
			borderFg = pal.accent
		}
		s = s.Border(lipgloss.RoundedBorder()).BorderForeground(borderFg)
	}
	return s
}

// paneInnerDims returns the content area dimensions inside a pane,
// accounting for padding (and border if present).
func paneInnerDims(paneWidth, paneHeight int, border bool) (int, int) {
	cw := paneWidth - 2*panePadding
	ch := paneHeight - 2*panePadding
	if border {
		cw -= paneBorderCols
		ch -= paneBorderRows
	}
	return max(cw, 1), max(ch, 0)
}

// makeTitledTopBorder constructs a rounded top border with a title
// embedded: ╭─ title ────╮. The title sits inside the border line,
// using the border's foreground color. This is the standard TUI
// titled-border pattern — keeps the title visible even when the pane
// content area is too narrow for an in-content title row.
func makeTitledTopBorder(width int, title string, borderFg color.Color) string {
	inner := width - 2 // left corner + right corner
	titleText := ""
	if title != "" {
		titleText = " " + title + " "
	}
	titleW := lipgloss.Width(titleText)
	remaining := max(inner-titleW, 0)
	leftFill := min(1, remaining)
	rightFill := remaining - leftFill
	raw := "╭" + strings.Repeat("─", leftFill) + titleText + strings.Repeat("─", rightFill) + "╮"
	return lipgloss.NewStyle().Foreground(borderFg).Render(raw)
}

// renderTitledPane is retained for backwards compatibility but now
// simply renders with no border + padding (the title moves to a bg-filled
// header row inside the content, handled by the caller).
func renderTitledPane(width, height int, focused bool, title string, content string) string {
	_ = title
	return paneStyle(width, height, focused, false).Render(content)
}

func (s *State) resize(width, height int) {
	s.width, s.height = width, height
	bodyWidth := width
	bodyHeight := max(height-chromeRowsTop-chromeRowsBottom, 0)
	s.containers.layout(bodyWidth, bodyHeight)
	s.images.layout_(bodyWidth, bodyHeight)
	s.volumes.layout_(bodyWidth, bodyHeight)
}
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	width := ctx.Width
	if s.width > 0 {
		width = s.width
	}

	header := renderTabBar(s, width)
	separator := renderHeaderSeparator(width)
	footer := renderFooter(s, width)

	var body string
	switch s.screen {
	case screenContainers:
		body = s.containers.View(s)
	case screenImages:
		body = s.images.View(s)
	case screenVolumes:
		body = s.volumes.View(s)
	}

	content := strings.Join([]string{header, separator, body, separator, footer}, "\n")
	if s.modal != nil {
		content = flatui.Overlay(content, renderModal(s.modal))
	}
	if s.screen == screenContainers {
		s.containers.zones.Scan(content)
	}
	frame := flatte.Frame{
		Content: content,
		Title:   "flat-docker — " + s.screen.Name(),
	}
	if s.command != nil {
		frame.Cursor = commandCursor(content, s.command)
	}
	return frame
}

func commandCursor(content string, m *commandModel) *flatte.Cursor {
	const marker = "\x1b[1m\x1b[38;5;117m:\x1b[0m "
	stripped := flatestStripAnsi(content)
	for y, line := range strings.Split(stripped, "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		if !looksLikeCommandBar(line) {
			continue
		}
		return &flatte.Cursor{
			X: idx + 2 + m.input.CursorColumn(),
			Y: y,
		}
	}
	return nil
}

func flatestStripAnsi(s string) string {
	return lipgloss.NewStyle().Render(s)
}

func looksLikeCommandBar(line string) bool {
	// The command bar always sits on the dark bg (status line area).
	// After ANSI strip, an empty command bar is ": type a command (try :help)"
	// and a populated one is ": <input>".
	return strings.HasPrefix(strings.TrimLeft(line, " "), ":")
}

func renderModal(m *confirmModel) string {
	title := fmt.Sprintf(" %s container ", m.action)
	body := fmt.Sprintf("\n  %s %s?\n\n  y confirm   n/esc cancel\n", m.action, m.targetName)
	return lipgloss.NewStyle().
		Width(40).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("203")).
		Render(title + body)
}

func renderTabBar(s *State, width int) string {
	s.headerTabs.SetActive(int(s.screen))
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Padding(0, 1).Render("flat-docker")
	tabs := s.headerTabs.Render()
	return composeHeader(title, tabs, width, pal.bg)
}

func renderHeaderSeparator(width int) string {
	return lipgloss.NewStyle().
		Foreground(pal.accent).
		Render(strings.Repeat("━", width))
}

func renderFooter(s *State, width int) string {
	hints := ""
	if s.command != nil {
		hints = s.command.keyHints()
	} else {
		switch s.screen {
		case screenContainers:
			hints = s.containers.keyHints()
		case screenImages:
			hints = s.images.keyHints()
		case screenVolumes:
			hints = s.volumes.keyHints()
		}
	}
	help := " " + hints + " "
	styled := lipgloss.NewStyle().Foreground(pal.muted).Render(help)
	return styled
}

const (
	focusFilter = iota
	focusList
	focusDetail
)

func scrollbarLines(offset, visible, total, height int) string {
	if total <= visible || height <= 0 || visible <= 0 {
		return strings.Repeat(" ", height)
	}
	maxOffset := total - visible
	thumbSize := max(1, height*visible/total)
	if thumbSize > height {
		thumbSize = height
	}
	thumbPos := int(float64(offset) / float64(maxOffset) * float64(height - thumbSize))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > height {
		thumbPos = height - thumbSize
	}
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			rows[i] = "█"
		} else {
			rows[i] = "░"
		}
	}
	return strings.Join(rows, "\n")
}

func withScrollbar(content, bar string) string {
	if bar == "" {
		return content
	}
	contentLines := strings.Split(content, "\n")
	barLines := strings.Split(bar, "\n")
	n := max(len(contentLines), len(barLines))
	out := make([]string, n)
	for i := 0; i < n; i++ {
		var line, barChar string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		if i < len(barLines) {
			barChar = barLines[i]
		} else {
			barChar = " "
		}
		out[i] = line + barChar
	}
	return strings.Join(out, "\n")
}

type detailTab int

const (
	tabStats detailTab = iota
	tabLogs
	tabInspect
)

func (t detailTab) Name() string {
	switch t {
	case tabStats:
		return "stats"
	case tabLogs:
		return "logs"
	case tabInspect:
		return "inspect"
	}
	return "?"
}

type Container struct {
	ID, Name, Image, Status, Ports string
}

var sampleContainers = []Container{
	{ID: "a1b2c3d4e5", Name: "nginx-proxy", Image: "nginx:1.25", Status: "running", Ports: "0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp"},
	{ID: "b2c3d4e5f6", Name: "api-server", Image: "myapp/api:2.1", Status: "running", Ports: "0.0.0.0:8080->8080/tcp"},
	{ID: "c3d4e5f6g7", Name: "postgres", Image: "postgres:16", Status: "running", Ports: "5432/tcp"},
	{ID: "d4e5f6g7h8", Name: "redis-cache", Image: "redis:7", Status: "running", Ports: "6379/tcp"},
	{ID: "e5f6g7h8i9", Name: "frontend", Image: "myapp/web:3.0", Status: "exited", Ports: ""},
	{ID: "f6g7h8i9j0", Name: "worker", Image: "myapp/worker:2.1", Status: "running", Ports: ""},
	{ID: "g7h8i9j0k1", Name: "nginx-web", Image: "nginx:1.25", Status: "running", Ports: "80/tcp"},
	{ID: "h8i9j0k1l2", Name: "migration", Image: "myapp/migrate:1.4", Status: "exited", Ports: ""},
}

type containersScreen struct {
	width, height     int
	listPaneWidth     int
	detailPaneWidth   int
	activityPaneWidth int
	bodyContentHeight int

	focus      flatui.FocusRing
	filter     flatui.TextField
	list       flatui.List
	containers []Container
	filtered   []int

	tab     detailTab
	cpu     flatui.Progress
	mem     flatui.Progress
	logs    flatui.Viewport
	inspect flatui.Viewport

	statsCache map[string]containerStats
	cpuHistory map[string][]float64
	memHistory map[string][]float64
	liveLogs   map[string][]string
	logTarget  string
	logCancel  context.CancelFunc
	zones      *flatui.ZoneScanner // auto-zones for list rows
	detailTabs *tabBar             // stats/logs/inspect tab strip with mouse support
	listHeight int

	activity   []string
	statusLine string
	followTail bool
	drag       *dragState // non-nil while a divider is being dragged
}

type dragState struct {
	divider             int // 0 = list|detail, 1 = detail|activity
	startX              int
	startListWidth      int
	startActivityWidth  int
}

const (
	dividerWidth     = 1
	minListWidth     = 20
	minDetailWidth   = 20
	minActivityWidth = 12
)

type logLevel int

const (
	logInfo logLevel = iota
	logWarn
	logError
	logDebug
)

func (l logLevel) Name() string {
	switch l {
	case logInfo:
		return "INFO "
	case logWarn:
		return "WARN "
	case logError:
		return "ERROR"
	case logDebug:
		return "DEBUG"
	}
	return "?    "
}

func (l logLevel) Color() color.Color {
	switch l {
	case logInfo:
		return pal.good
	case logWarn:
		return lipgloss.Color("221")
	case logError:
		return pal.bad
	case logDebug:
		return pal.muted
	}
	return pal.text
}

func styledLogLine(ts string, level logLevel, msg string) string {
	tsStyled := lipgloss.NewStyle().Foreground(pal.muted).Render(ts)
	levelStyled := lipgloss.NewStyle().Bold(true).Foreground(level.Color()).Render(level.Name())
	return tsStyled + " " + levelStyled + " " + msg
}

type containerStats struct {
	CPU, MEM float64
	Tick     int
}

func newContainersScreen() containersScreen {
	c := containersScreen{
		containers: sampleContainers,
		cpu:        flatui.NewProgress(18),
		mem:        flatui.NewProgress(18),
		zones:      flatui.NewZoneScanner(),
		followTail: true,
		detailTabs: newTabBar(
			tabItem{id: "stats", label: "stats"},
			tabItem{id: "logs", label: "logs"},
			tabItem{id: "inspect", label: "inspect"},
		),
	}
	c.focus.SetCount(3)
	c.focus.Select(focusList)
	return c
}

const statusLineRows = 1

func (c *containersScreen) layout(width, height int) {
	c.width, c.height = width, height
	c.focus.SetCount(3)

	// Initialize default pane widths on first layout; thereafter the user owns them
	if c.listPaneWidth == 0 {
		c.listPaneWidth = min(28, max(width/5, minListWidth))
	}
	if c.activityPaneWidth == 0 {
		c.activityPaneWidth = 24
	}
	c.clampPaneWidths(width)
	c.detailPaneWidth = max(width-c.listPaneWidth-c.activityPaneWidth-2*dividerWidth, minDetailWidth)

	c.bodyContentHeight = max(height-statusLineRows, 0)

	listInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const listChromeRows = 2
	c.listHeight = max(listInnerHeight-listChromeRows, 0)
	c.list.SetHeight(c.listHeight)

	detailInnerWidth := max(c.detailPaneWidth-paneBorderCols-1, 1)
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const detailChromeRows = 2 // title+tabs row + blank (title merged into tab bar via composeHeader)
	contentHeight := max(detailInnerHeight-detailChromeRows, 0)
	c.logs.SetSize(detailInnerWidth, contentHeight)
	c.inspect.SetSize(detailInnerWidth, contentHeight)
	c.cpu.SetWidth(max(detailInnerWidth-16, 4))
	c.mem.SetWidth(max(detailInnerWidth-16, 4))

	if len(c.filtered) == 0 && len(c.containers) > 0 {
		c.refreshFilter()
	}
	c.syncDetail()
	c.recomputeStatusLine()
	c.detailTabs.SetActive(int(c.tab))
}

// clampPaneWidths enforces minimums on list and activity widths given the
// total body width. Detail is derived after this runs; its minimum is
// enforced by shrinking list or activity here first.
func (c *containersScreen) clampPaneWidths(totalWidth int) {
	available := totalWidth - 2*dividerWidth
	// List and activity must leave room for minDetailWidth
	maxListActivity := available - minDetailWidth
	if c.listPaneWidth+c.activityPaneWidth > maxListActivity {
		overflow := c.listPaneWidth + c.activityPaneWidth - maxListActivity
		// Take half from each, respecting minimums
		fromList := min(overflow/2, c.listPaneWidth-minListWidth)
		fromActivity := min(overflow-fromList, c.activityPaneWidth-minActivityWidth)
		c.listPaneWidth -= fromList
		c.activityPaneWidth -= fromActivity
		// If still overflowing (both at minimum), force list down
		if c.listPaneWidth+c.activityPaneWidth > maxListActivity {
			c.listPaneWidth = minListWidth
			c.activityPaneWidth = max(maxListActivity-minListWidth, minActivityWidth)
		}
	}
	c.listPaneWidth = max(c.listPaneWidth, minListWidth)
	c.activityPaneWidth = max(c.activityPaneWidth, minActivityWidth)
}

func (c *containersScreen) applyDrag(currentX int) {
	if c.drag == nil {
		return
	}
	delta := currentX - c.drag.startX
	switch c.drag.divider {
	case 0: // list|detail: dragging right widens list
		newList := c.drag.startListWidth + delta
		maxList := c.width - minDetailWidth - c.activityPaneWidth - 2*dividerWidth
		c.listPaneWidth = max(minListWidth, min(newList, maxList))
	case 1: // detail|activity: dragging right narrows activity
		newActivity := c.drag.startActivityWidth - delta
		maxActivity := c.width - c.listPaneWidth - minDetailWidth - 2*dividerWidth
		c.activityPaneWidth = max(minActivityWidth, min(newActivity, maxActivity))
	}
	c.detailPaneWidth = max(c.width-c.listPaneWidth-c.activityPaneWidth-2*dividerWidth, minDetailWidth)
	// Re-size dependent widgets
	detailInnerWidth := max(c.detailPaneWidth-paneBorderCols-1, 1)
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const detailChromeRows = 2
	contentHeight := max(detailInnerHeight-detailChromeRows, 0)
	c.logs.SetSize(detailInnerWidth, contentHeight)
	c.inspect.SetSize(detailInnerWidth, contentHeight)
	c.cpu.SetWidth(max(detailInnerWidth-16, 4))
	c.mem.SetWidth(max(detailInnerWidth-16, 4))
	c.detailTabs.SetActive(int(c.tab))
}

// registerTabZones was removed — detail tabs now use the tabBar component
// with built-in mouse support. Hit-testing is via HandleMouseAt, not
// manual ZoneMap rectangles. See handleMouse for the click routing.

// dividerAt reports which divider (0 or 1) is at the given frame
// coordinates, if any. Returns -1 if no divider is hit.
func (c *containersScreen) dividerAt(x, y int) int {
	bodyTop := chromeRowsTop
	bodyBottom := chromeRowsTop + c.bodyContentHeight
	if y < bodyTop || y >= bodyBottom {
		return -1
	}
	// Layout: list(listPaneWidth) | div0(dividerWidth) | detail(detailPaneWidth) | div1(dividerWidth) | activity
	div0X := c.listPaneWidth
	div1X := c.listPaneWidth + dividerWidth + c.detailPaneWidth
	if x == div0X {
		return 0
	}
	if x == div1X {
		return 1
	}
	return -1
}

func (c *containersScreen) renderDivider(idx int) string {
	return renderDragDivider(c.bodyContentHeight, c.drag != nil && c.drag.divider == idx)
}

// renderDragDivider produces a 1-col-wide, height-tall divider with a
// background color matching the pane borders, and a single │ in the
// vertical center as a visible grip indicator. The bg fills the column
// (spaces are invisible against it); the │ stands out as a contrast.
func renderDragDivider(height int, dragging bool) string {
	bg := pal.panel
	gripFg := pal.bg // dark against gray — clearly visible
	if dragging {
		bg = pal.accent
		gripFg = pal.dark // dark against blue
	}
	midY := height / 2
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if i == midY {
			rows[i] = "│"
		} else {
			rows[i] = " "
		}
	}
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(dividerWidth).
		Height(height).
		MaxHeight(height).
		Foreground(gripFg).
		Background(bg).
		Render(content)
}

func (c *containersScreen) recomputeStatusLine() {
	total := len(c.containers)
	running := 0
	var totalCPU, totalMEM float64
	for i := range c.containers {
		if c.containers[i].Status == "running" {
			running++
		}
		if st, ok := c.statsCache[c.containers[i].ID]; ok {
			totalCPU += st.CPU
			totalMEM += st.MEM
		}
	}
	avgCPU, avgMEM := 0.0, 0.0
	if running > 0 {
		avgCPU = totalCPU / float64(running)
		avgMEM = totalMEM / float64(running)
	}
	filterValue := c.filter.Value
	if filterValue == "" {
		filterValue = "(all)"
	}
	c.statusLine = fmt.Sprintf(" %d containers (%d running)  avg CPU %2.0f%%  avg MEM %2.0f%%  filter: %s ",
		total, running, avgCPU, avgMEM, filterValue)
}

func (c *containersScreen) handleMouse(root *State, fx flatte.Effects[State], m flatte.MouseEvent) {
	// Ongoing drag — track motion and release
	if c.drag != nil {
		if m.Action == flatte.MouseRelease || (m.Button == flatte.MouseNone && m.Action != flatte.MouseMotion) {
			c.drag = nil
			return
		}
		if m.Action == flatte.MouseMotion {
			c.applyDrag(m.X)
			return
		}
		return
	}

	// Start drag on press over a divider
	if m.Action == flatte.MousePress {
		if div := c.dividerAt(m.X, m.Y); div >= 0 {
			c.drag = &dragState{
				divider:             div,
				startX:              m.X,
				startListWidth:      c.listPaneWidth,
				startActivityWidth:  c.activityPaneWidth,
			}
			return
		}
	}

	// Wheel routing (even when not pressing a divider)
	if m.Button == flatte.MouseWheelUp || m.Button == flatte.MouseWheelDown {
		delta := 3
		if m.Button == flatte.MouseWheelUp {
			delta = -3
		}
		c.scrollAt(m.X, m.Y, delta)
		return
	}

	if m.Action != flatte.MousePress {
		return
	}

	// Detail tab clicks via tabBar component.
	// composeHeader right-aligns the tabs; compute the tab strip start.
	detailContentStartX := c.listPaneWidth + dividerWidth + 1
	detailContentWidth := c.detailPaneWidth - paneBorderCols
	detailHeaderY := chromeRowsTop + 1
	if m.Y == detailHeaderY && m.X >= detailContentStartX {
		totalTabsW := 0
		for _, item := range c.detailTabs.items {
			totalTabsW += tabLabelWidth(item.label)
		}
		tabStripStart := detailContentWidth - totalTabsW
		if tabStripStart < 0 {
			tabStripStart = 0
		}
		localX := m.X - detailContentStartX
		if localX >= tabStripStart {
			if c.detailTabs.HandleMouseAt(localX - tabStripStart) {
				c.tab = detailTab(c.detailTabs.Active())
				return
			}
		}
	}

	// Auto-zones for list rows
	id, ok := c.zones.At(m.X, m.Y)
	if !ok {
		return
	}
	if len(id) > 5 && id[:5] == "list:" {
		idx, err := strconv.Atoi(id[5:])
		if err != nil || idx < 0 || idx >= c.list.Count() {
			return
		}
		c.list.Select(idx)
		c.onSelectionChange(root, fx)
	}
}

func (c *containersScreen) scrollAt(x, _, delta int) {
	listPaneEnd := c.listPaneWidth
	detailPaneStart := listPaneEnd + dividerWidth
	detailPaneEnd := detailPaneStart + c.detailPaneWidth

	switch {
	case x < listPaneEnd:
		if delta > 0 {
			c.list.MoveDown()
		} else {
			c.list.MoveUp()
		}
	case x >= detailPaneStart && x < detailPaneEnd:
		c.scrollActiveTab(delta)
	}
}

func (c *containersScreen) refreshFilter() {
	q := strings.ToLower(strings.TrimSpace(c.filter.Value))
	c.filtered = c.filtered[:0]
	for i, ct := range c.containers {
		haystack := strings.ToLower(ct.Name + " " + ct.Image)
		if q == "" || strings.Contains(haystack, q) {
			c.filtered = append(c.filtered, i)
		}
	}
	c.list.SetCount(len(c.filtered))
	c.syncDetail()
}

func (c *containersScreen) syncDetail() {
	c.recomputeDetailWidgets()
}

func (c *containersScreen) onSelectionChange(root *State, fx flatte.Effects[State]) {
	c.recomputeDetailWidgets()
	c.startScopedLogs(root, fx)
}

func (c *containersScreen) recomputeDetailWidgets() {
	ct := c.selected()
	if ct == nil {
		c.inspect.SetContent("")
		c.logs.SetWrappedContent("")
		c.cpu.SetPercent(0)
		c.mem.SetPercent(0)
		return
	}
	c.inspect.SetContent(fmt.Sprintf(
		"name:    %s\nimage:   %s\nstatus:  %s\nports:   %s\nid:      %s\n\n-- labels --\n%[2]s.app=%[1]s\n%[2]s.role=service\n%[2]s.managed-by=docker\n\n-- mounts --\n/var/lib/%[6]s:/data\n/etc/%[6]s:/conf:ro\n\n-- networks --\nbridge\n\n-- restart policy --\nunless-stopped\n\n-- created --\n2026-06-20T08:00:00Z\n\n-- health --\nhealthy (5/5)",
		ct.Name, ct.Image, ct.Status, ct.Ports, ct.ID, strings.ReplaceAll(ct.Name, "-", "_"),
	))

	if st, ok := c.statsCache[ct.ID]; ok {
		c.cpu.SetPercent(st.CPU)
		c.mem.SetPercent(st.MEM)
	} else {
		c.cpu.SetPercent(sampleCPU(ct))
		c.mem.SetPercent(sampleMEM(ct))
	}

	c.logs.SetWrappedContent(c.renderedLogsFor(ct.ID))
}

func (c *containersScreen) renderedLogsFor(id string) string {
	history := sampleLogs(c.containers[c.indexOfID(id)])
	live := c.liveLogs[id]
	if len(live) == 0 {
		return history
	}
	return history + "\n" + strings.Join(live, "\n")
}

func (c *containersScreen) indexOfID(id string) int {
	for i, ct := range c.containers {
		if ct.ID == id {
			return i
		}
	}
	return 0
}

func (c *containersScreen) tickStats(_ time.Time) {
	ct := c.selected()
	if ct == nil {
		return
	}
	if c.statsCache == nil {
		c.statsCache = make(map[string]containerStats)
	}
	if c.cpuHistory == nil {
		c.cpuHistory = make(map[string][]float64)
	}
	if c.memHistory == nil {
		c.memHistory = make(map[string][]float64)
	}
	st := c.statsCache[ct.ID]
	if st.Tick == 0 {
		st.CPU = sampleCPU(ct)
		st.MEM = sampleMEM(ct)
	}
	st.Tick++
	st.CPU = clampStat(st.CPU + stepFor(ct.ID, st.Tick, 7, 3))
	st.MEM = clampStat(st.MEM + stepFor(ct.Name, st.Tick, 5, 2))
	c.statsCache[ct.ID] = st

	c.cpuHistory[ct.ID] = appendHistory(c.cpuHistory[ct.ID], st.CPU, 30)
	c.memHistory[ct.ID] = appendHistory(c.memHistory[ct.ID], st.MEM, 30)

	c.cpu.SetPercent(st.CPU)
	c.mem.SetPercent(st.MEM)

	c.pushActivity(fmt.Sprintf("stats  %s  CPU %2.0f%%  MEM %2.0f%%", ct.Name, st.CPU, st.MEM))
	c.recomputeStatusLine()
}

func appendHistory(h []float64, v float64, cap int) []float64 {
	h = append(h, v)
	if len(h) > cap {
		h = h[len(h)-cap:]
	}
	return h
}

func (c *containersScreen) pushActivity(line string) {
	c.activity = append(c.activity, line)
	if len(c.activity) > 200 {
		c.activity = c.activity[len(c.activity)-200:]
	}
}

func clampStat(v float64) float64 {
	return math.Max(5, math.Min(95, v))
}

func stepFor(seed string, tick, mod, amp int) float64 {
	var s float64
	for i, r := range seed {
		s += float64(r) * float64(i+1)
	}
	return math.Mod(s+float64(tick), float64(mod)) - float64(amp)
}

func (c *containersScreen) startScopedLogs(_ *State, fx flatte.Effects[State]) {
	if c.logCancel != nil {
		c.logCancel()
		c.logCancel = nil
	}
	ct := c.selected()
	if ct == nil {
		c.logTarget = ""
		return
	}
	if ct.ID == c.logTarget {
		return
	}
	c.logTarget = ct.ID
	if c.liveLogs == nil {
		c.liveLogs = make(map[string][]string)
	}

	parent := fx.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c.logCancel = cancel
	targetID := ct.ID
	updates := fx.Updates

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				line := scopedLogLine(targetID, len(c.liveLogs[targetID]))
				update := flatte.Named("scoped-log:"+targetID, func(s *State) {
					s.containers.appendLiveLog(targetID, line)
				})
				if updates == nil {
					return
				}
				select {
				case updates <- update:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

func scopedLogLine(id string, seq int) string {
	ts := fmt.Sprintf("10:00:%02d", seq%60)
	level := logLevel(seq % 4)
	messages := []string{
		fmt.Sprintf("req=%d src=%s msg=handling request, processing payload, dispatching to backend workers", seq, id[:8]),
		fmt.Sprintf("req=%d src=%s msg=connection accepted from gateway, negotiating protocol, stream established", seq, id[:8]),
		fmt.Sprintf("req=%d src=%s msg=cache miss for key user:%d, fetching from primary store, populating warm tier", seq, id[:8], seq*7),
		fmt.Sprintf("req=%d src=%s msg=health check passed (cpu/memory/goroutine count within thresholds), reporting healthy", seq, id[:8]),
	}
	msg := messages[seq%len(messages)]
	if level == logError {
		msg = fmt.Sprintf("req=%d src=%s msg=upstream timeout after 30s — retrying with exponential backoff (attempt %d)", seq, id[:8], seq/4+1)
	}
	return styledLogLine(ts, level, msg)
}

func (c *containersScreen) appendLiveLog(id, line string) {
	if c.liveLogs == nil {
		c.liveLogs = make(map[string][]string)
	}
	c.liveLogs[id] = append(c.liveLogs[id], line)
	if len(c.liveLogs[id]) > 50 {
		c.liveLogs[id] = c.liveLogs[id][len(c.liveLogs[id])-50:]
	}
	if sel := c.selected(); sel != nil && sel.ID == id {
		c.logs.SetWrappedContent(c.renderedLogsFor(id))
		if c.followTail {
			c.logs.GotoBottom()
		}
	}
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	c.pushActivity("log    " + shortID + "  " + truncateForActivity(ansiStrip(line)))
}

func ansiStrip(s string) string {
	out := make([]rune, 0, len(s))
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' || (r >= '@' && r <= '~') {
				inEscape = false
			}
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func truncateForActivity(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

func sampleCPU(ct *Container) float64 {
	var sum float64
	for _, r := range ct.ID {
		sum += float64(r)
	}
	return math.Mod(sum/13.0, 95.0) + 5.0
}

func sampleMEM(ct *Container) float64 {
	var sum float64
	for _, r := range ct.Name {
		sum += float64(r)
	}
	return math.Mod(sum/3.0, 90.0) + 5.0
}

func sampleLogs(ct Container) string {
	const ts = "10:00:01"
	return strings.Join([]string{
		styledLogLine(ts, logInfo, "starting "+ct.Name),
		styledLogLine(ts, logInfo, "image="+ct.Image),
		styledLogLine(ts, logInfo, "status="+ct.Status),
		styledLogLine(ts, logInfo, "listening on configured ports"),
		styledLogLine(ts, logWarn, "deprecated config key 'storage.driver' — will be removed next release"),
	}, "\n")
}

func (c *containersScreen) prevTab() {
	c.tab = (c.tab - 1 + 3) % 3
}

func (c *containersScreen) nextTab() {
	c.tab = (c.tab + 1) % 3
}

func (c *containersScreen) selected() *Container {
	if len(c.filtered) == 0 {
		return nil
	}
	idx := c.list.Cursor()
	if idx < 0 || idx >= len(c.filtered) {
		return nil
	}
	return &c.containers[c.filtered[idx]]
}

func (c *containersScreen) Handle(root *State, ev flatte.Event, fx flatte.Effects[State]) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		c.handleMouse(root, fx, m)
		return
	}
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			c.focus.Prev()
		} else {
			c.focus.Next()
		}
	case flatte.KeyUp:
		if c.focus.Focused(focusList) {
			c.list.MoveUp()
			c.onSelectionChange(root, fx)
		} else if c.focus.Focused(focusDetail) {
			c.scrollActiveTab(-1)
		}
	case flatte.KeyDown:
		if c.focus.Focused(focusList) {
			c.list.MoveDown()
			c.onSelectionChange(root, fx)
		} else if c.focus.Focused(focusDetail) {
			c.scrollActiveTab(1)
		}
	case flatte.KeyBackspace:
		if c.focus.Focused(focusFilter) {
			c.filter.Backspace()
			c.refreshFilter()
			c.onSelectionChange(root, fx)
		}
	case flatte.KeyDelete:
		if c.focus.Focused(focusFilter) {
			c.filter.Delete()
			c.refreshFilter()
			c.onSelectionChange(root, fx)
		}
	case flatte.KeyLeft:
		if c.focus.Focused(focusFilter) {
			c.filter.MoveLeft()
		} else if c.focus.Focused(focusDetail) {
			c.prevTab()
		}
	case flatte.KeyRight:
		if c.focus.Focused(focusFilter) {
			c.filter.MoveRight()
		} else if c.focus.Focused(focusDetail) {
			c.nextTab()
		}
	case flatte.KeyCharacter:
		c.handleChar(root, fx, key.Rune)
	}
}

func (c *containersScreen) handleChar(root *State, fx flatte.Effects[State], r rune) {
	if r == ':' && !c.focus.Focused(focusFilter) {
		root.command = newCommand()
		return
	}
	switch c.focus.Index() {
	case focusFilter:
		c.filter.Insert(r)
		c.refreshFilter()
		c.onSelectionChange(root, fx)
	case focusList:
		switch r {
		case 'j', 'J':
			c.list.MoveDown()
			c.onSelectionChange(root, fx)
		case 'k', 'K':
			c.list.MoveUp()
			c.onSelectionChange(root, fx)
		case 's', 'S':
			root.openConfirm("stop", c.selected())
		case 'x', 'X':
			root.openConfirm("remove", c.selected())
		}
	case focusDetail:
		switch r {
		case 'j', 'J':
			c.scrollActiveTab(1)
		case 'k', 'K':
			c.scrollActiveTab(-1)
		case 'G':
			c.resumeFollow()
		case '[', 'h', 'H':
			c.prevTab()
		case ']', 'l', 'L':
			c.nextTab()
		case 'f':
			c.pageActiveTab(1)
		case 'b':
			c.pageActiveTab(-1)
		case 'd':
			c.halfPageActiveTab(1)
		case 'u':
			c.halfPageActiveTab(-1)
		}
	}
}

func (c *containersScreen) pageActiveTab(dir int) {
	switch c.tab {
	case tabLogs:
		if dir > 0 {
			c.logs.PageDown()
		} else {
			c.logs.PageUp()
		}
		c.syncFollowTail()
	case tabInspect:
		if dir > 0 {
			c.inspect.PageDown()
		} else {
			c.inspect.PageUp()
		}
	}
}

func (c *containersScreen) halfPageActiveTab(dir int) {
	switch c.tab {
	case tabLogs:
		if dir > 0 {
			c.logs.HalfPageDown()
		} else {
			c.logs.HalfPageUp()
		}
		c.syncFollowTail()
	case tabInspect:
		if dir > 0 {
			c.inspect.HalfPageDown()
		} else {
			c.inspect.HalfPageUp()
		}
	}
}

func (c *containersScreen) scrollActiveTab(delta int) {
	switch c.tab {
	case tabLogs:
		if delta > 0 {
			c.logs.LineDown(delta)
		} else {
			c.logs.LineUp(-delta)
		}
		c.syncFollowTail()
	case tabInspect:
		if delta > 0 {
			c.inspect.LineDown(delta)
		} else {
			c.inspect.LineUp(-delta)
		}
	}
}

func (c *containersScreen) syncFollowTail() {
	c.followTail = c.logs.AtBottom()
}

func (c *containersScreen) resumeFollow() {
	c.followTail = true
	c.logs.GotoBottom()
}

func (c *containersScreen) View(root *State) string {
	listPane := c.renderListPane()
	detailPane := c.renderDetailPane()
	activityPane := c.renderActivityPane()
	div0 := c.renderDivider(0)
	div1 := c.renderDivider(1)
	mainRow := lipgloss.JoinHorizontal(lipgloss.Top, listPane, div0, detailPane, div1, activityPane)
	var statusRow string
	if root.command != nil {
		statusRow = root.command.View(c.width)
	} else {
		statusRow = c.renderStatusLine()
	}
	return strings.Join([]string{mainRow, statusRow}, "\n")
}

func (c *containersScreen) renderStatusLine() string {
	if c.statusLine == "" {
		c.recomputeStatusLine()
	}
	return lipgloss.NewStyle().
		Width(c.width).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Render(c.statusLine)
}

func (c *containersScreen) renderActivityPane() string {
	activityInnerWidth := max(c.activityPaneWidth-paneBorderCols, 1)
	activityInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(activityInnerWidth).Height(activityInnerHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render("activity")

	bodyHeight := max(activityInnerHeight-2, 0)
	start := len(c.activity) - bodyHeight
	if start < 0 {
		start = 0
	}
	visible := c.activity[start:]
	if len(visible) == 0 {
		visible = []string{lipgloss.NewStyle().Foreground(pal.muted).Render("(no events yet)")}
	}
	truncated := make([]string, len(visible))
	for i, line := range visible {
		truncated[i] = truncateToWidth(line, activityInnerWidth)
	}
	headerRow := lipgloss.NewStyle().Width(activityInnerWidth).Background(pal.bg).Render(title)
	content := headerRow + "\n\n" + strings.Join(truncated, "\n")
	inner := contentStyle.Render(content)
	return paneStyle(c.activityPaneWidth, c.bodyContentHeight, false, false).Render(inner)
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansiTruncate(s, width-1) + "…"
}

func ansiTruncate(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func (c *containersScreen) keyHints() string {
	switch c.focus.Index() {
	case focusFilter:
		return "type filter  tab next  1/2/3 switch  q quit"
	case focusList:
		return "j/k move  s stop  x remove  : command  tab next  1/2/3 switch  q quit"
	case focusDetail:
		switch c.tab {
		case tabStats:
			return "←/→ or ]/[ switch tab  : command  tab next  1/2/3 switch  q quit"
		case tabLogs:
			return "j/k scroll  f/b page  G follow  ←/→ or ]/[ switch tab  : command  tab next  q quit"
		case tabInspect:
			return "j/k scroll  f/b page  ←/→ or ]/[ switch tab  : command  tab next  q quit"
		}
	}
	return "1/2/3 switch  q quit"
}

func (m *commandModel) keyHints() string {
	return "enter execute  ↑↓ history  esc cancel"
}

func (c *containersScreen) renderListPane() string {
	listInnerWidth := max(c.listPaneWidth-paneBorderCols-1, 1) // -1 for scrollbar
	listInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(listInnerWidth).Height(listInnerHeight)

	filterLine := "filter: " + c.filter.Value
	if c.filter.Value == "" {
		filterLine = "filter: (all)"
	}
	if c.focus.Focused(focusFilter) {
		filterLine = lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(filterLine)
	}

	listContent := c.list.View(func(idx int, selected bool) string {
		if idx < 0 || idx >= len(c.filtered) {
			return ""
		}
		ct := c.containers[c.filtered[idx]]
		marker := "  "
		if selected {
			marker = "> "
		}
		statusColor := pal.good
		if ct.Status != "running" {
			statusColor = pal.bad
		}
		statusIcon := lipgloss.NewStyle().Foreground(statusColor).Render("●")
		if ct.Status != "running" {
			statusIcon = lipgloss.NewStyle().Foreground(pal.muted).Render("○")
		}
		name := ct.Name
		if selected {
			name = lipgloss.NewStyle().Bold(true).Foreground(pal.text).Render(name)
		}
		row := statusIcon + marker + name
		return flatui.Mark("list:"+strconv.Itoa(idx), row)
	})
	if listContent == "" {
		listContent = lipgloss.NewStyle().Foreground(pal.muted).Render("  (no matches)")
	}

	inner := contentStyle.Render(filterLine + "\n\n" + listContent)
	bar := scrollbarLines(c.list.Offset(), c.listHeight, c.list.Count(), listInnerHeight)
	barStyled := lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	combined := withScrollbar(inner, barStyled)
	return paneStyle(c.listPaneWidth, c.bodyContentHeight, c.focus.Focused(focusList) || c.focus.Focused(focusFilter), false).Render(combined)
}

func (c *containersScreen) renderDetailPane() string {
	detailInnerWidth := max(c.detailPaneWidth-paneBorderCols-1, 1)
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(detailInnerWidth).Height(detailInnerHeight)

	selected := c.selected()
	if selected == nil {
		empty := lipgloss.NewStyle().Foreground(pal.muted).Render("(no container selected)")
		return paneStyle(c.detailPaneWidth, c.bodyContentHeight, c.focus.Focused(focusDetail), false).Render(empty)
	}

	c.detailTabs.SetActive(int(c.tab))
	tabs := c.detailTabs.Render()
	indicator := c.renderFollowIndicator()
	rightPart := tabs
	if indicator != "" {
		rightPart = tabs + " " + indicator
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(selected.Name)
	headerRow := composeHeader(title, rightPart, detailInnerWidth, pal.bg)

	body := c.renderActiveTab(selected)
	inner := contentStyle.Render(strings.Join([]string{headerRow, "", body}, "\n"))
	bar := c.detailScrollbar()
	barStyled := lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	combined := withScrollbar(inner, barStyled)
	focused := c.focus.Focused(focusDetail)
	return paneStyle(c.detailPaneWidth, c.bodyContentHeight, focused, false).Render(combined)
}

func (c *containersScreen) detailScrollbar() string {
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	switch c.tab {
	case tabLogs:
		return scrollbarLines(c.logs.Offset(), c.logs.VisibleLines(), c.logs.TotalLines(), detailInnerHeight)
	case tabInspect:
		return scrollbarLines(c.inspect.Offset(), c.inspect.VisibleLines(), c.inspect.TotalLines(), detailInnerHeight)
	default:
		return strings.Repeat(" ", detailInnerHeight)
	}
}

func (c *containersScreen) renderTabBar() string {
	c.detailTabs.SetActive(int(c.tab))
	bar := c.detailTabs.Render()
	if indicator := c.renderFollowIndicator(); indicator != "" {
		bar += " " + indicator
	}
	return lipgloss.NewStyle().Background(pal.bg).Render(bar)
}

func (c *containersScreen) renderActiveTab(selected *Container) string {
	switch c.tab {
	case tabStats:
		statusLine := selected.Status
		if selected.Status == "running" {
			statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Render(selected.Status)
		} else {
			statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(selected.Status)
		}
		cpuSpark := sparkline(c.cpuHistory[selected.ID], lipgloss.Color("117"))
		memSpark := sparkline(c.memHistory[selected.ID], lipgloss.Color("114"))
		return strings.Join([]string{
			"  status: " + statusLine,
			"  image:  " + selected.Image,
			"",
			"  CPU " + c.cpu.View(),
			"  hist " + cpuSpark,
			"",
			"  MEM " + c.mem.View(),
			"  hist " + memSpark,
		}, "\n")
	case tabLogs:
		view := c.logs.View()
		if view == "" {
			view = lipgloss.NewStyle().Foreground(pal.muted).Render("(no logs)")
		}
		return view
	case tabInspect:
		view := c.inspect.View()
		if view == "" {
			view = lipgloss.NewStyle().Foreground(pal.muted).Render("(no inspect data)")
		}
		return view
	}
	return ""
}

// renderFollowIndicator returns the follow/paused indicator for the logs
// tab. Rendered inside the tab-bar row (NOT as an extra body line, which
// would overflow the pane's Height and corrupt the frame — see TTY-found
// bug 2026-06-25).
func (c *containersScreen) renderFollowIndicator() string {
	if c.logs.TotalLines() == 0 || c.logs.TotalLines() <= c.logs.VisibleLines() {
		return ""
	}
	if c.followTail {
		return lipgloss.NewStyle().Foreground(pal.good).Render("↓ tail")
	}
	return lipgloss.NewStyle().Foreground(pal.muted).Render("↑ paused (G)")
}

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(history []float64, c color.Color) string {
	if len(history) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no history)")
	}
	runes := make([]rune, len(history))
	for i, v := range history {
		idx := int(v/100.0*float64(len(sparkBlocks))) % len(sparkBlocks)
		if idx < 0 {
			idx = 0
		}
		runes[i] = sparkBlocks[idx]
	}
	return lipgloss.NewStyle().Foreground(c).Render(string(runes))
}

type Image struct {
	RepoTag, ID, Size, Created string
	Containers                 int
}

var sampleImages = []Image{
	{RepoTag: "nginx:1.25", ID: "sha256:a1b2c3d4", Size: "150MB", Created: "2026-06-15", Containers: 3},
	{RepoTag: "myapp/api:2.1", ID: "sha256:b2c3d4e5", Size: "210MB", Created: "2026-06-10", Containers: 2},
	{RepoTag: "postgres:16", ID: "sha256:c3d4e5f6", Size: "380MB", Created: "2026-05-20", Containers: 1},
	{RepoTag: "redis:7", ID: "sha256:d4e5f6g7", Size: "110MB", Created: "2026-05-15", Containers: 1},
	{RepoTag: "myapp/web:3.0", ID: "sha256:e5f6g7h8", Size: "280MB", Created: "2026-06-08", Containers: 0},
	{RepoTag: "myapp/worker:2.1", ID: "sha256:f6g7h8i9", Size: "190MB", Created: "2026-06-09", Containers: 1},
	{RepoTag: "myapp/migrate:1.4", ID: "sha256:g7h8i9j0", Size: "85MB", Created: "2026-06-05", Containers: 0},
}

type imagesScreen struct {
	width   int
	height  int
	focus   flatui.FocusRing
	list    flatui.List
	images  []Image
	layout  *paneLayout
}

const (
	imgFocusList = iota
	imgFocusDetail
)

func newImagesScreen() *imagesScreen {
	s := &imagesScreen{images: sampleImages}
	s.focus.SetCount(2)
	s.focus.Select(imgFocusList)
	s.list.SetCount(len(sampleImages))
	s.layout = newPaneLayout(
		paneEntry{
			id:       "list",
			minWidth: minListWidth,
			render:   s.renderImageListContent,
			onMouse:  s.handleImageListMouse,
		},
		paneEntry{
			id:       "detail",
			minWidth: minDetailWidth,
			render:   s.renderImageDetailContent,
		},
	)
	return s
}

func (i *imagesScreen) layout_(width, height int) {
	i.width, i.height = width, height
	i.focus.SetCount(2)
	i.layout.Layout(width, height, chromeRowsTop)
	i.list.SetHeight(max(height-paneBorderRows-2, 0))
}

func (i *imagesScreen) Handle(_ *State, ev flatte.Event, _ flatte.Effects[State]) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		i.layout.HandleMouse(m)
		return
	}
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			i.focus.Prev()
		} else {
			i.focus.Next()
		}
	case flatte.KeyUp:
		if i.focus.Focused(imgFocusList) {
			i.list.MoveUp()
		}
	case flatte.KeyDown:
		if i.focus.Focused(imgFocusList) {
			i.list.MoveDown()
		}
	case flatte.KeyCharacter:
		if i.focus.Focused(imgFocusList) {
			switch key.Rune {
			case 'j', 'J':
				i.list.MoveDown()
			case 'k', 'K':
				i.list.MoveUp()
			}
		}
	}
}

func (i *imagesScreen) View(_ *State) string {
	return i.layout.View()
}

func (i *imagesScreen) keyHints() string {
	if i.focus.Focused(imgFocusList) {
		return "j/k move  tab next  1/2/3 switch  q quit"
	}
	return "tab next  1/2/3 switch  q quit"
}

func (i *imagesScreen) selected() *Image {
	if i.list.Cursor() < 0 || i.list.Cursor() >= len(i.images) {
		return nil
	}
	return &i.images[i.list.Cursor()]
}

func (i *imagesScreen) renderImageListContent(contentWidth, contentHeight int) string {
	style := lipgloss.NewStyle().Width(contentWidth - 1).Height(contentHeight)
	scrollStyle := lipgloss.NewStyle().Height(contentHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render("images")
	content := i.list.View(func(idx int, selected bool) string {
		marker := "  "
		if selected {
			marker = "> "
		}
		name := i.images[idx].RepoTag
		if selected {
			name = lipgloss.NewStyle().Bold(true).Render(name)
		}
		return marker + name
	})
	inner := style.Render(title + "\n\n" + content)
	visible := contentHeight - 2
	bar := strings.Repeat(" ", contentHeight)
	if i.list.Count() > visible {
		bar = scrollbarLines(i.list.Offset(), visible, i.list.Count(), contentHeight)
	}
	return withScrollbar(inner, scrollStyle.Foreground(pal.panel).Render(bar))
}

func (i *imagesScreen) renderImageDetailContent(contentWidth, contentHeight int) string {
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	sel := i.selected()
	if sel == nil {
		return style.Render(lipgloss.NewStyle().Foreground(pal.muted).Render("(no image selected)"))
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(sel.RepoTag)
	rows := []string{
		title, "",
		"  id:         " + sel.ID,
		"  size:       " + sel.Size,
		"  created:    " + sel.Created,
		"  containers: " + strconv.Itoa(sel.Containers),
	}
	return style.Render(strings.Join(rows, "\n"))
}

func (i *imagesScreen) handleImageListMouse(m flatte.MouseEvent, localX, localY int) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		row := localY - 2
		if row >= 0 && row < i.list.Count() {
			i.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		i.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		i.list.MoveUp()
	}
}

type Volume struct {
	Name, Driver, Mountpoint, Size string
}

var sampleVolumes = []Volume{
	{Name: "data", Driver: "local", Mountpoint: "/var/lib/docker/volumes/data/_data", Size: "1.2GB"},
	{Name: "config", Driver: "local", Mountpoint: "/var/lib/docker/volumes/config/_data", Size: "12MB"},
	{Name: "logs", Driver: "local", Mountpoint: "/var/lib/docker/volumes/logs/_data", Size: "450MB"},
	{Name: "cache", Driver: "local", Mountpoint: "/var/lib/docker/volumes/cache/_data", Size: "890MB"},
	{Name: "backups", Driver: "local", Mountpoint: "/var/lib/docker/volumes/backups/_data", Size: "5.4GB"},
}

type volumesScreen struct {
	width   int
	height  int
	focus   flatui.FocusRing
	list    flatui.List
	volumes []Volume
	layout  *paneLayout
}

const (
	volFocusList = iota
	volFocusDetail
)

func newVolumesScreen() *volumesScreen {
	s := &volumesScreen{volumes: sampleVolumes}
	s.focus.SetCount(2)
	s.focus.Select(volFocusList)
	s.list.SetCount(len(sampleVolumes))
	s.layout = newPaneLayout(
		paneEntry{
			id:       "list",
			minWidth: minListWidth,
			render:   s.renderVolumeListContent,
			onMouse:  s.handleVolumeListMouse,
		},
		paneEntry{
			id:       "detail",
			minWidth: minDetailWidth,
			render:   s.renderVolumeDetailContent,
		},
	)
	return s
}

func (v *volumesScreen) layout_(width, height int) {
	v.width, v.height = width, height
	v.focus.SetCount(2)
	v.layout.Layout(width, height, chromeRowsTop)
	v.list.SetHeight(max(height-paneBorderRows-2, 0))
}

func (v *volumesScreen) Handle(_ *State, ev flatte.Event, _ flatte.Effects[State]) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		v.layout.HandleMouse(m)
		return
	}
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			v.focus.Prev()
		} else {
			v.focus.Next()
		}
	case flatte.KeyUp:
		if v.focus.Focused(volFocusList) {
			v.list.MoveUp()
		}
	case flatte.KeyDown:
		if v.focus.Focused(volFocusList) {
			v.list.MoveDown()
		}
	case flatte.KeyCharacter:
		if v.focus.Focused(volFocusList) {
			switch key.Rune {
			case 'j', 'J':
				v.list.MoveDown()
			case 'k', 'K':
				v.list.MoveUp()
			}
		}
	}
}

func (v *volumesScreen) View(_ *State) string {
	return v.layout.View()
}

func (v *volumesScreen) keyHints() string {
	if v.focus.Focused(volFocusList) {
		return "j/k move  tab next  1/2/3 switch  q quit"
	}
	return "tab next  1/2/3 switch  q quit"
}

func (v *volumesScreen) selected() *Volume {
	if v.list.Cursor() < 0 || v.list.Cursor() >= len(v.volumes) {
		return nil
	}
	return &v.volumes[v.list.Cursor()]
}

func (v *volumesScreen) renderVolumeListContent(contentWidth, contentHeight int) string {
	style := lipgloss.NewStyle().Width(contentWidth - 1).Height(contentHeight)
	scrollStyle := lipgloss.NewStyle().Height(contentHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render("volumes")
	content := v.list.View(func(idx int, selected bool) string {
		marker := "  "
		if selected {
			marker = "> "
		}
		name := v.volumes[idx].Name
		if selected {
			name = lipgloss.NewStyle().Bold(true).Render(name)
		}
		return marker + name
	})
	inner := style.Render(title + "\n\n" + content)
	visible := contentHeight - 2
	bar := strings.Repeat(" ", contentHeight)
	if v.list.Count() > visible {
		bar = scrollbarLines(v.list.Offset(), visible, v.list.Count(), contentHeight)
	}
	return withScrollbar(inner, scrollStyle.Foreground(pal.panel).Render(bar))
}

func (v *volumesScreen) renderVolumeDetailContent(contentWidth, contentHeight int) string {
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	sel := v.selected()
	if sel == nil {
		return style.Render(lipgloss.NewStyle().Foreground(pal.muted).Render("(no volume selected)"))
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(sel.Name)
	rows := []string{
		title, "",
		"  driver:     " + sel.Driver,
		"  mountpoint: " + sel.Mountpoint,
		"  size:       " + sel.Size,
	}
	return style.Render(strings.Join(rows, "\n"))
}

func (v *volumesScreen) handleVolumeListMouse(m flatte.MouseEvent, localX, localY int) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		row := localY - 2
		if row >= 0 && row < v.list.Count() {
			v.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		v.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		v.list.MoveUp()
	}
}

func main() {
	state := NewState()
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Init:   initAsync,
		Handle: Handle,
		View:   View,
	}, flatte.WithMouse(flatte.MouseModeCellMotion))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initAsync(s *State, fx flatte.Effects[State]) {
	flatte.Every(fx, "stats-poll", 1*time.Second, func(s *State, now time.Time) {
		s.containers.tickStats(now)
	})
	s.containers.startScopedLogs(s, fx)
	s.containers.pushActivity(tipForGlyphSet(activeGlyphs))
}

// tipForGlyphSet returns a one-time hint that explains the glyph choice
// and how to switch. Detection of "did the glyphs render" is not possible
// from inside a terminal — there is no feedback channel from the terminal
// back to the app about font coverage. The honest workaround is to inform
// the user once and let them pick via env var.
func tipForGlyphSet(g glyphSet) string {
	if g == glyphPower {
		return "tip   if tab edges look like boxes, install a Nerd Font or set FLAT_DOCKER_GLYPHS=safe"
	}
	return "tip   using safe (ASCII) glyphs; set FLAT_DOCKER_GLYPHS=powerline with a Nerd Font for nicer tabs"
}
