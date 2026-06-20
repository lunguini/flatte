package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func emptyState() *State {
	s := &State{}
	s.ta.SetValue("")
	s.layout(80, 24)
	return s
}

func typeRunes(s *State, text string) {
	for _, r := range text {
		Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: r}, flat.Effects[State]{})
	}
}

func TestEditingInsertsAcrossNewlines(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hi")
	Handle(s, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})
	typeRunes(s, "yo")
	if s.ta.Value() != "hi\nyo" {
		t.Fatalf("Value() = %q, want \"hi\\nyo\"", s.ta.Value())
	}
	if s.ta.Row() != 1 || s.ta.Col() != 2 {
		t.Fatalf("cursor = (%d,%d), want (1,2)", s.ta.Row(), s.ta.Col())
	}
}

func TestArrowsAndBackspaceEdit(t *testing.T) {
	s := emptyState()
	typeRunes(s, "abc")
	Handle(s, flat.KeyEvent{Key: flat.KeyLeft}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyBackspace}, flat.Effects[State]{}) // remove 'b'
	if s.ta.Value() != "ac" {
		t.Fatalf("Value() = %q, want ac", s.ta.Value())
	}
}

func TestModifiedArrowsMoveByWord(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("alt-left cursor = (%d,%d), want start of world", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModCtrl}, flat.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != 0 {
		t.Fatalf("ctrl-left cursor = (%d,%d), want start", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyRight, Mod: flat.ModCtrl}, flat.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("ctrl-right cursor = (%d,%d), want end of hello", s.ta.Row(), s.ta.Col())
	}
}

func TestAltBFMoveByWord(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'b', Mod: flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("alt-b cursor = (%d,%d), want start of world", s.ta.Row(), s.ta.Col())
	}
	if s.ta.Value() != "hello world" {
		t.Fatalf("alt-b inserted text: %q", s.ta.Value())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'f', Mod: flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello world") {
		t.Fatalf("alt-f cursor = (%d,%d), want end", s.ta.Row(), s.ta.Col())
	}
}

func TestModifiedBackspaceAndDeleteRemoveWords(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})

	Handle(s, flat.KeyEvent{Key: flat.KeyBackspace, Mod: flat.ModCtrl}, flat.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-backspace value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-backspace cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flat.KeyEvent{Key: flat.KeyRight}, flat.Effects[State]{})
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyDelete, Mod: flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("alt-delete value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("alt-delete cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestShiftArrowsSelectAndTypingReplacesSelection(t *testing.T) {
	s := emptyState()
	typeRunes(s, "abcd")

	Handle(s, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModShift}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyLeft, Mod: flat.ModShift}, flat.Effects[State]{})
	if got := s.ta.SelectedText(); got != "cd" {
		t.Fatalf("SelectedText() = %q, want %q", got, "cd")
	}

	typeRunes(s, "X")
	if s.ta.Value() != "abX" {
		t.Fatalf("Value after replacing selection = %q, want %q", s.ta.Value(), "abX")
	}
	if _, ok := s.ta.Selection(); ok {
		t.Fatal("selection still active after typing")
	}
}

func TestReadlineWordDeleteAliases(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})

	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'w', Mod: flat.ModCtrl}, flat.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-w value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-w cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flat.KeyEvent{Key: flat.KeyRight}, flat.Effects[State]{})
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'd', Mod: flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("alt-d value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("alt-d cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestCtrlAltHDWordDeleteAliases(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})

	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'h', Mod: flat.ModCtrl | flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-alt-h value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-alt-h cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flat.KeyEvent{Key: flat.KeyRight}, flat.Effects[State]{})
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'd', Mod: flat.ModCtrl | flat.ModAlt}, flat.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("ctrl-alt-d value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("ctrl-alt-d cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestCtrlHDeletesWordBackward(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})

	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'h', Mod: flat.ModCtrl}, flat.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-h value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-h cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}
}

func TestRawCtrlWDeletesWordThroughRun(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := emptyState()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flat.Run(context.Background(), flat.App[State]{
			State:  state,
			Handle: Handle,
			View:   View,
		}, flat.WithInput(reader), flat.WithOutput(&out))
	}()

	_, _ = writer.Write([]byte("hello world\x17\x1b"))
	_ = writer.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit")
	}

	if state.ta.Value() != "hello " {
		t.Fatalf("raw ctrl-w value = %q, want %q", state.ta.Value(), "hello ")
	}
	if state.ta.Row() != 0 || state.ta.Col() != len("hello ") {
		t.Fatalf("raw ctrl-w cursor = (%d,%d), want (0,%d)", state.ta.Row(), state.ta.Col(), len("hello "))
	}
}

func TestEscQuits(t *testing.T) {
	s := emptyState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, flat.KeyEvent{Key: flat.KeyEscape}, fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestViewPlacesCursorAtOrigin(t *testing.T) {
	s := NewState()
	s.layout(72, 24)
	frame := View(s, flat.RenderContext{Width: 72})
	if frame.Cursor == nil {
		t.Fatal("editor view has no cursor")
	}
	// card origin (3,1) + 3 pinned lines + cell (0,0) = (3,4)
	if frame.Cursor.X != 3 || frame.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (3,4)", *frame.Cursor)
	}
}

func TestViewSoftWrapsLongLineAndKeepsCursorInsideTextareaBody(t *testing.T) {
	s := emptyState()
	s.layout(18, 10)
	typeRunes(s, "abcdefghijklmnopqrstuvwxyz")

	if got := s.ta.View(); got != "abcdefghijkl\nmnopqrstuvwx\nyz" {
		t.Fatalf("textarea view = %q, want wrapped rows", got)
	}
	frame := View(s, flat.RenderContext{Width: 18})
	if frame.Cursor == nil {
		t.Fatal("editor view has no cursor")
	}
	if frame.Cursor.X != 5 || frame.Cursor.Y != 6 {
		t.Fatalf("cursor = %+v, want (5,6)", *frame.Cursor)
	}
}

func TestDebugViewShowsLastDecodedKeyAndAction(t *testing.T) {
	s := emptyState()
	s.debugKeys = true
	s.layout(72, 24)
	typeRunes(s, "hello world")
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'w', Mod: flat.ModCtrl}, flat.Effects[State]{})

	frame := View(s, flat.RenderContext{Width: 72})
	if !strings.Contains(frame.Content, "last: character 'w' ctrl -> delete-word-left") {
		t.Fatalf("debug footer missing last key/action:\n%s", frame.Content)
	}
}

func TestInitialSnapshot(t *testing.T) {
	s := NewState()
	s.layout(72, 24)
	flatest.AssertGoldenFrame(t, "testdata/editor.golden", View(s, flat.RenderContext{Width: 72}))
}
