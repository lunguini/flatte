package flatui

import "testing"

func TestKeyMapViewFiltersDisabledBindings(t *testing.T) {
	km := KeyMap{
		{Keys: []string{"tab"}, Help: "focus"},
		{Keys: []string{"enter", "space"}, Help: "toggle"},
		{Keys: []string{"q"}, Help: "quit", Disabled: true},
	}
	got := km.View()
	want := "tab focus  enter/space toggle"
	if got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestKeyGroupsShortModeUsesFirstEnabledBindingPerGroup(t *testing.T) {
	groups := KeyGroups{
		{Title: "nav", Bindings: KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"up", "down"}, Help: "move"},
		}},
		{Title: "edit", Bindings: KeyMap{
			{Keys: []string{"x"}, Help: "disabled", Disabled: true},
			{Keys: []string{"type"}, Help: "search"},
		}},
	}

	got := groups.ViewWithOptions(KeyMapOptions{Mode: KeyMapShort})
	want := "nav: tab focus  edit: type search"
	if got != want {
		t.Fatalf("ViewWithOptions(short) = %q, want %q", got, want)
	}
}

func TestKeyGroupsFullModeIncludesAllEnabledBindings(t *testing.T) {
	groups := KeyGroups{
		{Title: "nav", Bindings: KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"up", "down"}, Help: "move"},
		}},
	}

	got := groups.ViewWithOptions(KeyMapOptions{Mode: KeyMapFull})
	want := "nav: tab focus  up/down move"
	if got != want {
		t.Fatalf("ViewWithOptions(full) = %q, want %q", got, want)
	}
}

func TestKeyGroupsFullModeWrapsToWidth(t *testing.T) {
	groups := KeyGroups{
		{Title: "nav", Bindings: KeyMap{
			{Keys: []string{"tab"}, Help: "focus"},
			{Keys: []string{"up", "down"}, Help: "move"},
		}},
		{Title: "app", Bindings: KeyMap{
			{Keys: []string{"esc"}, Help: "quit"},
		}},
	}

	got := groups.ViewWithOptions(KeyMapOptions{Mode: KeyMapFull, Width: 22})
	want := "nav: tab focus\nup/down move\napp: esc quit"
	if got != want {
		t.Fatalf("ViewWithOptions(wrap) = %q, want %q", got, want)
	}
}
