package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatest"
)

func typeRunes(s *State, text string) {
	for _, r := range text {
		Handle(s, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: r}, flatcore.Effects[State]{})
	}
}

func TestEnterSendsAndClearsInput(t *testing.T) {
	s := NewState()
	typeRunes(s, "hi")
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})
	if s.input.Value != "" {
		t.Fatalf("input after send = %q, want cleared", s.input.Value)
	}
	if s.sent != 1 {
		t.Fatalf("sent = %d, want 1", s.sent)
	}
}

func TestEnterOnEmptyInputDoesNothing(t *testing.T) {
	s := NewState()
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})
	if s.sent != 0 {
		t.Fatalf("sent = %d, want 0 (empty enter is a no-op)", s.sent)
	}
}

func TestModifiedArrowsMoveInputByWord(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world")

	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyLeft, Mod: flatcore.ModAlt}, flatcore.Effects[State]{})
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-left cursor = %d, want start of world", s.input.Cursor)
	}
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyLeft, Mod: flatcore.ModCtrl}, flatcore.Effects[State]{})
	if s.input.Cursor != 0 {
		t.Fatalf("ctrl-left cursor = %d, want start", s.input.Cursor)
	}
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyRight, Mod: flatcore.ModCtrl}, flatcore.Effects[State]{})
	if s.input.Cursor != len("hello") {
		t.Fatalf("ctrl-right cursor = %d, want end of hello", s.input.Cursor)
	}
}

func TestAltBFMoveInputByWord(t *testing.T) {
	s := NewState()
	typeRunes(s, "hello world")

	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'b', Mod: flatcore.ModAlt}, flatcore.Effects[State]{})
	if s.input.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", s.input.Cursor)
	}
	if s.input.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", s.input.Value)
	}
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'f', Mod: flatcore.ModAlt}, flatcore.Effects[State]{})
	if s.input.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", s.input.Cursor)
	}
}

func TestEscQuits(t *testing.T) {
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(NewState(), flatcore.KeyEvent{Key: flatcore.KeyEscape}, fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestViewPlacesCursorAfterPrompt(t *testing.T) {
	s := NewState()
	typeRunes(s, "ab")
	frame := View(s, flatcore.RenderContext{Width: 50})
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
	flatest.AssertGoldenFrame(t, "testdata/chat.golden", View(s, flatcore.RenderContext{Width: 50}))
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
		done <- flatcore.Run(context.Background(), flatcore.App[State]{
			State:  NewState(),
			Handle: Handle,
			View:   View,
		}, flatcore.WithInput(reader), flatcore.WithOutput(&out), flatcore.WithInline())
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
