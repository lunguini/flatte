package dockerapp

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

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
	width, height     int
	bodyYOffset       int
	listPaneWidth     int
	detailPaneWidth   int
	activityPaneWidth int
	bodyContentHeight int
	rects             map[string]layout.Rect // solved geometry, updated each layout pass

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
	// nextDisplayedLogSeq numbers applied display entries independently of the
	// retained 50-line window. A scoped value cancelled while queued was never
	// displayed and does not reserve a number; a restarted stream may reuse it
	// without creating duplicate visible entries.
	nextDisplayedLogSeq map[string]int
	logTarget           string
	logScope            *flatte.Scope
	zones               flatui.ZoneMap // measured list-row hit regions
	detailTabs          *flatui.TabBar // stats/logs/inspect tab strip with mouse support
	listHeight          int

	activity      []string
	statusLine    string
	followTail    bool
	drag          *dragState // non-nil while a divider is being dragged
	pendingCursor int        // restored from session after layout sets list count
}

type dragState struct {
	divider            int // 0 = list|detail, 1 = detail|activity
	startX             int
	startListWidth     int
	startActivityWidth int
}

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
		containers: append([]Container(nil), sampleContainers...),
		cpu:        flatui.NewProgress(18),
		mem:        flatui.NewProgress(18),
		followTail: true,
		detailTabs: flatui.NewTabBar(
			flatui.TabItem{ID: "stats", Label: "stats"},
			flatui.TabItem{ID: "logs", Label: "logs"},
			flatui.TabItem{ID: "inspect", Label: "inspect"},
		).WithGlyphs(pickGlyphs()).WithColors(pal.accent, pal.tabBg, pal.bg).WithID(detailTabsID),
	}
	c.focus.SetCount(3)
	c.focus.Select(focusList)
	// Keep the status line valid before the first resize/render. Rendering is a
	// pure projection; all recomputation happens on state mutation paths.
	c.recomputeStatusLine()
	return c
}

const statusLineRows = 1

// detailTabsID is the layout ID of the detail pane's tab strip, shared by the
// bar's WithID and the strip-rect it is hit-tested against.
const detailTabsID = "detailtabs"

// initLayout sets body dimensions and the user-owned pane widths. It does not
// solve geometry — that happens in adopt, fed by the single frame solve (View)
// or the absolute warm solve (resize/drag).
func (c *containersScreen) initLayout(width, height, bodyYOffset int) {
	c.width, c.height = width, height
	c.bodyYOffset = bodyYOffset
	c.focus.SetCount(3)

	// Initialize default pane widths on first layout; thereafter the user owns them
	if c.listPaneWidth == 0 {
		c.listPaneWidth = min(28, max(width/5, minListWidth))
	}
	if c.activityPaneWidth == 0 {
		c.activityPaneWidth = 24
	}
	c.clampPaneWidths(width)
}

// afterResize runs the one-time-per-resize content setup that depends on the
// sized widgets adopt has already produced: filter, restored cursor, detail
// widgets, status line, and the active detail tab.
func (c *containersScreen) afterResize() {
	if len(c.filtered) == 0 && len(c.containers) > 0 {
		c.refreshFilter()
	}
	if c.pendingCursor >= 0 && c.pendingCursor < c.list.Count() {
		c.list.Select(c.pendingCursor)
		c.recomputeDetailWidgets()
	}
	c.pendingCursor = -1
	c.syncDetail()
	c.recomputeStatusLine()
	c.detailTabs.SetActive(int(c.tab))
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

// listHeaderLines is the count of rows the list pane draws before the first
// list row: the filter line plus a blank separator row.
const listHeaderLines = 2

// listNode builds the list pane as a frame subtree: the filter line, a blank
// row, and the visible rows from List.Rows ("list:i", so hit-testing reads
// the solved row rects), beside the scrollbar column, inside the pane's
// padding (none on top, so the filter line is flush with the other panes'
// headers).
func (c *containersScreen) listNode() layout.Node {
	filterLine := "filter: " + c.filter.Value
	if c.filter.Value == "" {
		filterLine = "filter: (all)"
	}
	filterStyle := layout.TextStyle{}
	if c.focus.Focused(focusFilter) {
		filterStyle = layout.TextStyle{Bold: true, Fg: pal.accent}
	}
	var rows layout.Node = layout.Label{Text: "  (no matches)", Style: layout.TextStyle{Fg: pal.muted}}
	if c.list.Count() > 0 {
		rows = c.list.Rows("list", c.listRow)
	}
	innerHeight := c.listHeight + listHeaderLines
	bar := scrollbarLines(c.list.Offset(), c.listHeight, c.list.Count(), innerHeight)
	if strings.TrimSpace(bar) != "" { // style the track only when there is one
		bar = lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	}
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID:        "list",
			W:         layout.Fixed(c.listPaneWidth),
			H:         layout.Auto(),
			PadLeft:   panePadding,
			PadRight:  panePadding,
			PadBottom: panePadding,
		},
		Children: []layout.Node{layout.Row{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Children: []layout.Node{
				layout.Label{Text: filterLine, Style: filterStyle},
				layout.Label{},
				rows,
			}},
			layout.Text{NodeBase: layout.NodeBase{W: layout.Fixed(1)}, String: bar},
		}}},
	}
}

// listRow is one container: its status (a dot or ring in a window, ● or ○
// in the terminal), the selection marker (a chevron, or "> "), and its name,
// bold when selected.
func (c *containersScreen) listRow(idx int, selected bool) layout.Node {
	if idx < 0 || idx >= len(c.filtered) {
		return layout.Label{}
	}
	ct := c.containers[c.filtered[idx]]
	status := layout.Icon{Name: layout.IconDot, Text: "●", Style: layout.TextStyle{Fg: pal.good}}
	if ct.Status != "running" {
		status = layout.Icon{Name: layout.IconCircle, Text: "○", Style: layout.TextStyle{Fg: pal.muted}}
	}
	marker, name := layout.Icon{Text: "  "}, layout.Label{Text: ct.Name}
	if selected {
		marker = layout.Icon{Name: layout.IconChevronRight, Text: "> "}
		name.Style = layout.TextStyle{Bold: true, Fg: pal.text}
	}
	return layout.Row{Children: []layout.Node{status, marker, name}}
}

// detailNode is the detail pane as a frame subtree: the header (name and
// tab strip), a blank row, and the active tab's body beside the scrollbar
// column, inside the pane's padding. On the logs tab the follow indicator
// takes the bottom padding row, right-aligned.
func (c *containersScreen) detailNode() layout.Node {
	pane := layout.Col{NodeBase: layout.NodeBase{
		ID:        "detail",
		W:         layout.Grow(1),
		H:         layout.Auto(),
		PadLeft:   panePadding,
		PadRight:  panePadding,
		PadBottom: panePadding,
	}}
	selected := c.selected()
	if selected == nil {
		pane.Children = []layout.Node{layout.Label{Text: "(no container selected)", Style: layout.TextStyle{Fg: pal.muted}}}
		return pane
	}
	body := append([]layout.Node{c.detailHeader(selected), layout.Label{}}, c.tabBody(selected)...)
	bar := c.detailScrollbar(max(c.bodyContentHeight-paneBorderRows, 0))
	if strings.TrimSpace(bar) != "" { // style the track only when there is one
		bar = lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	}
	pane.Children = []layout.Node{layout.Row{NodeBase: layout.NodeBase{H: layout.Grow(1)}, Children: []layout.Node{
		layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Children: body},
		layout.Text{NodeBase: layout.NodeBase{W: layout.Fixed(1)}, String: bar},
	}}}
	if indicator := c.followIndicator(); indicator != nil {
		pane.PadBottom = 0
		pane.Children = append(pane.Children, layout.Row{Children: []layout.Node{layout.NewSpacer(), indicator}})
	}
	return pane
}

// tabBody is the active tab's content: stats as labelled rows and meters,
// logs and inspect as their viewports.
func (c *containersScreen) tabBody(selected *Container) []layout.Node {
	indent := layout.NodeBase{PadLeft: 2}
	switch c.tab {
	case tabStats:
		status := layout.TextStyle{Fg: lipgloss.Color("203")}
		if selected.Status == "running" {
			status = layout.TextStyle{Fg: lipgloss.Color("114")}
		}
		// Names sit in a fixed column per group, so values (and the CPU and
		// MEM bars) line up in a proportional font too.
		field := func(name string, width int, value layout.Node) layout.Node {
			return layout.Row{NodeBase: indent, Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(width)}, Text: name},
				value,
			}}
		}
		return []layout.Node{
			field("status:", 8, layout.Label{Text: selected.Status, Style: status}),
			field("image:", 8, layout.Label{Text: selected.Image}),
			layout.Label{},
			field("CPU", 4, c.cpu.Layout()),
			field("hist", 5, layout.Text{String: sparkline(c.cpuHistory[selected.ID], lipgloss.Color("117"))}),
			layout.Label{},
			field("MEM", 4, c.mem.Layout()),
			field("hist", 5, layout.Text{String: sparkline(c.memHistory[selected.ID], lipgloss.Color("114"))}),
		}
	case tabLogs:
		if c.logs.VisibleLines() == 0 {
			return []layout.Node{layout.Label{Text: "(no logs)", Style: layout.TextStyle{Fg: pal.muted}}}
		}
		return []layout.Node{c.logs.Layout()}
	case tabInspect:
		if c.inspect.VisibleLines() == 0 {
			return []layout.Node{layout.Label{Text: "(no inspect data)", Style: layout.TextStyle{Fg: pal.muted}}}
		}
		return []layout.Node{c.inspect.Layout()}
	}
	return nil
}

// activityNode is the activity pane: a filled title row, a blank row, and
// the newest events that fit, each truncated to the pane.
func (c *containersScreen) activityNode() layout.Node {
	width := max(c.activityPaneWidth-paneBorderCols, 1)
	bodyHeight := max(c.bodyContentHeight-paneBorderRows-2, 0)
	visible := c.activity[max(len(c.activity)-bodyHeight, 0):]
	children := []layout.Node{
		layout.Row{NodeBase: layout.NodeBase{Box: layout.Box{Background: pal.accent}}, Children: []layout.Node{
			layout.Label{Text: "activity", Style: layout.TextStyle{Bold: true, Fg: pal.dark}},
		}},
		layout.Label{},
	}
	if len(visible) == 0 {
		children = append(children, layout.Label{Text: "(no events yet)", Style: layout.TextStyle{Fg: pal.muted}})
	}
	for _, line := range visible {
		children = append(children, layout.Label{Text: truncateToWidth(line, width)})
	}
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID: "activity", W: layout.Fixed(c.activityPaneWidth), H: layout.Auto(),
			PadLeft: panePadding, PadRight: panePadding, PadBottom: panePadding,
		},
		Children: children,
	}
}

// dividerNode is the containers body's divider index (0: list|detail,
// 1: detail|activity), in the accent while it is dragged.
func (c *containersScreen) dividerNode(index int) layout.Node {
	return dragDivider("div"+strconv.Itoa(index), c.drag != nil && c.drag.divider == index)
}

// statusNode is the status row: the containers summary on a filled line.
func (c *containersScreen) statusNode() layout.Node {
	return layout.Row{
		NodeBase: layout.NodeBase{ID: "status", H: layout.Fixed(statusLineRows), Box: layout.Box{Background: lipgloss.Color("236")}},
		Children: []layout.Node{layout.Label{Text: c.statusLine, Style: layout.TextStyle{Fg: lipgloss.Color("252")}}},
	}
}

// containersBodyTree builds the 3-pane containers body layout tree for Solve.
func containersBodyTree(c *containersScreen, status layout.Node) layout.Col {
	return layout.Col{
		Children: []layout.Node{
			layout.Row{
				NodeBase: layout.NodeBase{H: layout.Grow(1)},
				Children: []layout.Node{
					c.listNode(),
					c.dividerNode(0),
					c.detailNode(),
					c.dividerNode(1),
					c.activityNode(),
				},
			},
			status,
		},
	}
}

// bodyNode returns the containers body subtree (3 panes + dividers over a
// status row) as a frame child that grows to fill the body. status is the
// bottom status/command line (the statusNode, or the command bar).
func (c *containersScreen) bodyNode(status layout.Node) layout.Node {
	t := containersBodyTree(c, status)
	t.H = layout.Grow(1)
	return t
}

// adopt takes the solved frame geometry (absolute coordinates) and derives the
// containers screen's dimensions and widget sizes from it. This is the single
// site where pane geometry is consumed; the host solves the frame and delivers
// it as a LayoutEvent before painting.
func (c *containersScreen) adopt(rects map[string]layout.Rect) {
	c.rects = rects
	list := rects["list"]
	c.bodyYOffset = list.Y
	c.bodyContentHeight = list.H
	c.detailPaneWidth = rects["detail"].W

	// Size widgets from solved inner dimensions.
	listInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const listChromeRows = 2
	c.listHeight = max(listInnerHeight-listChromeRows, 0)
	c.list.SetHeight(c.listHeight)

	detailInnerWidth := detailContentWidth(c.detailPaneWidth)
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const detailChromeRows = 2
	contentHeight := max(detailInnerHeight-detailChromeRows, 0)
	c.logs.SetSize(detailInnerWidth, contentHeight)
	c.inspect.SetSize(detailInnerWidth, contentHeight)
	c.cpu.SetWidth(max(detailInnerWidth-16, 4))
	c.mem.SetWidth(max(detailInnerWidth-16, 4))

	c.registerListZones()
}

// registerListZones feeds list-row hit rects to the zone map straight from
// the frame-solved geometry. Each visible row is an ID'd node ("list:i"), so its
// rect is recorded by the single frame walk — paint and hit-testing read the
// same rects and can't drift.
func (c *containersScreen) registerListZones() {
	c.zones.Clear()
	for id, rect := range c.rects {
		if strings.HasPrefix(id, "list:") {
			c.zones.Set(id, rect)
		}
	}
}

// detailHeader builds the detail pane's header row: the container name, then
// the tab strip flush right. It is the detail pane's child in the frame tree.
//
// The name takes the remaining width and truncates first. A strip wider than
// the pane is clipped at the pane edge rather than wrapped onto a second row:
// clipped tabs stay where they are painted (and clickable while any of them
// shows), where a wrap would move them away from their solved positions.
func (c *containersScreen) detailHeader(selected *Container) layout.Node {
	return layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: selected.Name, Style: layout.TextStyle{Bold: true, Fg: pal.dark}},
		c.detailTabs.Layout(),
	}}
}

// detailContentWidth is the detail pane's content width: the pane minus its
// side padding and the scrollbar column.
func detailContentWidth(paneW int) int { return max(paneW-paneBorderCols-1, 1) }

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

func (c *containersScreen) applyDrag(root *State, currentX int) {
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
	c.detailTabs.SetActive(int(c.tab))
}

// Detail tab hit-testing goes through the frame's solved tab rects +
// TabBar.HitTest, not manual ZoneMap rectangles. See handleMouse for routing.

// dividerAt reports which divider (0 or 1) is at the given frame
// coordinates, using the absolute solved rects for hit-testing.
func (c *containersScreen) dividerAt(x, y int) int {
	if c.rects == nil {
		return -1
	}
	if c.rects["div0"].Contains(x, y) {
		return 0
	}
	if c.rects["div1"].Contains(x, y) {
		return 1
	}
	return -1
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
			c.applyDrag(root, m.X)
			return
		}
		return
	}

	// Start drag on press over a divider
	if m.Action == flatte.MousePress {
		if div := c.dividerAt(m.X, m.Y); div >= 0 {
			c.drag = &dragState{
				divider:            div,
				startX:             m.X,
				startListWidth:     c.listPaneWidth,
				startActivityWidth: c.activityPaneWidth,
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

	// Detail tab clicks: the header is in the frame tree, so each tab's rect
	// is the one the frame painted.
	if idx, ok := c.detailTabs.HitTest(c.rects, m.X, m.Y); ok {
		c.setTab(detailTab(idx))
		return
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

func (c *containersScreen) scrollAt(x, y, delta int) {
	if c.rects == nil {
		return
	}
	if c.rects["list"].Contains(x, y) {
		if delta > 0 {
			c.list.MoveDown()
		} else {
			c.list.MoveUp()
		}
		return
	}
	if c.rects["detail"].Contains(x, y) {
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
	c.recomputeStatusLine()
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
	if c.logScope != nil {
		c.logScope.Cancel()
		c.logScope = nil
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
	if c.nextDisplayedLogSeq == nil {
		c.nextDisplayedLogSeq = make(map[string]int)
	}

	c.logScope = flatte.NewScope(fx, "logs:"+ct.ID)
	targetID := ct.ID
	// Capture the sequence on the loop goroutine before starting the worker.
	// The worker must not read liveLogs: folds append to that map on the loop
	// goroutine, and reading it here would race with those appends.
	startSeq := c.nextDisplayedLogSeq[targetID]
	flatte.ScopeStream(c.logScope, fx, func(ctx context.Context, send func(string)) {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		streamScopedLogs(ctx, targetID, startSeq, ticker.C, send)
	}, func(s *State, line string) {
		s.containers.appendLiveLog(targetID, line)
	})
}

// streamScopedLogs emits a sequence of log lines without consulting app
// state. ticks is injected so the sequencing behavior can be tested without
// waiting on wall-clock time; production passes a ticker channel.
func streamScopedLogs(ctx context.Context, targetID string, startSeq int, ticks <-chan time.Time, send func(string)) {
	seq := startSeq
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			send(scopedLogLine(targetID, seq))
			seq++
		}
	}
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

// hostAdvanceLogs is the fx-free equivalent of the ScopeStream log feed, for
// when the app is hosted and the async engine is not running (e.g. browser
// WASM in the landing showcase). It appends one synthetic log line for the
// selected container, following selection like startScopedLogs does.
func (c *containersScreen) hostAdvanceLogs() {
	ct := c.selected()
	if ct == nil {
		c.logTarget = ""
		return
	}
	if c.liveLogs == nil {
		c.liveLogs = make(map[string][]string)
	}
	if c.nextDisplayedLogSeq == nil {
		c.nextDisplayedLogSeq = make(map[string]int)
	}
	c.logTarget = ct.ID
	seq := c.nextDisplayedLogSeq[ct.ID]
	c.appendLiveLog(ct.ID, scopedLogLine(ct.ID, seq))
}

func (c *containersScreen) appendLiveLog(id, line string) {
	if c.liveLogs == nil {
		c.liveLogs = make(map[string][]string)
	}
	if c.nextDisplayedLogSeq == nil {
		c.nextDisplayedLogSeq = make(map[string]int)
	}
	c.nextDisplayedLogSeq[id]++
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
	c.setTab((c.tab - 1 + 3) % 3)
}

func (c *containersScreen) nextTab() {
	c.setTab((c.tab + 1) % 3)
}

// setTab is the single write site for the active detail tab. It keeps the tab
// strip's highlight in sync here (in the Handle path) so no Render method has
// to mutate the bar to sync it.
func (c *containersScreen) setTab(t detailTab) {
	c.tab = t
	c.detailTabs.SetActive(int(t))
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
		root.commandModal = newCommand()
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

func (c *containersScreen) detailScrollbar(detailInnerHeight int) string {
	switch c.tab {
	case tabLogs:
		return scrollbarLines(c.logs.Offset(), c.logs.VisibleLines(), c.logs.TotalLines(), detailInnerHeight)
	case tabInspect:
		return scrollbarLines(c.inspect.Offset(), c.inspect.VisibleLines(), c.inspect.TotalLines(), detailInnerHeight)
	default:
		return strings.Repeat(" ", detailInnerHeight)
	}
}

// followIndicator is the logs' follow/paused indicator, or nil while the
// logs fit. It takes the detail pane's bottom padding row (not an extra body
// line, which would overflow the pane — see TTY-found bug 2026-06-25; and not
// the header row, where it pushed the tab strip out of place).
func (c *containersScreen) followIndicator() layout.Node {
	if c.logs.TotalLines() == 0 || c.logs.TotalLines() <= c.logs.VisibleLines() {
		return nil
	}
	if c.followTail {
		return layout.Label{Text: "↓ tail", Style: layout.TextStyle{Fg: pal.good}}
	}
	return layout.Label{Text: "↑ paused (G)", Style: layout.TextStyle{Fg: pal.muted}}
}
