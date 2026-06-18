package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestVimKeysMoveCursorOnHome(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'j'}, flat.Effects[State]{})
	if state.homeCursor != 1 {
		t.Fatalf("homeCursor = %d, want 1 after j", state.homeCursor)
	}
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'k'}, flat.Effects[State]{})
	if state.homeCursor != 0 {
		t.Fatalf("homeCursor = %d, want 0 after k", state.homeCursor)
	}
}

func TestHomeSelectionAndEnterNavigateToDetails(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if state.screen != screenDetails {
		t.Fatalf("screen = %v, want details", state.screen)
	}
	if state.selected != 1 {
		t.Fatalf("selected = %d, want 1", state.selected)
	}
}

func TestHomeCanNavigateToSettings(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if state.screen != screenSettings {
		t.Fatalf("screen = %v, want settings", state.screen)
	}
}

func TestEscapeReturnsFromDetailsToHome(t *testing.T) {
	state := NewState()
	state.screen = screenDetails

	Handle(state, flat.KeyEvent{Key: flat.KeyEscape}, flat.Effects[State]{})

	if state.screen != screenHome {
		t.Fatalf("screen = %v, want home", state.screen)
	}
}

func TestSettingsOwnsTextInputState(t *testing.T) {
	state := NewState()
	state.screen = screenSettings

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'A'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'd'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyLeft}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'a'}, flat.Effects[State]{})

	if state.settingsName.Value != "Aad" {
		t.Fatalf("settings name = %q, want Aad", state.settingsName.Value)
	}
}

func TestQQuitsOnlyFromHome(t *testing.T) {
	state := NewState()
	state.screen = screenDetails
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if quit {
		t.Fatal("q should not quit from details")
	}

	state.screen = screenHome
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should quit from home")
	}
}

func TestViewDispatchesByScreen(t *testing.T) {
	state := NewState()

	home := View(state, flat.RenderContext{Width: 72}).Content
	if !strings.Contains(home, "Flat Pages") || !strings.Contains(home, "Open details") {
		t.Fatalf("home view missing expected content:\n%s", home)
	}

	state.screen = screenSettings
	state.settingsName.Value = "Ada"
	settings := View(state, flat.RenderContext{Width: 72}).Content
	if !strings.Contains(settings, "Settings") || !strings.Contains(settings, "Ada") {
		t.Fatalf("settings view missing expected content:\n%s", settings)
	}
}

func TestViewSetsTitlePerScreenAndCursorOnSettings(t *testing.T) {
	state := NewState()

	home := View(state, flat.RenderContext{Width: 72})
	if home.Title != "Flatte \u2014 home" {
		t.Fatalf("home title = %q, want Flatte \u2014 home", home.Title)
	}
	if home.Cursor != nil {
		t.Fatalf("home view has a cursor: %+v", *home.Cursor)
	}

	state.screen = screenSettings
	state.settingsName.Value = "Ada"
	state.settingsName.Cursor = 1
	settings := View(state, flat.RenderContext{Width: 72})
	if settings.Title != "Flatte \u2014 settings" {
		t.Fatalf("settings title = %q, want Flatte \u2014 settings", settings.Title)
	}
	if settings.Cursor == nil {
		t.Fatal("settings view has no cursor")
	}
	// row: card border(1) + title,subtle,blank(3) = 4
	// col: card origin(3) + "  name: "(8) + 1 typed cell = 12
	if settings.Cursor.X != 12 || settings.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (12,4)", *settings.Cursor)
	}
}

func TestViewMatchesHomeSnapshot(t *testing.T) {
	state := NewState()
	state.homeCursor = 1

	flatest.AssertGoldenFrame(t, "testdata/home.golden", View(state, flat.RenderContext{Width: 72}))
}

func TestViewMatchesDetailsSnapshot(t *testing.T) {
	state := NewState()
	state.screen = screenDetails
	state.selected = 2

	flatest.AssertGoldenFrame(t, "testdata/details.golden", View(state, flat.RenderContext{Width: 72}))
}

func TestViewMatchesSettingsSnapshot(t *testing.T) {
	state := NewState()
	state.screen = screenSettings
	state.settingsName.Value = "Ada"
	state.settingsName.Cursor = 1

	flatest.AssertGoldenFrame(t, "testdata/settings.golden", View(state, flat.RenderContext{Width: 72}))
}
