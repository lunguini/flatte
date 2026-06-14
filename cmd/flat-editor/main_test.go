package main

import (
	"context"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatest"
)

func emptyState() *State {
	s := &State{}
	s.ta.SetValue("")
	s.layout(80, 24)
	return s
}

func typeRunes(s *State, text string) {
	for _, r := range text {
		Handle(s, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: r}, flatcore.Effects[State]{})
	}
}

func TestEditingInsertsAcrossNewlines(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hi")
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})
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
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyLeft}, flatcore.Effects[State]{})
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyBackspace}, flatcore.Effects[State]{}) // remove 'b'
	if s.ta.Value() != "ac" {
		t.Fatalf("Value() = %q, want ac", s.ta.Value())
	}
}

func TestEscQuits(t *testing.T) {
	s := emptyState()
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyEscape}, fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestViewPlacesCursorAtOrigin(t *testing.T) {
	s := NewState()
	s.layout(72, 24)
	frame := View(s, flatcore.RenderContext{Width: 72})
	if frame.Cursor == nil {
		t.Fatal("editor view has no cursor")
	}
	// card origin (3,1) + 3 pinned lines + cell (0,0) = (3,4)
	if frame.Cursor.X != 3 || frame.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (3,4)", *frame.Cursor)
	}
}

func TestInitialSnapshot(t *testing.T) {
	s := NewState()
	s.layout(72, 24)
	flatest.AssertGoldenFrame(t, "testdata/editor.golden", View(s, flatcore.RenderContext{Width: 72}))
}
