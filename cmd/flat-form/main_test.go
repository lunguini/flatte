package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestHandleEditsFocusedFieldWithoutHiddenWidgetOwnership(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'A'}, flatcore.Effects[State]{})
	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'd'}, flatcore.Effects[State]{})
	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'a'}, flatcore.Effects[State]{})

	if state.fields[0].Input.Value != "Ada" {
		t.Fatalf("name field = %q, want Ada", state.fields[0].Input.Value)
	}
	if state.fields[0].Input.Cursor != 3 {
		t.Fatalf("name cursor = %d, want 3", state.fields[0].Input.Cursor)
	}
}

func TestTabMovesFocusBetweenFields(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.Event{Key: flatcore.KeyTab}, flatcore.Effects[State]{})
	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'o'}, flatcore.Effects[State]{})

	if state.focused != 1 {
		t.Fatalf("focused = %d, want filter field", state.focused)
	}
	if state.fields[1].Input.Value != "o" {
		t.Fatalf("filter field = %q, want o", state.fields[1].Input.Value)
	}
}

func TestCursorMovementBackspaceAndDeleteAreFieldLocal(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "abcd"
	state.fields[0].Input.Cursor = 2

	Handle(state, flatcore.Event{Key: flatcore.KeyLeft}, flatcore.Effects[State]{})
	Handle(state, flatcore.Event{Key: flatcore.KeyBackspace}, flatcore.Effects[State]{})
	Handle(state, flatcore.Event{Key: flatcore.KeyDelete}, flatcore.Effects[State]{})

	if state.fields[0].Input.Value != "cd" {
		t.Fatalf("name field = %q, want cd", state.fields[0].Input.Value)
	}
	if state.fields[0].Input.Cursor != 0 {
		t.Fatalf("name cursor = %d, want 0", state.fields[0].Input.Cursor)
	}
}

func TestEscapeBlursAndQQuitsOnlyWhenBlurred(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)
	if quit {
		t.Fatal("q should edit focused field, not quit")
	}
	if state.fields[0].Input.Value != "q" {
		t.Fatalf("name field = %q, want q", state.fields[0].Input.Value)
	}

	Handle(state, flatcore.Event{Key: flatcore.KeyEscape}, fx)
	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should quit after blur")
	}
}

func TestEnterSubmitsWhenFocused(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "Ada"
	state.fields[1].Input.Value = "op"

	Handle(state, flatcore.Event{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	if state.submitted != "name=Ada filter=op" {
		t.Fatalf("submitted = %q, want form summary", state.submitted)
	}
}

func TestViewRendersFocusedCursorAndSubmittedState(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "Ada"
	state.fields[0].Input.Cursor = 1
	state.fields[1].Input.Value = "op"
	state.submitted = "name=Ada filter=op"

	frame := View(state, flatcore.RenderContext{Width: 72})

	for _, want := range []string{"Flat Form", "A▌da", "filter", "name=Ada filter=op"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesSubmittedSnapshot(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "Ada"
	state.fields[0].Input.Cursor = 1
	state.fields[1].Input.Value = "op"
	state.fields[1].Input.Cursor = 2
	state.submitted = "name=Ada filter=op"

	flatuitest.AssertGolden(t, "testdata/submitted.golden", View(state, flatcore.RenderContext{Width: 72}))
}
