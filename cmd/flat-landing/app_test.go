package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestLandingViewUsesStyledSections(t *testing.T) {
	state := NewState()
	state.layout(92, 28)

	frame := View(state, flatte.RenderContext{Width: 92}).Content
	if !strings.Contains(frame, "\x1b[") {
		t.Fatalf("View() has no ANSI styling:\n%s", frame)
	}

	clean := flatest.CleanFrame(frame)
	for _, want := range []string{
		"Flatte",
		"Build TUIs like Go programs",
		"TextField",
		"Layout",
		"TinyGo first",
	} {
		if !strings.Contains(clean, want) {
			t.Fatalf("View() missing %q:\n%s", want, clean)
		}
	}
}

func TestLandingViewFitsRequestedWidth(t *testing.T) {
	state := NewState()
	state.layout(64, 20)

	frame := flatest.CleanFrame(View(state, flatte.RenderContext{Width: 64}).Content)
	for _, line := range strings.Split(frame, "\n") {
		if width := lipgloss.Width(line); width > 64 {
			t.Fatalf("line width = %d > 64:\n%s", width, frame)
		}
	}
}

func TestLandingHandleMovesSelectedProof(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	if got, want := state.catalog.Cursor(), 1; got != want {
		t.Fatalf("catalog cursor after j = %d, want %d", got, want)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})
	if got, want := state.catalog.Cursor(), 2; got != want {
		t.Fatalf("catalog cursor after down = %d, want %d", got, want)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'k'}, flatte.Effects[State]{})
	if got, want := state.catalog.Cursor(), 1; got != want {
		t.Fatalf("catalog cursor after k = %d, want %d", got, want)
	}
}

func TestLandingSearchFiltersComponents(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '/'}, flatte.Effects[State]{})
	for _, r := range "viewport" {
		Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
	}

	if got, want := state.search.Value, "viewport"; got != want {
		t.Fatalf("search value = %q, want %q", got, want)
	}
	if got, want := len(state.filtered), 1; got != want {
		t.Fatalf("filtered count = %d, want %d", got, want)
	}
	if item := state.selectedItem(); item.Name != "Viewport" {
		t.Fatalf("selected item after filter = %s, want Viewport", item.Name)
	}
}

func TestLandingSearchBackspaceDeletesQuery(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '/'}, flatte.Effects[State]{})
	for _, r := range "tree" {
		Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
	}
	Handle(state, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{})

	if got, want := state.search.Value, "tre"; got != want {
		t.Fatalf("search value after Backspace = %q, want %q", got, want)
	}
}

func TestLandingTabKeysSwitchSections(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyTab}, flatte.Effects[State]{})
	if got, want := state.activeTab, tabComponents; got != want {
		t.Fatalf("active tab after Tab = %d, want %d", got, want)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if got, want := state.activeTab, tabOverview; got != want {
		t.Fatalf("active tab after Shift-Tab = %d, want %d", got, want)
	}
}

func TestLandingMouseSelectsTabAndCatalogRow(t *testing.T) {
	state := NewState()
	state.layout(92, 28)
	View(state, flatte.RenderContext{Width: 92})

	components := state.rects["tabs:components"]
	Handle(state, flatte.MouseEvent{X: components.X, Y: components.Y, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
	if got, want := state.activeTab, tabComponents; got != want {
		t.Fatalf("active tab after mouse = %d, want %d", got, want)
	}

	View(state, flatte.RenderContext{Width: 92})
	list := state.rects["catalog-list"]
	Handle(state, flatte.MouseEvent{X: list.X + 1, Y: list.Y + 5, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
	if got, want := state.catalog.Cursor(), 2; got != want {
		t.Fatalf("catalog cursor after mouse = %d, want %d", got, want)
	}
}

func TestLandingViewRecordsLayoutRects(t *testing.T) {
	state := NewState()
	state.layout(92, 28)

	View(state, flatte.RenderContext{Width: 92})

	for _, id := range []string{"header", "tabs", "catalog-list", "detail-pane", "feature-pane", "footer"} {
		rect, ok := state.rects[id]
		if !ok {
			t.Fatalf("View() did not record rect %q; rects=%v", id, state.rects)
		}
		if rect.W <= 0 || rect.H <= 0 {
			t.Fatalf("rect %q = %+v, want positive size", id, rect)
		}
	}
}

func TestANSIToHTMLPreservesBoxDrawingRunes(t *testing.T) {
	html := ansiToHTML("╭─╮\n│x│\n╰─╯")
	for _, want := range []string{"╭", "│", "╰"} {
		if !strings.Contains(html, want) {
			t.Fatalf("ansiToHTML() missing %q in %q", want, html)
		}
	}
}

func TestCSSPixelsParsesComputedStyles(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64
		ok   bool
	}{
		{in: "24px", want: 24, ok: true},
		{in: " 11.5px ", want: 11.5, ok: true},
		{in: "normal", ok: false},
		{in: "", ok: false},
	} {
		got, ok := cssPixels(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("cssPixels(%q) = %.2f, %v; want %.2f, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLandingViewMatchesSnapshot(t *testing.T) {
	state := NewState()
	state.layout(92, 28)

	flatest.AssertGolden(t, "testdata/landing.golden", View(state, flatte.RenderContext{Width: 92}).Content)
}
