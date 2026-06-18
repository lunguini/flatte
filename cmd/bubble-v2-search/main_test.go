package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lunguini/flat/flatest"
	"github.com/lunguini/flat/flatui"
)

func TestTypedCharacterStartsSearchAndResultApplies(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	model := NewModel()

	next, cmd := model.Update(key('o', "o"))
	model = next.(Model)

	if model.query.Value != "o" {
		t.Fatalf("query = %q, want o", model.query.Value)
	}
	if !model.searching {
		t.Fatal("expected search to be marked in-flight")
	}
	if cmd == nil {
		t.Fatal("expected typing to return a search command")
	}

	model = updateModel(t, model, cmd())
	if model.searching {
		t.Fatal("expected search to finish after result message")
	}
	if got := strings.Join(model.results, ","); got != "sonnet,opus,freeform" {
		t.Fatalf("results = %q, want sonnet,opus,freeform", got)
	}
}

func TestTypingJAndKEditsQuery(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "1ms")
	model := NewModel()

	model = updateModel(t, model, key('j', "j"))
	model = updateModel(t, model, key('k', "k"))

	if model.query.Value != "jk" {
		t.Fatalf("query = %q, want %q", model.query.Value, "jk")
	}
}

func TestFocusedSearchCanTypeQ(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	model := NewModel()

	next, cmd := model.Update(key('q', "q"))
	model = next.(Model)

	if model.query.Value != "q" {
		t.Fatalf("query = %q, want q", model.query.Value)
	}
	if cmd == nil {
		t.Fatal("expected typing q to start a search, not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); ok {
		t.Fatal("q while focused must not quit")
	}
}

func TestUnfocusedSearchUsesQToQuit(t *testing.T) {
	model := NewModel()
	model.focused = false

	_, cmd := model.Update(key('q', "q"))

	if cmd == nil {
		t.Fatal("expected q to return a quit command when blurred")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestEnterTogglesFocus(t *testing.T) {
	model := NewModel()

	model = updateModel(t, model, key(tea.KeyEnter, ""))
	if model.focused {
		t.Fatal("expected enter to blur the query")
	}

	model = updateModel(t, model, key(tea.KeyEnter, ""))
	if !model.focused {
		t.Fatal("expected enter to focus the query again")
	}
}

func TestBackspaceStartsSearchForEditedQuery(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	model := NewModel()
	model.query = newSearchField("op")

	next, cmd := model.Update(key(tea.KeyBackspace, ""))
	model = next.(Model)

	if model.query.Value != "o" {
		t.Fatalf("query = %q, want o", model.query.Value)
	}
	if cmd == nil {
		t.Fatal("expected backspace edit to return a search command")
	}
}

func TestEmptyQueryClearsResultsWithoutSearching(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	model := NewModel()
	model.query = newSearchField("o")
	model.results = []string{"sonnet", "opus", "freeform"}

	next, cmd := model.Update(key(tea.KeyBackspace, ""))
	model = next.(Model)

	if model.searching {
		t.Fatal("expected empty query not to search")
	}
	if model.results != nil {
		t.Fatalf("results = %v, want nil", model.results)
	}
	if cmd != nil {
		t.Fatal("expected no search command for empty query")
	}
}

func TestStaleSearchResultIsIgnored(t *testing.T) {
	t.Setenv("FLAT_SEARCH_DELAY", "0s")
	model := NewModel()

	// First search: "o" — will be superseded immediately.
	next, staleCmd := model.Update(key('o', "o"))
	model = next.(Model)

	// Second search: "op" supersedes the first.
	next, freshCmd := model.Update(key('p', "p"))
	model = next.(Model)

	// The first search finishes after the second one started: its result
	// message carries an old generation and must be dropped.
	staleMsg := staleCmd().(searchResultMsg)
	model = updateModel(t, model, staleMsg)

	if !model.searching {
		t.Fatal("stale result must not end the in-flight search")
	}
	if model.results != nil {
		t.Fatalf("stale result overwrote results: %v", model.results)
	}

	// The fresh result applies normally.
	model = updateModel(t, model, freshCmd())
	if model.searching {
		t.Fatal("expected fresh result to finish the search")
	}
	if got := strings.Join(model.results, ","); got != "opus" {
		t.Fatalf("results = %q, want opus for query 'op'", got)
	}

	// Even after the fresh result applied, a late stale delivery is a no-op.
	model = updateModel(t, model, staleMsg)
	if got := strings.Join(model.results, ","); got != "opus" {
		t.Fatalf("late stale result overwrote results: %q", got)
	}
}

func TestViewRendersSearchState(t *testing.T) {
	model := NewModel()
	model.focused = false
	model.searching = true
	model.query.Value = "o"

	view := model.View()

	if !view.AltScreen {
		t.Fatal("expected v2 view to request alt screen")
	}
	for _, want := range []string{"Bubble v2 Search", "query: o", "searching..."} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("View() missing %q:\n%s", want, view.Content)
		}
	}
}

func TestViewMatchesFocusedSearchingSnapshot(t *testing.T) {
	model := NewModel()
	model.searching = true
	model.query.Value = "o"
	model.query.Cursor = 1

	flatest.AssertGolden(t, "testdata/focused-searching.golden", model.View().Content)
}

func TestViewMatchesResultsSnapshot(t *testing.T) {
	model := NewModel()
	model.focused = false
	model.query = newSearchField("o")
	model.results = []string{"sonnet", "opus", "freeform"}

	flatest.AssertGolden(t, "testdata/results.golden", model.View().Content)
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

func key(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()

	next, _ := model.Update(msg)
	return next.(Model)
}
