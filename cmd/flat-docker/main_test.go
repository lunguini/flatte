package main

import (
	"context"
	"strings"
	"testing"

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
	wantList := 26
	wantDetail := 80 - wantList - 2
	if c.listPaneWidth != wantList || c.detailPaneWidth != wantDetail {
		t.Fatalf("panes = %d/%d, want %d/%d", c.listPaneWidth, c.detailPaneWidth, wantList, wantDetail)
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
