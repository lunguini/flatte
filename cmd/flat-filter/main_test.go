package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func ready() *State {
	s := NewState()
	s.layout(14)
	return s
}

func key(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}
}

func typeQuery(s *State, text string) {
	for _, r := range text {
		Handle(s, key(r), flatte.Effects[State]{})
	}
}

func TestTypingFiltersVisibleList(t *testing.T) {
	s := ready()

	typeQuery(s, "api")

	if got := labels(s.currentPageIndexes()); strings.Join(got, ",") != "API gateway,API schema,API client" {
		t.Fatalf("filtered labels = %v, want API entries", got)
	}
	if s.list.Count() != 3 {
		t.Fatalf("list Count() = %d, want 3", s.list.Count())
	}
}

func TestSelectionClampsWhenFilterShrinks(t *testing.T) {
	s := ready()
	for range 4 {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})
	}
	if s.list.Cursor() != 4 {
		t.Fatalf("setup cursor = %d, want 4", s.list.Cursor())
	}

	typeQuery(s, "release")

	if s.list.Cursor() != 1 {
		t.Fatalf("cursor after shrink = %d, want 1", s.list.Cursor())
	}
	if got := labels(s.currentPageIndexes()); strings.Join(got, ",") != "Release notes,Release checklist" {
		t.Fatalf("filtered labels = %v, want release entries", got)
	}
}

func TestClearingQueryRestoresAllItems(t *testing.T) {
	s := ready()
	typeQuery(s, "api")
	for range 3 {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})
	}

	if len(s.filteredIndexes) != len(s.items) {
		t.Fatalf("filtered count = %d, want %d", len(s.filteredIndexes), len(s.items))
	}
	if got := labels(s.currentPageIndexes()); strings.Join(got, ",") != "API gateway,Auth flow,Build cache,Deploy queue,Docs index" {
		t.Fatalf("first page labels = %v, want restored first page", got)
	}
}

func TestPrintableQEditsAndEscapeQuits(t *testing.T) {
	s := ready()
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(s, key('q'), fx)
	if quit {
		t.Fatal("q should edit the query, not quit")
	}
	if s.query.Value != "q" {
		t.Fatalf("query = %q, want q", s.query.Value)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyEscape}, fx)
	if !quit {
		t.Fatal("escape should request quit")
	}
}

func TestPaginationLimitsVisibleRows(t *testing.T) {
	s := ready() // list height/page size 5

	if s.pages.Pages() != 3 {
		t.Fatalf("Pages() = %d, want 3", s.pages.Pages())
	}
	if got := labels(s.currentPageIndexes()); strings.Join(got, ",") != "API gateway,Auth flow,Build cache,Deploy queue,Docs index" {
		t.Fatalf("page 1 labels = %v, want first five items", got)
	}

	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})

	if s.pages.Page() != 1 {
		t.Fatalf("Page() = %d, want 1", s.pages.Page())
	}
	if got := labels(s.currentPageIndexes()); strings.Join(got, ",") != "Error budget,Feature flags,Graph report,Health check,Inbox triage" {
		t.Fatalf("page 2 labels = %v, want second five items", got)
	}
	if s.list.Count() != 5 {
		t.Fatalf("list Count() = %d, want page size 5", s.list.Count())
	}
}

func TestViewSnapshot(t *testing.T) {
	s := ready()
	typeQuery(s, "api")

	flatest.AssertGoldenFrame(t, "testdata/filter.golden", View(s, flatte.RenderContext{Width: 72}))
}

func labels(indexes []int) []string {
	out := make([]string, 0, len(indexes))
	for _, i := range indexes {
		out = append(out, items()[i].Title)
	}
	return out
}
