package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui"
)

func TestHandleStartsSearchForTypedCharacters(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatte.StateUpdate[State], 1)
	state := State{focused: true}

	Handle(&state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'o'}, flatte.Effects[State]{
		Context: context.Background(),
		Updates: updates,
	})

	if state.query.Value != "o" {
		t.Fatalf("query = %q, want o", state.query.Value)
	}
	if !state.searching {
		t.Fatal("expected search to be marked in-flight")
	}

	update := receiveSearchUpdate(t, updates)
	update.Apply(&state)
	if state.searching {
		t.Fatal("expected search to finish after update")
	}
	if got := strings.Join(state.results, ","); got != "sonnet,opus,freeform" {
		t.Fatalf("results = %q, want sonnet,opus,freeform", got)
	}
}

func TestTypingJAndKEditsQuery(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "1ms")
	updates := make(chan flatte.StateUpdate[State], 4)
	fx := flatte.NewEffects(t.Context(), updates, nil)
	state := &State{focused: true}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, fx)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'k'}, fx)

	if state.query.Value != "jk" {
		t.Fatalf("query = %q, want %q", state.query.Value, "jk")
	}
}

func TestFocusedSearchCanTypeQ(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatte.StateUpdate[State], 1)
	state := State{focused: true}

	Handle(&state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, flatte.Effects[State]{
		Context: context.Background(),
		Updates: updates,
	})

	if state.query.Value != "q" {
		t.Fatalf("query = %q, want q", state.query.Value)
	}
}

func TestFocusedSearchUsesAltBFForWordMovement(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	state := State{focused: true}
	state.query.Value = "hello world"
	state.query.Cursor = len("hello world")

	Handle(&state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'b', Mod: flatte.ModAlt}, flatte.Effects[State]{
		Context: context.Background(),
	})
	if state.query.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", state.query.Cursor)
	}
	if state.query.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", state.query.Value)
	}
	Handle(&state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'f', Mod: flatte.ModAlt}, flatte.Effects[State]{
		Context: context.Background(),
	})
	if state.query.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", state.query.Cursor)
	}
}

func TestUnfocusedSearchUsesQToQuit(t *testing.T) {
	state := State{focused: false}
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(&state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("expected q to request quit when search input is unfocused")
	}
}

func TestBackspaceStartsSearchForEditedQuery(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatte.StateUpdate[State], 2)
	state := State{focused: true}
	state.query.Value = "op"
	state.query.Cursor = 2

	Handle(&state, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{
		Context: context.Background(),
		Updates: updates,
	})

	if state.query.Value != "o" {
		t.Fatalf("query = %q, want o", state.query.Value)
	}
}

func TestStaleSearchUpdateIsIgnored(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "1ms")
	updates := make(chan flatte.StateUpdate[State], 16)
	fx := flatte.NewEffects(t.Context(), updates, nil)

	// First search: "op" — will be superseded immediately.
	state := State{focused: true}
	state.query.Value = "op"
	state.query.Cursor = 2
	startSearch(&state, fx)

	// Second search: "o" supersedes the first.
	state.query.Value = "o"
	state.query.Cursor = 1
	startSearch(&state, fx)

	// Collect the update that arrives (from the second search).
	update := receiveSearchUpdate(t, updates)
	update.Apply(&state)

	// Drain any stale update from the first search that may arrive late.
	select {
	case stale := <-updates:
		stale.Apply(&state)
	case <-time.After(50 * time.Millisecond):
	}

	// Results must reflect the second search query "o", not the first "op".
	for _, r := range state.results {
		if !strings.Contains(strings.ToLower(r), "o") {
			t.Fatalf("results contain item not matching 'o': %v", state.results)
		}
	}
	if got := strings.Join(state.results, ","); got != "sonnet,opus,freeform" {
		t.Fatalf("results = %q, want sonnet,opus,freeform for query 'o'", got)
	}
}

func TestViewRendersSearchState(t *testing.T) {
	state := State{searching: true}
	state.query.Value = "o"

	frame := View(&state, flatte.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Search", "query: o", "searching..."} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewPlacesCursorOnlyWhenFocused(t *testing.T) {
	state := State{focused: true}
	state.query.Value = "ok"
	state.query.Cursor = 2

	frame := View(&state, flatte.RenderContext{Width: 72})
	if frame.Cursor == nil {
		t.Fatal("focused view has no cursor")
	}
	// row: card border(1) + title,subtle,blank(3) = 4
	// col: card origin(3) + "  query: "(9) + 2 typed cells = 14
	if frame.Cursor.X != 14 || frame.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (14,4)", *frame.Cursor)
	}
	if strings.Contains(frame.Content, "\u258c") {
		t.Fatalf("View() still paints the fake cursor marker:\n%s", frame.Content)
	}

	state.focused = false
	if blurred := View(&state, flatte.RenderContext{Width: 72}); blurred.Cursor != nil {
		t.Fatalf("blurred view still has a cursor: %+v", *blurred.Cursor)
	}
}

func TestViewMatchesFocusedSearchingSnapshot(t *testing.T) {
	state := State{focused: true, searching: true}
	state.query.Value = "o"
	state.query.Cursor = 1

	flatest.AssertGoldenFrame(t, "testdata/focused-searching.golden", View(&state, flatte.RenderContext{Width: 72}))
}

func TestViewMatchesResultsSnapshot(t *testing.T) {
	state := State{
		query:   newSearchField("o"),
		results: []string{"sonnet", "opus", "freeform"},
	}

	flatest.AssertGoldenFrame(t, "testdata/results.golden", View(&state, flatte.RenderContext{Width: 72}))
}

func TestSearchDelayEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "25ms")

	if got := searchDelay(); got != 25*time.Millisecond {
		t.Fatalf("searchDelay() = %s, want 25ms", got)
	}
}

func newSearchField(value string) flatui.TextField {
	return flatui.TextField{Value: value, Cursor: len(value)}
}

func receiveSearchUpdate(t *testing.T, updates <-chan flatte.StateUpdate[State]) flatte.StateUpdate[State] {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for search update")
		return nil
	}
}
