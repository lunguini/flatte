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
	logTarget  string
	logScope   *flatte.Scope
	zones      *flatui.ZoneScanner // auto-zones for list rows
	detailTabs *flatui.TabBar      // stats/logs/inspect tab strip with mouse support
	listHeight int

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
		containers: sampleContainers,
		cpu:        flatui.NewProgress(18),
		mem:        flatui.NewProgress(18),
		zones:      flatui.NewZoneScanner(),
		followTail: true,
		detailTabs: flatui.NewTabBar(
			flatui.TabItem{ID: "stats", Label: "stats"},
			flatui.TabItem{ID: "logs", Label: "logs"},
			flatui.TabItem{ID: "inspect", Label: "inspect"},
		).WithGlyphs(pickGlyphs()).WithColors(pal.accent, pal.tabBg, pal.bg).WithID(detailTabsID),
	}
	c.focus.SetCount(3)
	c.focus.Select(focusList)
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

// listNode builds the list pane as a real subtree: a Chrome-painted frame
// (filter line, separator, scrollbar, padding — reproducing renderListPane's
// non-row cells) with one ID'd Text per visible row composed on top. The row
// nodes give the frame solve authoritative per-row rects, so hit-testing reads
// rects["list:i"] instead of a parallel rowRects computation. Per-side padding
// keeps the rows out of the regions the Chrome paints: the filter/separator
// header rows on top, the left pad, and the right pad + scrollbar column.
func (c *containersScreen) listNode() layout.Node {
	offset := c.list.Offset()
	end := min(offset+c.listHeight, c.list.Count())
	rows := make([]layout.Node, 0, max(end-offset, 0))
	for i := offset; i < end; i++ {
		rows = append(rows, layout.Text{
			NodeBase: layout.NodeBase{ID: "list:" + strconv.Itoa(i), H: layout.Fixed(1)},
			String:   c.renderListRow(i, i == c.list.Cursor()),
		})
	}
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID:       "list",
			W:        layout.Fixed(c.listPaneWidth),
			H:        layout.Auto(),
			Chrome:   c.renderListChrome,
			PadTop:   listHeaderLines,
			PadLeft:  panePadding,
			PadRight: panePadding + 1, // right pad + scrollbar column
		},
		Children: rows,
	}
}

type containerDetailPane struct {
	layout.NodeBase
	screen *containersScreen
}

func newContainerDetailPane(c *containersScreen) containerDetailPane {
	return containerDetailPane{
		NodeBase: layout.NodeBase{ID: "detail", W: layout.Grow(1), H: layout.Auto()},
		screen:   c,
	}
}

func (p containerDetailPane) Render(r layout.Rect) string {
	if p.screen == nil {
		return ""
	}
	return p.screen.renderDetailPane(r)
}

type containerActivityPane struct {
	layout.NodeBase
	screen *containersScreen
}

func newContainerActivityPane(c *containersScreen) containerActivityPane {
	return containerActivityPane{
		NodeBase: layout.NodeBase{ID: "activity", W: layout.Fixed(c.activityPaneWidth), H: layout.Auto()},
		screen:   c,
	}
}

func (p containerActivityPane) Render(r layout.Rect) string {
	if p.screen == nil {
		return ""
	}
	return p.screen.renderActivityPane(r)
}

type containerDivider struct {
	layout.NodeBase
	screen *containersScreen
	index  int
}

func newContainerDivider(c *containersScreen, index int) containerDivider {
	return containerDivider{
		NodeBase: layout.NodeBase{ID: "div" + strconv.Itoa(index), W: layout.Fixed(dividerWidth), H: layout.Auto()},
		screen:   c,
		index:    index,
	}
}

func (d containerDivider) Render(r layout.Rect) string {
	if d.screen == nil {
		return ""
	}
	return d.screen.renderDivider(d.index, r)
}

type containerStatusRow struct {
	layout.NodeBase
	render func(layout.Rect) string
}

func newContainerStatusRow(render func(layout.Rect) string) containerStatusRow {
	return containerStatusRow{
		NodeBase: layout.NodeBase{ID: "status", W: layout.Auto(), H: layout.Fixed(statusLineRows)},
		render:   render,
	}
}

func (s containerStatusRow) Render(r layout.Rect) string {
	if s.render == nil {
		return ""
	}
	return s.render(r)
}

// containersBodyTree builds the 3-pane containers body layout tree for Solve.
func containersBodyTree(c *containersScreen, status func(layout.Rect) string) layout.Col {
	return layout.Col{
		Children: []layout.Node{
			layout.Row{
				NodeBase: layout.NodeBase{H: layout.Grow(1)},
				Children: []layout.Node{
					c.listNode(),
					newContainerDivider(c, 0),
					newContainerDetailPane(c),
					newContainerDivider(c, 1),
					newContainerActivityPane(c),
				},
			},
			newContainerStatusRow(status),
		},
	}
}

// bodyNode returns the containers body subtree (3 panes + dividers over a
// status row) as a frame child that grows to fill the body. status renders the
// bottom status/command line; pass nil when only solving for geometry.
func (c *containersScreen) bodyNode(status func(layout.Rect) string) layout.Node {
	t := containersBodyTree(c, status)
	t.H = layout.Grow(1)
	return t
}

// adopt takes the solved frame geometry (absolute coordinates) and derives the
// containers screen's dimensions and widget sizes from it. This is the single
// site where pane geometry is consumed — the solve itself happens once, in the
// frame solve (View) or the warm solve (resize/drag).
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

	detailInnerWidth := max(c.detailPaneWidth-paneBorderCols-1, 1)
	detailInnerHeight := max(c.bodyContentHeight-paneBorderRows, 0)
	const detailChromeRows = 2
	contentHeight := max(detailInnerHeight-detailChromeRows, 0)
	c.logs.SetSize(detailInnerWidth, contentHeight)
	c.inspect.SetSize(detailInnerWidth, contentHeight)
	c.cpu.SetWidth(max(detailInnerWidth-16, 4))
	c.mem.SetWidth(max(detailInnerWidth-16, 4))

	c.registerListZones()
}

// registerListZones feeds list-row hit rects to the zone scanner straight from
// the frame-solved geometry. Each visible row is an ID'd node ("list:i"), so its
// rect is recorded by the single frame walk — paint and hit-testing read the
// same rects and can't drift.
func (c *containersScreen) registerListZones() {
	c.zones.Reset()
	for id, rect := range c.rects {
		if strings.HasPrefix(id, "list:") {
			c.zones.Set(id, rect)
		}
	}
}

// detailTabStripRects returns the detail tab strip's absolute rect keyed by its
// layout ID. The strip is right-aligned in the pane's content header row (the
// same placement renderDetailPane composes), and its width is the strip's own
// natural layout size — so the per-label math stays private to the widget.
func (c *containersScreen) detailTabStripRects() map[string]layout.Rect {
	detailRect := c.rects["detail"]
	stripW, _ := c.detailTabs.Layout().Size()
	contentStartX := detailRect.X + panePadding
	contentWidth := detailRect.W - paneBorderCols
	localStart := contentWidth - stripW.Value
	if localStart < 0 || detailRect.W == 0 {
		return nil
	}
	return map[string]layout.Rect{
		detailTabsID: {
			X: contentStartX + localStart,
			Y: detailRect.Y,
			W: stripW.Value,
			H: 1,
		},
	}
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
	root.refreshActiveRects()
	c.detailTabs.SetActive(int(c.tab))
}

// Detail tab hit-testing goes through detailTabStripRects + TabBar.HitTest —
// solved geometry, not manual ZoneMap rectangles. See handleMouse for routing.

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

func (c *containersScreen) renderDivider(idx int, r layout.Rect) string {
	return renderDragDivider(r.H, c.drag != nil && c.drag.divider == idx)
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

	// Detail tab clicks. The detail pane composes its own header row internally
	// (it is a leaf), so per-tab rects aren't in the frame geometry; instead we
	// hand HitTest the tab strip's absolute rect and let it map the click via
	// its private label-width math. The strip is right-aligned in the pane's
	// content, matching renderDetailPane.
	if idx, ok := c.detailTabs.HitTest(c.detailTabStripRects(), m.X, m.Y); ok {
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

	c.logScope = flatte.NewScope(fx, "logs:"+ct.ID)
	targetID := ct.ID
	flatte.ScopeStream(c.logScope, fx, func(ctx context.Context, send func(string)) {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				send(scopedLogLine(targetID, len(c.liveLogs[targetID])))
			}
		}
	}, func(s *State, line string) {
		s.containers.appendLiveLog(targetID, line)
	})
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
	c.logTarget = ct.ID
	c.appendLiveLog(ct.ID, scopedLogLine(ct.ID, len(c.liveLogs[ct.ID])))
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

func (c *containersScreen) renderStatusLine(width int) string {
	if c.statusLine == "" {
		c.recomputeStatusLine()
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Render(c.statusLine)
}

func (c *containersScreen) renderActivityPane(r layout.Rect) string {
	activityInnerWidth := max(r.W-paneBorderCols, 1)
	activityInnerHeight := max(r.H-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(activityInnerWidth).Height(activityInnerHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.dark).Render("activity")

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
	headerRow := lipgloss.NewStyle().Width(activityInnerWidth).Background(pal.accent).Render(title)
	content := headerRow + "\n\n" + strings.Join(truncated, "\n")
	inner := contentStyle.Render(content)
	return paneStyle(r.W, r.H, false, false).Render(inner)
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

// renderListRow renders one list row exactly as the pane always has (status
// glyph, selection marker, name). It is the per-row node's paint, called from
// listNode with the absolute item index.
func (c *containersScreen) renderListRow(idx int, selected bool) string {
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
	return statusIcon + marker + name
}

// renderListChrome paints the pane frame — filter line, blank separator,
// scrollbar column, and padding — with a blank row area. The per-row Text
// children (see listNode) compose the rows on top of that blank area, so the
// non-row cells come from the same lipgloss pipeline the pane always used and
// stay byte-identical. When nothing matches, the frame carries the placeholder
// itself (there are no row children to draw it).
func (c *containersScreen) renderListChrome(r layout.Rect) string {
	rows := ""
	if c.list.Count() == 0 {
		rows = lipgloss.NewStyle().Foreground(pal.muted).Render("  (no matches)")
	}

	listInnerWidth := max(r.W-paneBorderCols-1, 1) // -1 for scrollbar
	listInnerHeight := max(r.H-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(listInnerWidth).Height(listInnerHeight)

	filterLine := "filter: " + c.filter.Value
	if c.filter.Value == "" {
		filterLine = "filter: (all)"
	}
	if c.focus.Focused(focusFilter) {
		filterLine = lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(filterLine)
	}

	inner := contentStyle.Render(filterLine + "\n\n" + rows)
	bar := scrollbarLines(c.list.Offset(), c.listHeight, c.list.Count(), listInnerHeight)
	barStyled := lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	combined := withScrollbar(inner, barStyled)
	return paneStyle(r.W, r.H, c.focus.Focused(focusList) || c.focus.Focused(focusFilter), false).Render(combined)
}

func (c *containersScreen) renderDetailPane(r layout.Rect) string {
	detailInnerWidth := max(r.W-paneBorderCols-1, 1)
	detailInnerHeight := max(r.H-paneBorderRows, 0)
	contentStyle := lipgloss.NewStyle().Width(detailInnerWidth).Height(detailInnerHeight)

	selected := c.selected()
	if selected == nil {
		empty := lipgloss.NewStyle().Foreground(pal.muted).Render("(no container selected)")
		return paneStyle(r.W, r.H, c.focus.Focused(focusDetail), false).Render(empty)
	}

	indicator := c.renderFollowIndicator()
	rightChildren := []layout.Node{c.detailTabs.Layout()}
	if indicator != "" {
		rightChildren = append(rightChildren, layout.Text{String: " " + indicator})
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.dark).Render(selected.Name)
	headerNode := layout.Row{
		Children: []layout.Node{
			layout.Text{String: title},
			layout.NewSpacer(),
			layout.Row{Children: rightChildren},
		},
	}
	// Compose at the row's natural width, not the (possibly narrower) pane
	// width, so an over-wide tab strip is preserved instead of clipped. The
	// enclosing contentStyle.Render(Width: detailInnerWidth) then wraps the
	// overflow onto a second line — the same behavior the retired join-based
	// layout.Render produced.
	composeW := detailInnerWidth
	if natW, _ := headerNode.Size(); natW.Value > composeW {
		composeW = natW.Value
	}
	headerRow, _ := layout.SolveAndCompose(headerNode, composeW, 1)

	body := c.renderActiveTab(selected)
	inner := contentStyle.Render(strings.Join([]string{headerRow, "", body}, "\n"))
	bar := c.detailScrollbar(detailInnerHeight)
	barStyled := lipgloss.NewStyle().Foreground(pal.panel).Render(bar)
	combined := withScrollbar(inner, barStyled)
	focused := c.focus.Focused(focusDetail)
	return paneStyle(r.W, r.H, focused, false).Render(combined)
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
