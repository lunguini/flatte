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
