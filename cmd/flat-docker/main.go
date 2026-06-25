package main

import (
	"context"
	"fmt"
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
	images     imagesScreen
	volumes    volumesScreen

	modal *confirmModel
}

type confirmModel struct {
	action     string
	targetID   string
	targetName string
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
	return s
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	if s.modal != nil {
		s.modal.Handle(s, ev, fx)
		return
	}
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.resize(e.Width, e.Height)
		return
	case flatte.KeyEvent:
		if handleGlobalKey(s, e, fx) {
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
	chromeRowsTop    = 2
	chromeRowsBottom = 1
)

func (s *State) resize(width, height int) {
	s.width, s.height = width, height
	bodyWidth := width
	bodyHeight := max(height-chromeRowsTop-chromeRowsBottom, 0)
	s.containers.layout(bodyWidth, bodyHeight)
	s.images.layout(bodyWidth, bodyHeight)
	s.volumes.layout(bodyWidth, bodyHeight)
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	width := ctx.Width
	if s.width > 0 {
		width = s.width
	}

	header := renderTabBar(s, width)
	separator := strings.Repeat("─", width)
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
	return flatte.Frame{
		Content: content,
		Title:   "flat-docker — " + s.screen.Name(),
	}
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
	labels := []struct {
		name   string
		active bool
	}{
		{"1 containers", s.screen == screenContainers},
		{"2 images", s.screen == screenImages},
		{"3 volumes", s.screen == screenVolumes},
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		text := " " + l.name + " "
		if l.active {
			text = "[" + l.name + "]"
		}
		if l.active {
			text = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")).Render(text)
		} else {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(text)
		}
		parts[i] = text
	}
	return strings.Join(parts, " ")
}

func renderFooter(s *State, width int) string {
	hints := ""
	switch s.screen {
	case screenContainers:
		hints = s.containers.keyHints()
	case screenImages:
		hints = "images screen"
	case screenVolumes:
		hints = "volumes screen"
	}
	help := " " + hints + " "
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(help)
	return styled
}

const (
	focusFilter = iota
	focusList
	focusDetail
)

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
	width, height   int
	listPaneWidth   int
	detailPaneWidth int

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
	liveLogs   map[string][]string
	logTarget  string
	logCancel  context.CancelFunc
	zones      flatui.ZoneMap
	listHeight int
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
	}
	c.focus.SetCount(3)
	c.focus.Select(focusList)
	return c
}

func (c *containersScreen) layout(width, height int) {
	c.width, c.height = width, height
	c.focus.SetCount(3)
	c.listPaneWidth = min(30, max(width/3, 16))
	c.detailPaneWidth = max(width-c.listPaneWidth-2, 0)
	const listChromeRows = 3
	c.listHeight = max(height-listChromeRows, 0)
	c.list.SetHeight(c.listHeight)

	const detailChromeRows = 3
	contentWidth := max(c.detailPaneWidth-2, 1)
	contentHeight := max(height-detailChromeRows, 0)
	c.logs.SetSize(contentWidth, contentHeight)
	c.inspect.SetSize(contentWidth, contentHeight)
	c.cpu.SetWidth(max(contentWidth-12, 4))
	c.mem.SetWidth(max(contentWidth-12, 4))

	if len(c.filtered) == 0 && len(c.containers) > 0 {
		c.refreshFilter()
	}
	c.syncDetail()
	c.registerMouseZones()
}

func (c *containersScreen) registerMouseZones() {
	c.zones.Clear()
	listStartY := chromeRowsTop + 2
	end := min(c.list.Offset()+c.listHeight, c.list.Count())
	for row := c.list.Offset(); row < end; row++ {
		i := row - c.list.Offset()
		c.zones.Set("list:"+strconv.Itoa(row), flatui.Rect{
			X: 0, Y: listStartY + i,
			Width: c.listPaneWidth, Height: 1,
		})
	}

	tabsY := chromeRowsTop + 1
	tabsX := c.listPaneWidth + 2
	c.zones.Set("tab:stats", flatui.Rect{X: tabsX, Y: tabsY, Width: 7, Height: 1})
	c.zones.Set("tab:logs", flatui.Rect{X: tabsX + 8, Y: tabsY, Width: 6, Height: 1})
	c.zones.Set("tab:inspect", flatui.Rect{X: tabsX + 15, Y: tabsY, Width: 9, Height: 1})
}

func (c *containersScreen) handleMouse(root *State, fx flatte.Effects[State], m flatte.MouseEvent) {
	if m.Action != flatte.MousePress {
		return
	}
	id, ok := c.zones.At(m.X, m.Y)
	if !ok {
		return
	}
	if id == "tab:stats" {
		c.tab = tabStats
		return
	}
	if id == "tab:logs" {
		c.tab = tabLogs
		return
	}
	if id == "tab:inspect" {
		c.tab = tabInspect
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
		c.logs.SetContent("")
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

	c.logs.SetContent(c.renderedLogsFor(ct.ID))
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
	st := c.statsCache[ct.ID]
	if st.Tick == 0 {
		st.CPU = sampleCPU(ct)
		st.MEM = sampleMEM(ct)
	}
	st.Tick++
	st.CPU = clampStat(st.CPU + stepFor(ct.ID, st.Tick, 7, 3))
	st.MEM = clampStat(st.MEM + stepFor(ct.Name, st.Tick, 5, 2))
	c.statsCache[ct.ID] = st
	c.cpu.SetPercent(st.CPU)
	c.mem.SetPercent(st.MEM)
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
	return fmt.Sprintf("2026-06-25 10:00:%02d req=%d src=%s msg=handling", seq%60, seq, id[:8])
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
		c.logs.SetContent(c.renderedLogsFor(id))
	}
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
	const ts = "2026-06-25 10:00:01"
	return strings.Join([]string{
		ts + " starting " + ct.Name + " (" + ct.ID + ")",
		ts + " image: " + ct.Image,
		ts + " status: " + ct.Status,
		ts + " ready",
		ts + " accepting traffic",
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
		}
	case flatte.KeyRight:
		if c.focus.Focused(focusFilter) {
			c.filter.MoveRight()
		}
	case flatte.KeyCharacter:
		c.handleChar(root, fx, key.Rune)
	}
}

func (c *containersScreen) handleChar(root *State, fx flatte.Effects[State], r rune) {
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
		case '[', 'h', 'H':
			c.prevTab()
		case ']', 'l', 'L':
			c.nextTab()
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
	case tabInspect:
		if delta > 0 {
			c.inspect.LineDown(delta)
		} else {
			c.inspect.LineUp(-delta)
		}
	}
}

func (c *containersScreen) View(_ *State) string {
	listPane := c.renderListPane()
	detailPane := c.renderDetailPane()
	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, "  ", detailPane)
}

func (c *containersScreen) keyHints() string {
	switch c.focus.Index() {
	case focusFilter:
		return "type filter  tab next  1/2/3 switch  q quit"
	case focusList:
		return "j/k move  s stop  x remove  tab next  1/2/3 switch  q quit"
	case focusDetail:
		switch c.tab {
		case tabStats:
			return "]/h/l switch tab  tab next  1/2/3 switch  q quit"
		case tabLogs:
			return "j/k scroll  ]/h/l switch tab  tab next  q quit"
		case tabInspect:
			return "j/k scroll  ]/h/l switch tab  tab next  q quit"
		}
	}
	return "1/2/3 switch  q quit"
}

func (c *containersScreen) renderListPane() string {
	style := lipgloss.NewStyle().Width(c.listPaneWidth)

	filterLine := "filter: " + c.filter.Value
	if c.filter.Value == "" {
		filterLine = "filter: (all)"
	}
	if c.focus.Focused(focusFilter) {
		filterLine = lipgloss.NewStyle().Bold(true).Render(filterLine)
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
		statusIcon := "●"
		if ct.Status != "running" {
			statusIcon = "○"
		}
		return fmt.Sprintf("%s%s%s", statusIcon, marker, ct.Name)
	})
	if listContent == "" {
		listContent = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  (no matches)")
	}

	return style.Render(filterLine + "\n\n" + listContent)
}

func (c *containersScreen) renderDetailPane() string {
	style := lipgloss.NewStyle().Width(c.detailPaneWidth)

	selected := c.selected()
	if selected == nil {
		return style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no container selected)"))
	}

	titleLine := selected.Name
	if c.focus.Focused(focusDetail) {
		titleLine = lipgloss.NewStyle().Bold(true).Render(titleLine)
	}

	tabLine := c.renderTabBar()
	body := c.renderActiveTab(selected)

	return style.Render(strings.Join([]string{titleLine, tabLine, "", body}, "\n"))
}

func (c *containersScreen) renderTabBar() string {
	tabs := []struct {
		name   string
		active bool
	}{
		{"stats", c.tab == tabStats},
		{"logs", c.tab == tabLogs},
		{"inspect", c.tab == tabInspect},
	}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		text := " " + t.name + " "
		if t.active {
			text = "[" + t.name + "]"
			text = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")).Render(text)
		} else {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(text)
		}
		parts[i] = text
	}
	return strings.Join(parts, " ")
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
		return strings.Join([]string{
			"  status: " + statusLine,
			"  image:  " + selected.Image,
			"",
			"  CPU " + c.cpu.View(),
			"  MEM " + c.mem.View(),
		}, "\n")
	case tabLogs:
		view := c.logs.View()
		if view == "" {
			view = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no logs)")
		}
		return view
	case tabInspect:
		view := c.inspect.View()
		if view == "" {
			view = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no inspect data)")
		}
		return view
	}
	return ""
}

type imagesScreen struct {
	width, height int
}

func (i *imagesScreen) layout(width, height int) {
	i.width, i.height = width, height
}

func (i *imagesScreen) Handle(_ *State, _ flatte.Event, _ flatte.Effects[State]) {}

func (i *imagesScreen) View(root *State) string {
	return placeholderBody(root.screen.Name(), i.width, i.height)
}

type volumesScreen struct {
	width, height int
}

func (v *volumesScreen) layout(width, height int) {
	v.width, v.height = width, height
}

func (v *volumesScreen) Handle(_ *State, _ flatte.Event, _ flatte.Effects[State]) {}

func (v *volumesScreen) View(root *State) string {
	return placeholderBody(root.screen.Name(), v.width, v.height)
}

func placeholderBody(screenName string, width, height int) string {
	lines := []string{
		"",
		fmt.Sprintf("  %s screen", screenName),
		fmt.Sprintf("  body: %d wide × %d tall", width, height),
		"",
		"  placeholder — task 2 adds real content",
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
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
}
