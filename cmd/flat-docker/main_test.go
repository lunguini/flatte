package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func resizedState(width, height int) *State {
	s := NewState()
	Handle(s, flatte.ResizeEvent{Width: width, Height: height}, flatte.Effects[State]{})
	return s
}

func TestResizePropagatesBodyDimensionsToEveryScreen(t *testing.T) {
	s := resizedState(80, 24)

	wantWidth := 80
	wantHeight := 24 - chromeRowsTop - chromeRowsBottom
	for _, c := range []struct {
		name string
		w, h int
	}{
		{"containers", s.containers.width, s.containers.height},
		{"images", s.images.width, s.images.height},
		{"volumes", s.volumes.width, s.volumes.height},
	} {
		if c.w != wantWidth || c.h != wantHeight {
			t.Fatalf("%s body = %d×%d, want %d×%d", c.name, c.w, c.h, wantWidth, wantHeight)
		}
	}
}

func TestNumberKeysSwitchScreens(t *testing.T) {
	s := resizedState(80, 24)

	Handle(s, keyChar('2'), flatte.Effects[State]{})
	if s.screen != screenImages {
		t.Fatalf("after 2: screen = %v, want images", s.screen)
	}

	Handle(s, keyChar('3'), flatte.Effects[State]{})
	if s.screen != screenVolumes {
		t.Fatalf("after 3: screen = %v, want volumes", s.screen)
	}

	Handle(s, keyChar('1'), flatte.Effects[State]{})
	if s.screen != screenContainers {
		t.Fatalf("after 1: screen = %v, want containers", s.screen)
	}
}

func TestQQuits(t *testing.T) {
	s := resizedState(80, 24)
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(s, keyChar('q'), fx)
	if !quit {
		t.Fatal("q did not quit")
	}
}

func TestNonGlobalKeyFallsThroughToActiveScreen(t *testing.T) {
	s := resizedState(80, 24)
	before := s.screen

	Handle(s, keyChar('x'), flatte.Effects[State]{})
	if s.screen != before {
		t.Fatalf("non-global key changed screen from %v to %v", before, s.screen)
	}
}

func TestViewTitleReflectsActiveScreen(t *testing.T) {
	s := resizedState(80, 24)

	if got := View(s, flatte.RenderContext{Width: 80}).Title; got != "flat-docker — containers" {
		t.Fatalf("title = %q", got)
	}

	s.screen = screenImages
	if got := View(s, flatte.RenderContext{Width: 80}).Title; got != "flat-docker — images" {
		t.Fatalf("title = %q", got)
	}
}

func TestViewRendersSharedChromeAndActiveScreenBody(t *testing.T) {
	s := resizedState(80, 24)

	content := View(s, flatte.RenderContext{Width: 80}).Content

	if !strings.Contains(content, "[1 containers]") {
		t.Fatalf("active tab not bracketed in:\n%s", content)
	}
	if !strings.Contains(content, "filter:") || !strings.Contains(content, "nginx-proxy") {
		t.Fatalf("containers list pane missing in:\n%s", content)
	}
	if !strings.Contains(content, "1/2/3 switch") {
		t.Fatalf("footer missing in:\n%s", content)
	}
}

func TestViewMatchesContainersGolden(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenContainers
	flatest.AssertGoldenFrame(t, "testdata/containers.golden", View(s, flatte.RenderContext{Width: 80}))
}

func TestViewMatchesImagesGolden(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenImages
	flatest.AssertGoldenFrame(t, "testdata/images.golden", View(s, flatte.RenderContext{Width: 80}))
}

func TestViewMatchesVolumesGolden(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenVolumes
	flatest.AssertGoldenFrame(t, "testdata/volumes.golden", View(s, flatte.RenderContext{Width: 80}))
}

func TestContainersFocusStartsOnList(t *testing.T) {
	s := resizedState(80, 24)
	if !s.containers.focus.Focused(focusList) {
		t.Fatalf("initial focus = %d, want %d (list)", s.containers.focus.Index(), focusList)
	}
}

func TestTabCyclesFocusForwardThroughListDetailFilter(t *testing.T) {
	s := resizedState(80, 24)
	want := []int{focusDetail, focusFilter, focusList}
	for _, w := range want {
		Handle(s, keyTab(false), flatte.Effects[State]{})
		if !s.containers.focus.Focused(w) {
			t.Fatalf("after Tab, focus = %d, want %d", s.containers.focus.Index(), w)
		}
	}
}

func TestShiftTabCyclesFocusBackward(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyTab(true), flatte.Effects[State]{})
	if !s.containers.focus.Focused(focusFilter) {
		t.Fatalf("after Shift-Tab from list, focus = %d, want %d (filter)", s.containers.focus.Index(), focusFilter)
	}
}

func TestFilterTypingNarrowsList(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)

	for _, r := range "nginx" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}

	want := []string{"nginx-proxy", "nginx-web"}
	if got := len(s.containers.filtered); got != len(want) {
		t.Fatalf("after filter 'nginx', filtered has %d items, want %d", got, len(want))
	}
	for i, name := range want {
		if got := s.containers.containers[s.containers.filtered[i]].Name; got != name {
			t.Fatalf("filtered[%d] = %q, want %q", i, got, name)
		}
	}
}

func TestFilterBackspaceRestoresItems(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	for _, r := range "nginx" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})

	if got := len(s.containers.filtered); got != 2 {
		t.Fatalf("after 'ngi' filter, filtered has %d items, want 2 (only nginx-proxy and nginx-web contain 'ngi')", got)
	}
}

func TestListMovementOnlyWhenListFocused(t *testing.T) {
	s := resizedState(80, 24)
	if !s.containers.focus.Focused(focusList) {
		t.Fatal("expected list focus")
	}
	startCursor := s.containers.list.Cursor()

	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.list.Cursor() != startCursor+1 {
		t.Fatalf("j on list focus: cursor = %d, want %d", s.containers.list.Cursor(), startCursor+1)
	}

	Handle(s, keyTab(false), flatte.Effects[State]{}) // list → detail
	Handle(s, keyChar('j'), flatte.Effects[State]{}) // detail ignores j
	if s.containers.list.Cursor() != startCursor+1 {
		t.Fatalf("j on detail focus should not move list, cursor = %d", s.containers.list.Cursor())
	}
}

func TestFilterTypingDoesNotMoveList(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	startCursor := s.containers.list.Cursor()

	Handle(s, keyChar('j'), flatte.Effects[State]{}) // j edits filter, not list
	if s.containers.list.Cursor() != startCursor {
		t.Fatalf("j on filter focus should not move list, cursor = %d", s.containers.list.Cursor())
	}
	if s.containers.filter.Value != "j" {
		t.Fatalf("filter = %q, want 'j'", s.containers.filter.Value)
	}
}

func TestDetailShowsSelectedContainer(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('j'), flatte.Effects[State]{}) // list focus, move to second item

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "api-server") {
		t.Fatalf("detail pane missing selected name in:\n%s", content)
	}
	if !strings.Contains(content, "myapp/api:2.1") {
		t.Fatalf("detail pane missing selected image in:\n%s", content)
	}
}

func TestDetailShowsEmptyStateWhenNoMatches(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	for _, r := range "zzz" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "no container selected") {
		t.Fatalf("empty detail state missing in:\n%s", content)
	}
}

func TestContainersLayoutSplitsBodyWidthBetweenPanes(t *testing.T) {
	s := resizedState(80, 24)
	c := &s.containers
	if c.listPaneWidth+c.detailPaneWidth+c.activityPaneWidth+4 != 80 {
		t.Fatalf("panes %d + %d + %d + 4 (gaps) != 80 (total)", c.listPaneWidth, c.detailPaneWidth, c.activityPaneWidth)
	}
	if c.activityPaneWidth == 0 {
		t.Fatalf("activity pane width is 0 — anchor-right pane missing")
	}
}

func TestDetailStartsOnStatsTab(t *testing.T) {
	s := resizedState(80, 24)
	if s.containers.tab != tabStats {
		t.Fatalf("initial tab = %v, want tabStats", s.containers.tab)
	}
}

func TestRightBracketSwitchesToNextTab(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)

	Handle(s, keyChar(']'), flatte.Effects[State]{})
	if s.containers.tab != tabLogs {
		t.Fatalf("after ] from stats, tab = %v, want logs", s.containers.tab)
	}

	Handle(s, keyChar(']'), flatte.Effects[State]{})
	if s.containers.tab != tabInspect {
		t.Fatalf("after ] from logs, tab = %v, want inspect", s.containers.tab)
	}

	Handle(s, keyChar(']'), flatte.Effects[State]{})
	if s.containers.tab != tabStats {
		t.Fatalf("after ] from inspect (wrap), tab = %v, want stats", s.containers.tab)
	}
}

func TestLeftBracketSwitchesToPreviousTab(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)

	Handle(s, keyChar('['), flatte.Effects[State]{})
	if s.containers.tab != tabInspect {
		t.Fatalf("after [ from stats (wrap), tab = %v, want inspect", s.containers.tab)
	}
}

func TestHLAlsoSwitchTabs(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)

	Handle(s, keyChar('l'), flatte.Effects[State]{})
	if s.containers.tab != tabLogs {
		t.Fatalf("after l, tab = %v, want logs", s.containers.tab)
	}

	Handle(s, keyChar('h'), flatte.Effects[State]{})
	if s.containers.tab != tabStats {
		t.Fatalf("after h, tab = %v, want stats", s.containers.tab)
	}
}

func TestTabSwitchKeysOnlyWorkWhenDetailFocused(t *testing.T) {
	s := resizedState(80, 24)
	// focus starts on list
	Handle(s, keyChar(']'), flatte.Effects[State]{})
	if s.containers.tab != tabStats {
		t.Fatalf("] on list focus changed tab to %v, want stats", s.containers.tab)
	}
}

func TestScrollOnlyAffectsActiveTab(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)

	// On stats tab: j does nothing (stats not scrollable)
	startCPUOffset := s.containers.logs.Offset()
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.logs.Offset() != startCPUOffset {
		t.Fatalf("j on stats tab moved logs offset: %d -> %d", startCPUOffset, s.containers.logs.Offset())
	}

	// Switch to logs, j scrolls logs
	Handle(s, keyChar(']'), flatte.Effects[State]{}) // stats -> logs
	logsBefore := s.containers.logs.Offset()
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.logs.Offset() == logsBefore && s.containers.logs.TotalLines() > s.containers.logs.VisibleLines() {
		t.Fatalf("j on logs tab did not scroll logs (offset stayed %d)", s.containers.logs.Offset())
	}

	// Switch to inspect, j scrolls inspect (not logs)
	Handle(s, keyChar(']'), flatte.Effects[State]{}) // logs -> inspect
	logsAfter := s.containers.logs.Offset()
	inspectBefore := s.containers.inspect.Offset()
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.logs.Offset() != logsAfter {
		t.Fatalf("j on inspect tab moved logs: %d -> %d", logsAfter, s.containers.logs.Offset())
	}
	if s.containers.inspect.Offset() == inspectBefore {
		t.Fatalf("j on inspect tab did not scroll inspect (offset stayed %d)", s.containers.inspect.Offset())
	}
}

func TestStatsTabShowsCPUMemBars(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabStats

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "CPU") || !strings.Contains(content, "MEM") {
		t.Fatalf("stats tab missing CPU/MEM in:\n%s", content)
	}
}

func TestLogsTabShowsLogLines(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "starting nginx-proxy") {
		t.Fatalf("logs tab missing log content in:\n%s", content)
	}
}

func TestInspectTabShowsContainerFields(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabInspect

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "name:") || !strings.Contains(content, "image:") {
		t.Fatalf("inspect tab missing container fields in:\n%s", content)
	}
	if !strings.Contains(content, "a1b2c3d4e5") {
		t.Fatalf("inspect tab missing container id in:\n%s", content)
	}
}

func TestTabBarShowsActiveTabBracketed(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "[logs]") {
		t.Fatalf("active tab not bracketed in:\n%s", content)
	}
	if strings.Contains(content, "[stats]") || strings.Contains(content, "[inspect]") {
		t.Fatalf("inactive tabs should not be bracketed in:\n%s", content)
	}
}

func TestChangingSelectedContainerUpdatesDetailTabs(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusList)

	Handle(s, keyChar('j'), flatte.Effects[State]{}) // move to api-server
	s.containers.tab = tabInspect

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "myapp/api:2.1") {
		t.Fatalf("inspect tab did not update after list move:\n%s", content)
	}
}

func TestStatsTickSidestepUpdatesCurrentContainerOnly(t *testing.T) {
	s := resizedState(80, 24)
	startCPU := s.containers.cpu.Percent()

	s.containers.tickStats(time.Now())

	if s.containers.cpu.Percent() == startCPU {
		t.Fatalf("stats tick did not change displayed CPU (start=%.1f, now=%.1f)", startCPU, s.containers.cpu.Percent())
	}
	st := s.containers.statsCache["a1b2c3d4e5"] // first container ID
	if st.Tick != 1 {
		t.Fatalf("statsCache tick = %d, want 1", st.Tick)
	}

	// Switch to second container and tick — first container's stats should not change
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	firstTick := s.containers.statsCache["a1b2c3d4e5"]
	s.containers.tickStats(time.Now())
	if s.containers.statsCache["a1b2c3d4e5"].Tick != firstTick.Tick {
		t.Fatalf("first container's stats changed while second was selected")
	}
	if _, ok := s.containers.statsCache["b2c3d4e5f6"]; !ok {
		t.Fatalf("second container not in statsCache after tick")
	}
}

func TestScopedLogsRestartOnSelectionChange(t *testing.T) {
	s := resizedState(80, 24)
	ctx := context.Background()
	fx := flatte.NewEffects[State](ctx, make(chan flatte.StateUpdate[State], 100), func() {})

	// Initial streamer for first container
	s.containers.startScopedLogs(s, fx)
	if s.containers.logCancel == nil {
		t.Fatal("no logCancel after first startScopedLogs")
	}
	if s.containers.logTarget != "a1b2c3d4e5" {
		t.Fatalf("logTarget = %q, want a1b2c3d4e5", s.containers.logTarget)
	}

	// Move selection to second container — should cancel old, start new
	Handle(s, keyChar('j'), fx)
	if s.containers.logCancel == nil {
		t.Fatal("logCancel became nil after selection change")
	}
	if s.containers.logTarget != "b2c3d4e5f6" {
		t.Fatalf("logTarget = %q after selection change, want b2c3d4e5f6", s.containers.logTarget)
	}
}

func TestScopedLogsCancelReleasesGoroutine(t *testing.T) {
	s := resizedState(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fx := flatte.NewEffects[State](ctx, make(chan flatte.StateUpdate[State], 100), func() {})

	s.containers.startScopedLogs(s, fx)
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	s.containers.logCancel() // cancel the streamer's own context
	// streamer should exit; verify the parent context still alive
	select {
	case <-done:
		t.Fatal("parent context cancelled by streamer cleanup")
	default:
	}
}

func TestAppendLiveLogUpdatesDisplayedViewportWhenSelectedMatches(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.tab = tabLogs

	before := s.containers.logs.TotalLines()
	s.containers.appendLiveLog("a1b2c3d4e5", "test live line")

	if s.containers.logs.TotalLines() <= before {
		t.Fatalf("logs viewport not updated after live append: before=%d after=%d", before, s.containers.logs.TotalLines())
	}
	view := s.containers.logs.View()
	if !strings.Contains(view, "test live line") {
		t.Fatalf("appended line not in logs view:\n%s", view)
	}
}

func TestAppendLiveLogDoesNotTouchOtherContainerBuffers(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.appendLiveLog("b2c3d4e5f6", "other container line")
	view := s.containers.logs.View()
	if strings.Contains(view, "other container line") {
		t.Fatalf("non-selected container's log leaked into displayed viewport:\n%s", view)
	}
}

func TestStopKeyOpensConfirmModal(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('s'), flatte.Effects[State]{})
	if s.modal == nil {
		t.Fatal("no modal opened after s")
	}
	if s.modal.action != "stop" {
		t.Fatalf("modal action = %q, want stop", s.modal.action)
	}
	if s.modal.targetID != "a1b2c3d4e5" {
		t.Fatalf("modal targetID = %q, want a1b2c3d4e5", s.modal.targetID)
	}
}

func TestRemoveKeyOpensConfirmModal(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('x'), flatte.Effects[State]{})
	if s.modal == nil || s.modal.action != "remove" {
		t.Fatalf("after x, modal = %+v, want action=remove", s.modal)
	}
}

func TestModalCapturesInputUntilClosed(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('s'), flatte.Effects[State]{})
	if s.modal == nil {
		t.Fatal("modal not opened")
	}

	// j during modal should NOT move the list (modal captures it)
	listCursorBefore := s.containers.list.Cursor()
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.list.Cursor() != listCursorBefore {
		t.Fatalf("j during modal moved list: %d -> %d", listCursorBefore, s.containers.list.Cursor())
	}
}

func TestModalConfirmAppliesAction(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('s'), flatte.Effects[State]{})
	Handle(s, keyChar('y'), flatte.Effects[State]{})

	if s.modal != nil {
		t.Fatal("modal not closed after y")
	}
	if s.containers.containers[0].Status != "exited" {
		t.Fatalf("after confirm stop, status = %q, want exited", s.containers.containers[0].Status)
	}
}

func TestModalCancelDoesNotApplyAction(t *testing.T) {
	s := resizedState(80, 24)
	originalStatus := s.containers.containers[0].Status
	Handle(s, keyChar('s'), flatte.Effects[State]{})
	Handle(s, keyChar('n'), flatte.Effects[State]{})

	if s.modal != nil {
		t.Fatal("modal not closed after n")
	}
	if s.containers.containers[0].Status != originalStatus {
		t.Fatalf("after cancel, status changed: %q -> %q", originalStatus, s.containers.containers[0].Status)
	}
}

func TestModalEscapeCloses(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('s'), flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEscape}, flatte.Effects[State]{})
	if s.modal != nil {
		t.Fatal("modal not closed after Esc")
	}
}

func TestModalRendersAsOverlayOverBase(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('s'), flatte.Effects[State]{})

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "stop container") {
		t.Fatalf("modal title missing in:\n%s", content)
	}
	if !strings.Contains(content, "stop nginx-proxy?") {
		t.Fatalf("modal body missing target name in:\n%s", content)
	}
	// Base content still present underneath
	if !strings.Contains(content, "[1 containers]") {
		t.Fatalf("base tab bar missing under modal in:\n%s", content)
	}
}

func TestMouseClickListRowSelectsIt(t *testing.T) {
	s := resizedState(80, 24)
	// Production flow: View populates zones (via Scan) before any mouse event
	// is dispatched. Tests must mirror that.
	View(s, flatte.RenderContext{Width: 80})
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      2, Y: 5,
	}, flatte.Effects[State]{})

	if sel := s.containers.selected(); sel == nil || sel.Name != "api-server" {
		got := "nil"
		if sel != nil {
			got = sel.Name
		}
		t.Fatalf("after click on row 1, selected = %s, want api-server", got)
	}
}

func TestMouseClickTabHeaderSwitchesTab(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	// tab:logs is at X=26-31 per the scanner; click X=28
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      28, Y: 3,
	}, flatte.Effects[State]{})

	if s.containers.tab != tabLogs {
		t.Fatalf("after click on logs tab header, tab = %v, want logs", s.containers.tab)
	}
}

func TestMouseClickOutsideZonesIsNoOp(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	originalTab := s.containers.tab
	originalSelected := s.containers.selected()

	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      27, Y: 10,
	}, flatte.Effects[State]{})

	if s.containers.tab != originalTab {
		t.Fatalf("click in gap changed tab")
	}
	newSelected := s.containers.selected()
	if (originalSelected == nil) != (newSelected == nil) ||
		(originalSelected != nil && newSelected != nil && originalSelected.ID != newSelected.ID) {
		t.Fatalf("click in gap changed selection")
	}
}

func TestMouseMotionDoesNotTrigger(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	originalTab := s.containers.tab

	Handle(s, flatte.MouseEvent{
		Action: flatte.MouseMotion,
		Button: flatte.MouseLeft,
		X:      28, Y: 3,
	}, flatte.Effects[State]{})

	if s.containers.tab != originalTab {
		t.Fatalf("mouse motion triggered tab change")
	}
}

func TestMouseZonesRecomputeOnResize(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      2, Y: 4,
	}, flatte.Effects[State]{})
	if s.containers.list.Cursor() != 0 {
		t.Fatalf("click Y=4 should select row 0, got cursor %d", s.containers.list.Cursor())
	}

	Handle(s, flatte.ResizeEvent{Width: 80, Height: 30}, flatte.Effects[State]{})
	View(s, flatte.RenderContext{Width: 80})
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      2, Y: 4,
	}, flatte.Effects[State]{})
	if s.containers.list.Cursor() != 0 {
		t.Fatalf("after resize, click Y=4 should still select row 0, got cursor %d", s.containers.list.Cursor())
	}
}

func TestImagesScreenListMovesWithJK(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenImages
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.images.list.Cursor() != 1 {
		t.Fatalf("images cursor after j = %d, want 1", s.images.list.Cursor())
	}
	Handle(s, keyChar('k'), flatte.Effects[State]{})
	if s.images.list.Cursor() != 0 {
		t.Fatalf("images cursor after k = %d, want 0", s.images.list.Cursor())
	}
}

func TestImagesScreenDetailShowsSelectedImage(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenImages
	Handle(s, keyChar('j'), flatte.Effects[State]{})

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "myapp/api:2.1") || !strings.Contains(content, "sha256:b2c3d4e5") {
		t.Fatalf("images detail missing selected image in:\n%s", content)
	}
}

func TestImagesScreenTabCyclesFocus(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenImages
	if !s.images.focus.Focused(imgFocusList) {
		t.Fatal("images focus should start on list")
	}
	Handle(s, keyTab(false), flatte.Effects[State]{})
	if !s.images.focus.Focused(imgFocusDetail) {
		t.Fatal("images focus should be on detail after Tab")
	}
	Handle(s, keyTab(false), flatte.Effects[State]{})
	if !s.images.focus.Focused(imgFocusList) {
		t.Fatal("images focus should wrap to list after second Tab")
	}
}

func TestVolumesScreenListMovesWithJK(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenVolumes
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.volumes.list.Cursor() != 2 {
		t.Fatalf("volumes cursor after 2j = %d, want 2", s.volumes.list.Cursor())
	}
}

func TestVolumesScreenDetailShowsSelectedVolume(t *testing.T) {
	s := resizedState(80, 24)
	s.screen = screenVolumes
	Handle(s, keyChar('j'), flatte.Effects[State]{})

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "config") || !strings.Contains(content, "/var/lib/docker/volumes/config/_data") {
		t.Fatalf("volumes detail missing selected volume in:\n%s", content)
	}
}

func TestScreensAreIsolatedFromEachOther(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	containersCursor := s.containers.list.Cursor()
	Handle(s, keyChar('2'), flatte.Effects[State]{})
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	Handle(s, keyChar('1'), flatte.Effects[State]{})

	if s.containers.list.Cursor() != containersCursor {
		t.Fatalf("containers cursor drifted while on images: %d -> %d", containersCursor, s.containers.list.Cursor())
	}
	if s.images.list.Cursor() != 2 {
		t.Fatalf("images cursor not preserved: %d", s.images.list.Cursor())
	}
}

func TestStatusLineUpdatesAfterStatsTick(t *testing.T) {
	s := resizedState(80, 24)
	before := s.containers.statusLine
	s.containers.tickStats(time.Now())
	after := s.containers.statusLine
	if before == after {
		t.Fatalf("status line did not change after tick\nbefore: %q\nafter:  %q", before, after)
	}
	if !strings.Contains(after, "running") {
		t.Fatalf("status line missing running count: %q", after)
	}
}

func TestActivityPaneReceivesEventsFromStatsTick(t *testing.T) {
	s := resizedState(80, 24)
	if len(s.containers.activity) != 0 {
		t.Fatalf("activity not empty at start: %d events", len(s.containers.activity))
	}
	s.containers.tickStats(time.Now())
	if len(s.containers.activity) != 1 {
		t.Fatalf("after one tick, activity = %d events, want 1", len(s.containers.activity))
	}
	if !strings.Contains(s.containers.activity[0], "nginx-proxy") {
		t.Fatalf("activity event missing container name: %q", s.containers.activity[0])
	}
}

func TestActivityPaneReceivesEventsFromLiveLogAppend(t *testing.T) {
	s := resizedState(80, 24)
	before := len(s.containers.activity)
	s.containers.appendLiveLog("a1b2c3d4e5", "test log line for activity")
	if len(s.containers.activity) != before+1 {
		t.Fatalf("activity not appended after live log: %d -> %d", before, len(s.containers.activity))
	}
	last := s.containers.activity[len(s.containers.activity)-1]
	if !strings.Contains(last, "test log line") {
		t.Fatalf("activity event content missing: %q", last)
	}
}

func TestSparklineRendersBlockCharsAfterHistory(t *testing.T) {
	s := resizedState(80, 24)
	// Drive several ticks so cpuHistory fills
	for i := 0; i < 5; i++ {
		s.containers.tickStats(time.Now())
	}
	hist := s.containers.cpuHistory["a1b2c3d4e5"]
	if len(hist) != 5 {
		t.Fatalf("cpuHistory len = %d, want 5", len(hist))
	}
	rendered := sparkline(hist, lipgloss.Color("117"))
	if strings.Contains(rendered, "(no history)") {
		t.Fatalf("sparkline still shows placeholder after ticks: %q", rendered)
	}
	// Should contain at least one of the block characters
	if !strings.ContainsAny(rendered, "▁▂▃▄▅▆▇█") {
		t.Fatalf("sparkline missing block chars: %q", rendered)
	}
}

func TestSparklineEmptyHistoryShowsPlaceholder(t *testing.T) {
	rendered := sparkline(nil, lipgloss.Color("117"))
	if !strings.Contains(rendered, "(no history)") {
		t.Fatalf("empty sparkline should show placeholder, got %q", rendered)
	}
}

func TestCPUHistoryCapsAtThirtyEntries(t *testing.T) {
	s := resizedState(80, 24)
	for i := 0; i < 50; i++ {
		s.containers.tickStats(time.Now())
	}
	hist := s.containers.cpuHistory["a1b2c3d4e5"]
	if len(hist) != 30 {
		t.Fatalf("cpuHistory len = %d, want 30 (capped)", len(hist))
	}
}

func TestAnchorRightActivityPanePresentInContainersView(t *testing.T) {
	s := resizedState(80, 24)
	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "activity") {
		t.Fatalf("anchor-right activity pane missing in:\n%s", content)
	}
}

func TestStatusLinePresentAtBottomOfContainersBody(t *testing.T) {
	s := resizedState(80, 24)
	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "containers (") {
		t.Fatalf("status line missing aggregate count in:\n%s", content)
	}
}

func keyChar(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}
}

func keyTab(shift bool) flatte.KeyEvent {
	var mod flatte.Mod
	if shift {
		mod = flatte.ModShift
	}
	return flatte.KeyEvent{Key: flatte.KeyTab, Mod: mod}
}
