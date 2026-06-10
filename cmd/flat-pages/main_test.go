package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestVimKeysMoveCursorOnHome(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'j'}, flatcore.Effects[State]{})
	if state.homeCursor != 1 {
		t.Fatalf("homeCursor = %d, want 1 after j", state.homeCursor)
	}
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'k'}, flatcore.Effects[State]{})
	if state.homeCursor != 0 {
		t.Fatalf("homeCursor = %d, want 0 after k", state.homeCursor)
	}
}

func TestHomeSelectionAndEnterNavigateToDetails(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	if state.screen != screenDetails {
		t.Fatalf("screen = %v, want details", state.screen)
	}
	if state.selected != 1 {
		t.Fatalf("selected = %d, want 1", state.selected)
	}
}

func TestHomeCanNavigateToSettings(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	if state.screen != screenSettings {
		t.Fatalf("screen = %v, want settings", state.screen)
	}
}

func TestEscapeReturnsFromDetailsToHome(t *testing.T) {
	state := NewState()
	state.screen = screenDetails

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEscape}, flatcore.Effects[State]{})

	if state.screen != screenHome {
		t.Fatalf("screen = %v, want home", state.screen)
	}
}

func TestSettingsOwnsTextInputState(t *testing.T) {
	state := NewState()
	state.screen = screenSettings

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'A'}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'd'}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyLeft}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'a'}, flatcore.Effects[State]{})

	if state.settingsName.Value != "Aad" {
		t.Fatalf("settings name = %q, want Aad", state.settingsName.Value)
	}
}

func TestQQuitsOnlyFromHome(t *testing.T) {
	state := NewState()
	state.screen = screenDetails
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)
	if quit {
		t.Fatal("q should not quit from details")
	}

	state.screen = screenHome
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should quit from home")
	}
}

func TestViewDispatchesByScreen(t *testing.T) {
	state := NewState()

	home := View(state, flatcore.RenderContext{Width: 72})
	if !strings.Contains(home, "Flat Pages") || !strings.Contains(home, "Open details") {
		t.Fatalf("home view missing expected content:\n%s", home)
	}

	state.screen = screenSettings
	state.settingsName.Value = "Ada"
	settings := View(state, flatcore.RenderContext{Width: 72})
	if !strings.Contains(settings, "Settings") || !strings.Contains(settings, "Ada") {
		t.Fatalf("settings view missing expected content:\n%s", settings)
	}
}

func TestViewMatchesHomeSnapshot(t *testing.T) {
	state := NewState()
	state.homeCursor = 1

	flatuitest.AssertGolden(t, "testdata/home.golden", View(state, flatcore.RenderContext{Width: 72}))
}

func TestViewMatchesDetailsSnapshot(t *testing.T) {
	state := NewState()
	state.screen = screenDetails
	state.selected = 2

	flatuitest.AssertGolden(t, "testdata/details.golden", View(state, flatcore.RenderContext{Width: 72}))
}

func TestViewMatchesSettingsSnapshot(t *testing.T) {
	state := NewState()
	state.screen = screenSettings
	state.settingsName.Value = "Ada"
	state.settingsName.Cursor = 1

	flatuitest.AssertGolden(t, "testdata/settings.golden", View(state, flatcore.RenderContext{Width: 72}))
}
