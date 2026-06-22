package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func typeRunes(s *State, text string) {
	for _, r := range text {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
	}
}

func TestEnterSendsAndClearsInput(t *testing.T) {
	s := NewState()
	typeRunes(s, "hi")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	if s.input.Value != "" {
		t.Fatalf("input after send = %q, want cleared", s.input.Value)
	}
	if s.sent != 1 {
		t.Fatalf("sent = %d, want 1", s.sent)
	}
}

func TestEnterOnEmptyInputDoesNothing(t *testing.T) {
	s := NewState()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	if s.sent != 0 {
		t.Fatalf("sent = %d, want 0 (empty enter is a no-op)", s.sent)
	}
}

func TestModifiedArrowsMoveInputByWord(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-left cursor = %d, want start of world", s.input.Cursor)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.input.Cursor != 0 {
		t.Fatalf("ctrl-left cursor = %d, want start", s.input.Cursor)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.input.Cursor != len("hello") {
		t.Fatalf("ctrl-right cursor = %d, want end of hello", s.input.Cursor)
	}
}

func TestAltBFMoveInputByWord(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'b', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", s.input.Cursor)
	}
	if s.input.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", s.input.Value)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'f', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", s.input.Cursor)
	}
}

func TestModifiedBackspaceAndDeleteRemoveInputWords(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world café")
	s.input.SetCursor(len("hello world"))

	Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.input.Value != "hello  café" {
		t.Fatalf("ctrl-backspace value = %q, want %q", s.input.Value, "hello  café")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("ctrl-backspace cursor = %d, want %d", s.input.Cursor, len("hello "))
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyDelete, Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Value != "hello " {
		t.Fatalf("alt-delete value = %q, want %q", s.input.Value, "hello ")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-delete cursor = %d, want %d", s.input.Cursor, len("hello "))
	}
}

func TestReadlineWordDeleteAliases(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world café")
	s.input.SetCursor(len("hello world"))

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'w', Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.input.Value != "hello  café" {
		t.Fatalf("ctrl-w value = %q, want %q", s.input.Value, "hello  café")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("ctrl-w cursor = %d, want %d", s.input.Cursor, len("hello "))
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Value != "hello " {
		t.Fatalf("alt-d value = %q, want %q", s.input.Value, "hello ")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-d cursor = %d, want %d", s.input.Cursor, len("hello "))
	}
}

func TestCtrlAltHDWordDeleteAliases(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world café")
	s.input.SetCursor(len("hello world"))

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h', Mod: flatte.ModCtrl | flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Value != "hello  café" {
		t.Fatalf("ctrl-alt-h value = %q, want %q", s.input.Value, "hello  café")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("ctrl-alt-h cursor = %d, want %d", s.input.Cursor, len("hello "))
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd', Mod: flatte.ModCtrl | flatte.ModAlt}, flatte.Effects[State]{})
	if s.input.Value != "hello " {
		t.Fatalf("ctrl-alt-d value = %q, want %q", s.input.Value, "hello ")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("ctrl-alt-d cursor = %d, want %d", s.input.Cursor, len("hello "))
	}
}

func TestCtrlHDeletesInputWordBackward(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world café")
	s.input.SetCursor(len("hello world"))

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h', Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.input.Value != "hello  café" {
		t.Fatalf("ctrl-h value = %q, want %q", s.input.Value, "hello  café")
	}
	if s.input.Cursor != len("hello ") {
		t.Fatalf("ctrl-h cursor = %d, want %d", s.input.Cursor, len("hello "))
	}
}

func TestEscQuits(t *testing.T) {
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(NewState(), flatte.KeyEvent{Key: flatte.KeyEscape}, fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestViewPlacesCursorAfterPrompt(t *testing.T) {
	s := NewState()
	typeRunes(s, "ab")
	frame := View(s, flatte.RenderContext{Width: 50})
	if frame.Cursor == nil {
		t.Fatal("no cursor")
	}
	// card origin x (3) + prompt "› " width (2) + 2 typed cells = 7
	if frame.Cursor.X != 7 || frame.Cursor.Y != 2 {
		t.Fatalf("cursor = %+v, want (7,2)", *frame.Cursor)
	}
}

func TestViewSnapshot(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello")
	flatest.AssertGoldenFrame(t, "testdata/chat.golden", View(s, flatte.RenderContext{Width: 50}))
}

// TestSentMessageReachesScrollback drives the real Run inline over a pipe:
// typing a line and pressing enter must fx.Print it into the output.
func TestSentMessageReachesScrollback(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flatte.Run(context.Background(), flatte.App[State]{
			State:  NewState(),
			Handle: Handle,
			View:   View,
		}, flatte.WithInput(reader), flatte.WithOutput(&out), flatte.WithInline())
	}()

	// type "hi", send (\r), then quit with Ctrl-C (\x03, default quit)
	if _, err := writer.Write([]byte("hi\r\x03")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run")
	}
	if !strings.Contains(out.String(), "you: hi") {
		t.Fatalf("sent message did not reach scrollback:\n%q", out.String())
	}
}
