package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestHandleStartsSearchForTypedCharacters(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatcore.StateUpdate[State], 1)
	state := State{focused: true}

	Handle(&state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'o'}, flatcore.Effects[State]{
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
	updates := make(chan flatcore.StateUpdate[State], 4)
	fx := flatcore.NewEffects(t.Context(), updates, nil)
	state := &State{focused: true}

	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'j'}, fx)
	Handle(state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'k'}, fx)

	if state.query.Value != "jk" {
		t.Fatalf("query = %q, want %q", state.query.Value, "jk")
	}
}

func TestFocusedSearchCanTypeQ(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatcore.StateUpdate[State], 1)
	state := State{focused: true}

	Handle(&state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'q'}, flatcore.Effects[State]{
		Context: context.Background(),
		Updates: updates,
	})

	if state.query.Value != "q" {
		t.Fatalf("query = %q, want q", state.query.Value)
	}
}

func TestUnfocusedSearchUsesQToQuit(t *testing.T) {
	state := State{focused: false}
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(&state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("expected q to request quit when search input is unfocused")
	}
}

func TestBackspaceStartsSearchForEditedQuery(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	updates := make(chan flatcore.StateUpdate[State], 2)
	state := State{focused: true}
	state.query.Value = "op"
	state.query.Cursor = 2

	Handle(&state, flatcore.Event{Key: flatcore.KeyBackspace}, flatcore.Effects[State]{
		Context: context.Background(),
		Updates: updates,
	})

	if state.query.Value != "o" {
		t.Fatalf("query = %q, want o", state.query.Value)
	}
}

func TestStaleSearchUpdateIsIgnored(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "1ms")
	updates := make(chan flatcore.StateUpdate[State], 16)
	fx := flatcore.NewEffects(t.Context(), updates, nil)

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

	frame := View(&state, flatcore.RenderContext{Width: 72})

	for _, want := range []string{"Flat Search", "query: o", "searching..."} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesFocusedSearchingSnapshot(t *testing.T) {
	state := State{focused: true, searching: true}
	state.query.Value = "o"
	state.query.Cursor = 1

	flatuitest.AssertGolden(t, "testdata/focused-searching.golden", View(&state, flatcore.RenderContext{Width: 72}))
}

func TestViewMatchesResultsSnapshot(t *testing.T) {
	state := State{
		query:   newSearchField("o"),
		results: []string{"sonnet", "opus", "freeform"},
	}

	flatuitest.AssertGolden(t, "testdata/results.golden", View(&state, flatcore.RenderContext{Width: 72}))
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

func receiveSearchUpdate(t *testing.T, updates <-chan flatcore.StateUpdate[State]) flatcore.StateUpdate[State] {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for search update")
		return nil
	}
}
