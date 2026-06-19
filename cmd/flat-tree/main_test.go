package main

import (
	"strings"
	"testing"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestTabMovesFocusBetweenSections(t *testing.T) {
	s := NewState()
	if !s.focus.Focused(int(focusTree)) {
		t.Fatalf("initial focus index = %d, want tree", s.focus.Index())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	if !s.focus.Focused(int(focusSearch)) {
		t.Fatalf("after tab focus index = %d, want search", s.focus.Index())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab, Mod: flat.ModShift}, flat.Effects[State]{})
	if !s.focus.Focused(int(focusTree)) {
		t.Fatalf("after shift-tab focus index = %d, want tree", s.focus.Index())
	}
}

func TestTreeToggleChangesVisibleRows(t *testing.T) {
	s := NewState()
	before := len(s.tree.VisibleRows())
	Handle(s, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})
	after := len(s.tree.VisibleRows())
	if after >= before {
		t.Fatalf("visible rows after collapsing root = %d, want less than %d", after, before)
	}
	if got := s.tree.CursorID(); got != "workspace" {
		t.Fatalf("CursorID() = %q, want workspace", got)
	}
}

func TestSearchInputOnlyWhenFocused(t *testing.T) {
	s := NewState()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'h'}, flat.Effects[State]{})
	if s.search.Value != "" {
		t.Fatalf("tree-focused character edited search: %q", s.search.Value)
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'h'}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'i'}, flat.Effects[State]{})
	if s.search.Value != "hi" {
		t.Fatalf("search Value = %q, want hi", s.search.Value)
	}
	if !strings.Contains(s.details.View(), "Search: hi") {
		t.Fatalf("details did not reflect search:\n%s", s.details.View())
	}
}

func TestTreeViewMatchesSnapshot(t *testing.T) {
	s := NewState()
	flatest.AssertGoldenFrame(t, "testdata/tree.golden", View(s, flat.RenderContext{Width: 72}))
}
