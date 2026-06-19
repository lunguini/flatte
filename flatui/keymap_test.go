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
