package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestHandleEditsFocusedFieldWithoutHiddenWidgetOwnership(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'A'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'd'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'a'}, flat.Effects[State]{})

	if state.fields[0].Input.Value != "Ada" {
		t.Fatalf("name field = %q, want Ada", state.fields[0].Input.Value)
	}
	if state.fields[0].Input.Cursor != 3 {
		t.Fatalf("name cursor = %d, want 3", state.fields[0].Input.Cursor)
	}
}

func TestTabMovesFocusBetweenFields(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'o'}, flat.Effects[State]{})

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

	Handle(state, flat.KeyEvent{Key: flat.KeyLeft}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyBackspace}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyDelete}, flat.Effects[State]{})

	if state.fields[0].Input.Value != "cd" {
		t.Fatalf("name field = %q, want cd", state.fields[0].Input.Value)
	}
	if state.fields[0].Input.Cursor != 0 {
		t.Fatalf("name cursor = %d, want 0", state.fields[0].Input.Cursor)
	}
}

func TestModifiedArrowsMoveFocusedFieldByWord(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "hello world"
	state.fields[0].Input.Cursor = len("hello world")

	Handle(state, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModAlt}, flat.Effects[State]{})
	if state.fields[0].Input.Cursor != len("hello ") {
		t.Fatalf("alt-left cursor = %d, want start of world", state.fields[0].Input.Cursor)
	}
	Handle(state, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModCtrl}, flat.Effects[State]{})
	if state.fields[0].Input.Cursor != 0 {
		t.Fatalf("ctrl-left cursor = %d, want start", state.fields[0].Input.Cursor)
	}
	Handle(state, flat.KeyEvent{Key: flat.KeyRight, Mod: flat.ModCtrl}, flat.Effects[State]{})
	if state.fields[0].Input.Cursor != len("hello") {
		t.Fatalf("ctrl-right cursor = %d, want end of hello", state.fields[0].Input.Cursor)
	}
}

func TestAltBFMoveFocusedFieldByWord(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "hello world"
	state.fields[0].Input.Cursor = len("hello world")

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'b', Mod: flat.ModAlt}, flat.Effects[State]{})
	if state.fields[0].Input.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", state.fields[0].Input.Cursor)
	}
	if state.fields[0].Input.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", state.fields[0].Input.Value)
	}
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'f', Mod: flat.ModAlt}, flat.Effects[State]{})
	if state.fields[0].Input.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", state.fields[0].Input.Cursor)
	}
}

func TestEscapeBlursAndQQuitsOnlyWhenBlurred(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if quit {
		t.Fatal("q should edit focused field, not quit")
	}
	if state.fields[0].Input.Value != "q" {
		t.Fatalf("name field = %q, want q", state.fields[0].Input.Value)
	}

	Handle(state, flat.KeyEvent{Key: flat.KeyEscape}, fx)
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should quit after blur")
	}
}

func TestEnterSubmitsWhenFocused(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "Ada"
	state.fields[1].Input.Value = "op"

	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

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

	frame := View(state, flat.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Form", "Ada", "filter", "name=Ada filter=op"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "▌") {
		t.Fatalf("View() still paints the fake cursor marker:\n%s", frame)
	}
}

func TestViewPlacesCursorInFocusedField(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Insert('a')
	state.fields[0].Input.Insert('b')

	frame := View(state, flat.RenderContext{Width: 72})
	if frame.Cursor == nil {
		t.Fatal("editing view has no cursor")
	}
	// row: card border(1) + title,subtle,blank(3) + field 0 = 4
	// col: card origin(3) + "> name: "(8) + 2 typed cells = 13
	if frame.Cursor.X != 13 || frame.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (13,4)", *frame.Cursor)
	}

	state.focused = 1
	if second := View(state, flat.RenderContext{Width: 72}); second.Cursor == nil || second.Cursor.Y != 5 {
		t.Fatalf("cursor on second field = %+v, want row 5", second.Cursor)
	}

	state.editing = false
	if blurred := View(state, flat.RenderContext{Width: 72}); blurred.Cursor != nil {
		t.Fatalf("blurred view still has a cursor: %+v", *blurred.Cursor)
	}
}

func TestViewMatchesSubmittedSnapshot(t *testing.T) {
	state := NewState()
	state.fields[0].Input.Value = "Ada"
	state.fields[0].Input.Cursor = 1
	state.fields[1].Input.Value = "op"
	state.fields[1].Input.Cursor = 2
	state.submitted = "name=Ada filter=op"

	flatest.AssertGoldenFrame(t, "testdata/submitted.golden", View(state, flat.RenderContext{Width: 72}))
}
