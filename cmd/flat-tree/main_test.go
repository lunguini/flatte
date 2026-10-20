package main

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestTabMovesFocusBetweenSections(t *testing.T) {
	s := NewState()
	if !s.focus.Focused(int(focusTree)) {
		t.Fatalf("initial focus index = %d, want tree", s.focus.Index())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab}, flatte.Effects[State]{})
	if !s.focus.Focused(int(focusSearch)) {
		t.Fatalf("after tab focus index = %d, want search", s.focus.Index())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if !s.focus.Focused(int(focusTree)) {
		t.Fatalf("after shift-tab focus index = %d, want tree", s.focus.Index())
	}
}

func TestTreeToggleChangesVisibleRows(t *testing.T) {
	s := NewState()
	before := len(s.tree.VisibleRows())
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	after := len(s.tree.VisibleRows())
	if after >= before {
		t.Fatalf("visible rows after collapsing root = %d, want less than %d", after, before)
	}
	if got := s.tree.CursorID(); got != "workspace" {
		t.Fatalf("CursorID() = %q, want workspace", got)
	}
}

// TestTreeLeftRightCollapseAndExpand: right expands a closed node or steps
// into an open one; left collapses an open node or steps out to its parent.
// h/l do the same.
func TestTreeLeftRightCollapseAndExpand(t *testing.T) {
	for _, keys := range []struct {
		name        string
		left, right flatte.KeyEvent
	}{
		{"arrows", flatte.KeyEvent{Key: flatte.KeyLeft}, flatte.KeyEvent{Key: flatte.KeyRight}},
		{"h/l", flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h'}, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'l'}},
	} {
		t.Run(keys.name, func(t *testing.T) {
			s := NewState() // workspace open; cursor on it
			press := func(k flatte.KeyEvent, wantCursor string, wantOpen ...string) {
				t.Helper()
				Handle(s, k, flatte.Effects[State]{})
				if got := s.tree.CursorID(); got != wantCursor {
					t.Fatalf("cursor = %q, want %q", got, wantCursor)
				}
				open := map[string]bool{}
				for _, r := range s.tree.VisibleRows() {
					if r.Expanded {
						open[r.ID] = true
					}
				}
				if len(open) != len(wantOpen) {
					t.Fatalf("open nodes = %v, want %v", open, wantOpen)
				}
				for _, id := range wantOpen {
					if !open[id] {
						t.Fatalf("open nodes = %v, want %v", open, wantOpen)
					}
				}
			}
			press(keys.right, "flatui", "workspace")           // open: step into first child
			press(keys.right, "flatui", "workspace", "flatui") // closed: expand
			press(keys.right, "tree", "workspace", "flatui")   // open: step in
			press(keys.right, "tree", "workspace", "flatui")   // leaf: nothing
			press(keys.left, "flatui", "workspace", "flatui")  // leaf: step out to parent
			press(keys.left, "flatui", "workspace")            // open: collapse
			press(keys.left, "workspace", "workspace")         // closed: step out
			press(keys.left, "workspace")                      // open root: collapse
			press(keys.left, "workspace")                      // closed root: nothing
		})
	}
}

// TestSearchFiltersTheTree: a query shows only matching nodes (with their
// subtrees) and the ancestors that lead to them, expanded, with the cursor
// on the first match; clearing it restores the tree as it was.
func TestSearchFiltersTheTree(t *testing.T) {
	s := NewState()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{}) // cursor on flatui
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab}, flatte.Effects[State]{})  // focus search
	typeText := func(text string) {
		for _, r := range text {
			Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
		}
	}
	labels := func() string {
		var out []string
		for _, r := range s.activeTree().VisibleRows() {
			out = append(out, r.Label)
		}
		return strings.Join(out, " ")
	}
	typeText("VIEW") // case-insensitive
	if got := labels(); got != "workspace flatui viewport.go" {
		t.Fatalf("rows for VIEW = %q", got)
	}
	if got := s.activeTree().CursorID(); got != "viewport" {
		t.Fatalf("cursor = %q, want the first match", got)
	}
	for range 4 {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})
	}
	typeText("cmd") // a matching folder keeps its whole subtree
	if got := labels(); got != "workspace cmd flat-tree flat-style" {
		t.Fatalf("rows for cmd = %q", got)
	}
	typeText("zzz")
	if got := labels(); got != "" {
		t.Fatalf("rows for cmdzzz = %q, want none", got)
	}
	if frame := composedAt(View(s, flatte.RenderContext{Width: 72}), 72, 18).Content; !strings.Contains(frame, "no matches") {
		t.Fatalf("empty results not shown:\n%s", frame)
	}
	for range 6 {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})
	}
	if got := labels(); got != "workspace flatui cmd .docs" || s.activeTree().CursorID() != "flatui" {
		t.Fatalf("cleared search shows %q with cursor %q, want the tree as before", got, s.activeTree().CursorID())
	}
}

// TestResultsAreNavigable: with a query, the tree keys move and fold within
// the results.
func TestResultsAreNavigable(t *testing.T) {
	s := NewState()
	settle(s, 72, 18) // size the details pane as the runtime would
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab}, flatte.Effects[State]{})
	for _, r := range "go" {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, flatte.Effects[State]{}) // back to the tree
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	if got := s.activeTree().CursorID(); got != "viewport" {
		t.Fatalf("j in results moved to %q, want viewport", got)
	}
	if !strings.Contains(s.details.View(), "Selected: viewport.go") {
		t.Fatalf("details do not follow the results cursor:\n%s", s.details.View())
	}
}

func TestSearchInputOnlyWhenFocused(t *testing.T) {
	s := NewState()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'x'}, flatte.Effects[State]{})
	if s.search.Value != "" {
		t.Fatalf("tree-focused character edited search: %q", s.search.Value)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyTab}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h'}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'i'}, flatte.Effects[State]{})
	if s.search.Value != "hi" {
		t.Fatalf("search Value = %q, want hi", s.search.Value)
	}
	if !strings.Contains(s.details.View(), "Search: hi") {
		t.Fatalf("details did not reflect search:\n%s", s.details.View())
	}
}

func TestTreeViewMatchesSnapshot(t *testing.T) {
	s := NewState()
	settle(s, 72, 18)
	flatest.AssertGoldenFrame(t, "testdata/tree.golden", composedAt(View(s, flatte.RenderContext{Width: 72}), 72, 18))
}

// settle composes the view at width×height and delivers the solved rects as
// the runtime does before painting.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

// composedAt is View's frame painted on a width×height terminal.
func composedAt(f flatte.Frame, width, height int) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, width, height)
	return f
}
