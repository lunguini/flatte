package main

import (
	"context"
	"fmt"
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

	if !strings.Contains(content, "1 containers") {
		t.Fatalf("containers tab label missing in:\n%s", content)
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
	// Panes touch (no gaps) — widths sum to body width
	if c.listPaneWidth+c.detailPaneWidth+c.activityPaneWidth != 80 {
		t.Fatalf("panes %d + %d + %d != 80 (no gaps; panes touch)", c.listPaneWidth, c.detailPaneWidth, c.activityPaneWidth)
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

func TestLeftRightArrowsAlsoSwitchTabs(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)

	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	if s.containers.tab != tabLogs {
		t.Fatalf("after Right arrow, tab = %v, want logs", s.containers.tab)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	if s.containers.tab != tabInspect {
		t.Fatalf("after second Right, tab = %v, want inspect", s.containers.tab)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft}, flatte.Effects[State]{})
	if s.containers.tab != tabLogs {
		t.Fatalf("after Left from inspect, tab = %v, want logs", s.containers.tab)
	}
}

func TestLeftArrowInFilterMovesCursor(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	for _, r := range "abc" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	if s.containers.filter.Cursor != 3 {
		t.Fatalf("filter cursor = %d, want 3", s.containers.filter.Cursor)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft}, flatte.Effects[State]{})
	if s.containers.filter.Cursor != 2 {
		t.Fatalf("after Left in filter, cursor = %d, want 2", s.containers.filter.Cursor)
	}
}

func TestScrollbarLinesShowsThumbInMiddle(t *testing.T) {
	bar := scrollbarLines(5, 4, 14, 10)
	if !strings.Contains(bar, "█") {
		t.Fatalf("scrollbar missing thumb char: %q", bar)
	}
	if !strings.Contains(bar, "░") {
		t.Fatalf("scrollbar missing track char: %q", bar)
	}
}

func TestScrollbarLinesEmptyWhenContentFits(t *testing.T) {
	bar := scrollbarLines(0, 10, 5, 10)
	if strings.Contains(bar, "█") || strings.Contains(bar, "░") {
		t.Fatalf("scrollbar should be blank when content fits: %q", bar)
	}
}

func TestInspectTabShowsScrollbarWhenContentOverflows(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80}) // populate zones
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabInspect

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "█") {
		t.Fatalf("inspect scrollbar (thumb) not rendered:\n%s", content)
	}
	if !strings.Contains(content, "░") {
		t.Fatalf("inspect scrollbar (track) not rendered:\n%s", content)
	}
}

func TestMouseWheelScrollsListWhenOverListPane(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	startCursor := s.containers.list.Cursor()

	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseWheelDown,
		X:      2, Y: 6,
	}, flatte.Effects[State]{})

	if s.containers.list.Cursor() != startCursor+1 {
		t.Fatalf("wheel down over list: cursor %d -> %d, want %d",
			startCursor, s.containers.list.Cursor(), startCursor+1)
	}
}

func TestMouseWheelScrollsDetailViewportWhenOverDetailPane(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabInspect
	startOffset := s.containers.inspect.Offset()

	detailX := s.containers.listPaneWidth + 4
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseWheelDown,
		X:      detailX, Y: 6,
	}, flatte.Effects[State]{})

	if s.containers.inspect.Offset() == startOffset && s.containers.inspect.TotalLines() > s.containers.inspect.VisibleLines() {
		t.Fatalf("wheel down over detail inspect did not scroll (offset stayed %d)", s.containers.inspect.Offset())
	}
}

func TestFKeyPagesDownInLogsTab(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	startOffset := s.containers.logs.Offset()

	Handle(s, keyChar('f'), flatte.Effects[State]{})

	if s.containers.logs.Offset() == startOffset && s.containers.logs.TotalLines() > s.containers.logs.VisibleLines() {
		t.Fatalf("f did not page-down logs (offset stayed %d)", s.containers.logs.Offset())
	}
}

func TestBKeyPagesUpInLogsTab(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	// Push enough live log content to make scrolling meaningful
	for i := 0; i < 30; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("test log line %d for scrolling", i))
	}
	s.containers.logs.GotoBottom()
	startOffset := s.containers.logs.Offset()
	if startOffset == 0 {
		t.Fatalf("setup: logs did not scroll after appending content (total=%d visible=%d)",
			s.containers.logs.TotalLines(), s.containers.logs.VisibleLines())
	}

	Handle(s, keyChar('b'), flatte.Effects[State]{})

	if s.containers.logs.Offset() >= startOffset {
		t.Fatalf("b did not page-up logs (offset %d -> %d)", startOffset, s.containers.logs.Offset())
	}
}

func TestDAndUHalfPageInspect(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabInspect
	s.containers.inspect.GotoBottom()
	startOffset := s.containers.inspect.Offset()

	Handle(s, keyChar('u'), flatte.Effects[State]{})
	if s.containers.inspect.Offset() >= startOffset {
		t.Fatalf("u did not half-page-up inspect (offset %d -> %d)", startOffset, s.containers.inspect.Offset())
	}

	midOffset := s.containers.inspect.Offset()
	Handle(s, keyChar('d'), flatte.Effects[State]{})
	if s.containers.inspect.Offset() <= midOffset {
		t.Fatalf("d did not half-page-down inspect (offset %d -> %d)", midOffset, s.containers.inspect.Offset())
	}
}

func TestPageKeysDoNothingWhenFilterFocused(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	startValue := s.containers.filter.Value

	Handle(s, keyChar('f'), flatte.Effects[State]{})
	Handle(s, keyChar('b'), flatte.Effects[State]{})

	if s.containers.filter.Value != startValue+"fb" {
		t.Fatalf("f/b should edit filter when filter-focused; got %q want %q", s.containers.filter.Value, startValue+"fb")
	}
}

func TestColonOpensCommandBarFromListFocus(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	if s.command == nil {
		t.Fatal("command bar not opened after :")
	}
}

func TestColonDoesNotOpenWhenFilterFocused(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusFilter)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	if s.command != nil {
		t.Fatal("command bar opened from filter focus — should have typed into filter instead")
	}
	if s.containers.filter.Value != ":" {
		t.Fatalf("filter = %q, want :", s.containers.filter.Value)
	}
}

func TestCommandBarCapturesKeysUntilClosed(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar(':'), flatte.Effects[State]{})

	// j during command bar should NOT move the list
	listCursorBefore := s.containers.list.Cursor()
	Handle(s, keyChar('j'), flatte.Effects[State]{})
	if s.containers.list.Cursor() != listCursorBefore {
		t.Fatalf("j leaked through to list during command: %d -> %d", listCursorBefore, s.containers.list.Cursor())
	}
	if s.command.input.Value != "j" {
		t.Fatalf("command input = %q, want j", s.command.input.Value)
	}
}

func TestEscClosesCommandBar(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEscape}, flatte.Effects[State]{})
	if s.command != nil {
		t.Fatal("command bar not closed after Esc")
	}
}

func TestEnterExecutesFilterCommand(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	for _, r := range "filter nginx" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	if s.command != nil {
		t.Fatal("command bar should auto-close after Enter (lazydocker convention)")
	}
	if s.containers.filter.Value != "nginx" {
		t.Fatalf("filter = %q, want nginx", s.containers.filter.Value)
	}
	if len(s.containers.filtered) != 2 {
		t.Fatalf("filtered len = %d, want 2 (nginx-proxy, nginx-web)", len(s.containers.filtered))
	}
}

func TestEnterGotoCommandClosesBar(t *testing.T) {
	s := resizedState(80, 24)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	for _, r := range "goto images" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	if s.command != nil {
		t.Fatal("command bar should close after :goto")
	}
	if s.screen != screenImages {
		t.Fatalf("screen = %v, want images", s.screen)
	}
}

func TestCommandHistoryNavigation(t *testing.T) {
	s := resizedState(80, 24)
	// Execute three commands
	for _, cmd := range []string{"filter a", "filter b", "filter c"} {
		Handle(s, keyChar(':'), flatte.Effects[State]{})
		for _, r := range cmd {
			Handle(s, keyChar(r), flatte.Effects[State]{})
		}
		Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	}

	// Open fresh, press Up — should recall "filter c"
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyUp}, flatte.Effects[State]{})
	if s.command.input.Value != "filter c" {
		t.Fatalf("after one Up, input = %q, want 'filter c'", s.command.input.Value)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyUp}, flatte.Effects[State]{})
	if s.command.input.Value != "filter b" {
		t.Fatalf("after two Up, input = %q, want 'filter b'", s.command.input.Value)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})
	if s.command.input.Value != "filter c" {
		t.Fatalf("after Up Up Down, input = %q, want 'filter c'", s.command.input.Value)
	}
}

func TestCommandOutputRoutesToActivityFeed(t *testing.T) {
	s := resizedState(80, 24)
	before := len(s.containers.activity)
	Handle(s, keyChar(':'), flatte.Effects[State]{})
	for _, r := range "help" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	after := len(s.containers.activity)
	if after < before+2 {
		t.Fatalf("activity feed should have >=2 new entries (cmd + output); got %d -> %d", before, after)
	}
	last := s.containers.activity[after-1]
	if !strings.Contains(last, "filter") || !strings.Contains(last, "goto") {
		t.Fatalf("help output missing expected commands: %q", last)
	}
}

func TestCommandBarReplacesStatusLineWhenOpen(t *testing.T) {
	s := resizedState(80, 24)
	closedContent := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(closedContent, "avg CPU") {
		t.Fatalf("status line should show when command closed:\n%s", closedContent)
	}

	Handle(s, keyChar(':'), flatte.Effects[State]{})
	openContent := View(s, flatte.RenderContext{Width: 80}).Content
	if strings.Contains(openContent, "avg CPU") {
		t.Fatalf("status line should hide when command open:\n%s", openContent)
	}
	if !strings.Contains(openContent, "type a command") {
		t.Fatalf("command bar placeholder missing:\n%s", openContent)
	}
}

func TestStreamCommandInjectsWrappedStyledContent(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	before := s.containers.logs.TotalLines()

	Handle(s, keyChar(':'), flatte.Effects[State]{})
	for _, r := range "stream" {
		Handle(s, keyChar(r), flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	after := s.containers.logs.TotalLines()
	if after <= before {
		t.Fatalf("logs did not grow after :stream: %d -> %d", before, after)
	}

	content := View(s, flatte.RenderContext{Width: 80}).Content
	if !strings.Contains(content, "ERROR") {
		t.Fatalf("streamed content missing ERROR level (color exercise):\n%s", content)
	}
	// Hardwrap is character-level, so words split across visual rows.
	// Assert on short tokens that fit in any reasonable column width.
	if !strings.Contains(content, "streaming") || !strings.Contains(content, "complete") {
		t.Fatalf("streamed content missing body tokens (note: long lines hard-wrap char-level):\n%s", content)
	}
}

func TestFollowTailDefaultsTrue(t *testing.T) {
	s := resizedState(80, 24)
	if !s.containers.followTail {
		t.Fatal("followTail should default to true")
	}
}

func TestScrollingUpAwayFromBottomPausesFollow(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	// Push enough content to make scrolling meaningful
	for i := 0; i < 30; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("line %d for follow-tail test", i))
	}
	if !s.containers.followTail {
		t.Fatal("followTail should still be true after auto-tail on append")
	}

	// Scroll up — should pause following
	Handle(s, keyChar('k'), flatte.Effects[State]{})
	if s.containers.followTail {
		t.Fatal("followTail should be false after scrolling up")
	}
}

func TestGKeyResumesFollowingAndJumpsToBottom(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	for i := 0; i < 30; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("line %d", i))
	}
	// Force pause
	s.containers.logs.LineUp(5)
	s.containers.syncFollowTail()
	if s.containers.followTail {
		t.Fatal("expected followTail=false after LineUp")
	}
	offsetBefore := s.containers.logs.Offset()

	Handle(s, keyChar('G'), flatte.Effects[State]{})

	if !s.containers.followTail {
		t.Fatal("followTail should be true after G")
	}
	if s.containers.logs.Offset() <= offsetBefore {
		t.Fatalf("G should jump to bottom; offset %d -> %d", offsetBefore, s.containers.logs.Offset())
	}
}

func TestAppendLiveLogRespectsPausedFollow(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	for i := 0; i < 20; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("initial line %d", i))
	}
	// Pause and capture offset
	s.containers.logs.LineUp(3)
	s.containers.syncFollowTail()
	pausedOffset := s.containers.logs.Offset()
	if s.containers.followTail {
		t.Fatal("expected paused state")
	}

	// Append more — should NOT auto-scroll because paused
	s.containers.appendLiveLog("a1b2c3d4e5", "post-pause line")
	if s.containers.logs.Offset() != pausedOffset {
		t.Fatalf("paused append moved offset: %d -> %d", pausedOffset, s.containers.logs.Offset())
	}
}

func TestAppendLiveLogAutoScrollsWhenFollowing(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	for i := 0; i < 20; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("initial line %d", i))
	}
	if !s.containers.logs.AtBottom() {
		t.Fatal("expected at bottom after auto-follow appends")
	}
}

func TestFollowIndicatorShowsWhenContentOverflows(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs
	for i := 0; i < 30; i++ {
		s.containers.appendLiveLog("a1b2c3d4e5", fmt.Sprintf("overflow line %d", i))
	}

	content := View(s, flatte.RenderContext{Width: 80}).Content
	// Indicator is now rendered inside the tab bar row (right of "inspect"),
	// not as an extra body line. Look for either "↓ tail" or "↑ paused".
	if !strings.Contains(content, "tail") && !strings.Contains(content, "paused") {
		t.Fatalf("follow indicator missing after overflow:\n%s", content)
	}
}

func TestLogsUseWrappedContentSoLongLinesWrap(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs

	// Inject a clearly-too-long line
	s.containers.appendLiveLog("a1b2c3d4e5",
		styledLogLine("99:99:99", logInfo,
			"this is a deliberately very long log line that absolutely cannot fit in the narrow detail pane and must wrap across multiple visual rows"))

	totalWrapped := s.containers.logs.TotalLines()
	if totalWrapped < 4 {
		t.Fatalf("expected long line to wrap to >=4 visual rows; got %d", totalWrapped)
	}
}

func TestPickGlyphSetDefaultsToPowerline(t *testing.T) {
	t.Setenv("FLAT_DOCKER_GLYPHS", "")
	g := pickGlyphSet()
	if g != glyphPower {
		t.Fatalf("default glyph set = %+v, want glyphPower", g)
	}
}

func TestPickGlyphSetRespectsEnvVar(t *testing.T) {
	cases := []struct{ env, name string }{
		{"safe", "safe"},
		{"ascii", "ascii"},
		{"off", "off"},
		{"0", "0"},
		{"false", "false"},
		{"SAFE", "SAFE (capital)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("FLAT_DOCKER_GLYPHS", c.env)
			g := pickGlyphSet()
			if g != glyphSafe {
				t.Fatalf("FLAT_DOCKER_GLYPHS=%q -> %+v, want glyphSafe", c.env, g)
			}
		})
	}
}

func TestPickGlyphSetPowerlineWhenExplicitlySet(t *testing.T) {
	t.Setenv("FLAT_DOCKER_GLYPHS", "powerline")
	g := pickGlyphSet()
	if g != glyphPower {
		t.Fatalf("FLAT_DOCKER_GLYPHS=powerline -> %+v, want glyphPower", g)
	}
}

func TestTipForGlyphSetMentionsFallbackForPowerline(t *testing.T) {
	tip := tipForGlyphSet(glyphPower)
	if !strings.Contains(tip, "FLAT_DOCKER_GLYPHS=safe") {
		t.Fatalf("powerline tip should mention the safe fallback: %q", tip)
	}
	if !strings.Contains(tip, "Nerd Font") {
		t.Fatalf("powerline tip should mention Nerd Font: %q", tip)
	}
}

func TestTipForGlyphSetMentionsUpgradeForSafe(t *testing.T) {
	tip := tipForGlyphSet(glyphSafe)
	if !strings.Contains(tip, "FLAT_DOCKER_GLYPHS=powerline") {
		t.Fatalf("safe tip should mention the powerline upgrade: %q", tip)
	}
}

func TestPowerlineTabWidthMatchesContent(t *testing.T) {
	rendered, width := powerlineTab("stats", true)
	// Width is auto-derived from the rendered string. Whatever lipgloss
	// reports, the rendered string's visible width should match.
	if width != lipgloss.Width(rendered) {
		t.Fatalf("returned width %d != lipgloss.Width(rendered) %d", width, lipgloss.Width(rendered))
	}
	// When using safe glyphs, the brackets add 2 chars vs the inner label width.
	// When using powerline, the slants add 2 chars. Either way, the result is
	// len(label) + 6 (2 glyphs + 2*2 padding). Sanity check.
	if width < lipgloss.Width("stats")+2 {
		t.Fatalf("width %d looks too small for label 'stats'", width)
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
	// Long lines wrap inside the narrow detail pane, so check for parts
	// rather than the contiguous substring.
	if !strings.Contains(content, "starting") || !strings.Contains(content, "nginx-proxy") {
		t.Fatalf("logs tab missing seed content in:\n%s", content)
	}
	if !strings.Contains(content, "INFO") {
		t.Fatalf("logs tab missing INFO level marker (color styling):\n%s", content)
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

func TestTabBarShowsActiveTabStyled(t *testing.T) {
	s := resizedState(80, 24)
	s.containers.focus.Select(focusDetail)
	s.containers.tab = tabLogs

	content := View(s, flatte.RenderContext{Width: 80}).Content
	// After ANSI strip, both "stats" and "logs" should appear (no brackets
	// anymore — visual distinction is via background color, which is stripped).
	if !strings.Contains(content, "logs") {
		t.Fatalf("active tab label missing in:\n%s", content)
	}
	if !strings.Contains(content, "stats") || !strings.Contains(content, "inspect") {
		t.Fatalf("inactive tab labels missing in:\n%s", content)
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
	if !strings.Contains(content, "1 containers") {
		t.Fatalf("base tab label missing under modal in:\n%s", content)
	}
}

func TestMouseClickListRowSelectsIt(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80}) // populate zones

	rowRect, ok := s.containers.zones.Rect("list:1")
	if !ok {
		t.Fatal("list:1 zone not registered after View")
	}
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      rowRect.X + 2, Y: rowRect.Y,
	}, flatte.Effects[State]{})

	if sel := s.containers.selected(); sel == nil || sel.Name != "api-server" {
		got := "nil"
		if sel != nil {
			got = sel.Name
		}
		t.Fatalf("after click on list:1 zone, selected = %s, want api-server", got)
	}
}

func TestMouseClickTabHeaderSwitchesTab(t *testing.T) {
	s := resizedState(80, 24)
	View(s, flatte.RenderContext{Width: 80})

	// Tabs use a manual ZoneMap (positions deterministic from label widths),
	// NOT auto-zones — the OSC9 markers confused lipgloss's width calculation
	// inside the width-constrained pane. Verify the manual zones work.
	logsRect, ok := s.containers.tabZones.Rect("tab:logs")
	if !ok {
		t.Fatal("tab:logs manual zone not registered")
	}
	Handle(s, flatte.MouseEvent{
		Action: flatte.MousePress,
		Button: flatte.MouseLeft,
		X:      logsRect.X + 2, Y: logsRect.Y, // middle of the tab
	}, flatte.Effects[State]{})

	if s.containers.tab != tabLogs {
		t.Fatalf("after click on tab:logs zone at %+v, tab = %v, want logs", logsRect, s.containers.tab)
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
