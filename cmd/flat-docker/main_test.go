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
	if !strings.Contains(content, "containers screen") {
		t.Fatalf("containers body missing in:\n%s", content)
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

func keyChar(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}
}
