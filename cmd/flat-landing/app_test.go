package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// shellState returns a laid-out state past the intro, on the given tab.
func shellState(active landingTab) *State {
	s := NewState()
	s.phase = phaseShell
	s.active = active
	s.layout(92, 28)
	return s
}

func TestIntroWaitsForKey(t *testing.T) {
	s := NewState()
	if s.phase != phaseIntro {
		t.Fatalf("new state phase = %d, want intro", s.phase)
	}
	// The intro banner renders the word Flatte and holds before revealing.
	clean := flatest.CleanFrame(View(s, flatte.RenderContext{Width: 92}).Content)
	if !strings.Contains(clean, "█") {
		t.Fatalf("intro view has no banner glyphs:\n%s", clean)
	}
	// The intro must NOT auto-advance — it waits for the user.
	for i := 0; i < introTicks*3; i++ {
		Tick(s, time.Now())
	}
	if s.phase != phaseIntro {
		t.Fatalf("phase after %d ticks = %d, want intro (must wait for key)", introTicks*3, s.phase)
	}
	// Once revealed, the prompt is shown.
	if got := flatest.CleanFrame(View(s, flatte.RenderContext{Width: 92}).Content); !strings.Contains(got, "press any key") {
		t.Fatalf("intro missing continue prompt:\n%s", got)
	}
}

func TestKeySkipsIntro(t *testing.T) {
	s := NewState()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' '}, flatte.Effects[State]{})
	if s.phase != phaseShell {
		t.Fatalf("phase after key = %d, want shell", s.phase)
	}
}

func TestTabNavigationCyclesTabs(t *testing.T) {
	s := shellState(tabGame)

	// Ctrl+Right works from any tab, including the game tab where plain arrows
	// steer the snake.
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.active != tabApp {
		t.Fatalf("active after Ctrl+Right = %d, want tabApp", s.active)
	}

	// Plain arrows switch on the doc tabs (they don't steer anything).
	s.active = tabWhat
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	if s.active != tabLayout {
		t.Fatalf("active after Right on doc tab = %d, want tabLayout", s.active)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft}, flatte.Effects[State]{})
	if s.active != tabWhat {
		t.Fatalf("active after Left on doc tab = %d, want tabWhat", s.active)
	}
}

func TestUITabCatalogNavigation(t *testing.T) {
	s := shellState(tabUI)

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	if got := s.catalog.Cursor(); got != 1 {
		t.Fatalf("catalog cursor after j = %d, want 1", got)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})
	if got := s.catalog.Cursor(); got != 2 {
		t.Fatalf("catalog cursor after down = %d, want 2", got)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'k'}, flatte.Effects[State]{})
	if got := s.catalog.Cursor(); got != 1 {
		t.Fatalf("catalog cursor after k = %d, want 1", got)
	}
}

func TestGameTabDoesNotHijackTabNavForSteering(t *testing.T) {
	s := shellState(tabGame)
	// A plain Right on the game tab must NOT switch tabs (it steers the snake).
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	if s.active != tabGame {
		t.Fatalf("active after plain Right on game tab = %d, want tabGame", s.active)
	}
}

func TestMouseSwitchesTab(t *testing.T) {
	s := shellState(tabGame)
	View(s, flatte.RenderContext{Width: 92})

	r := s.rects["tab-4"] // the UI tab
	if r.W == 0 {
		t.Fatalf("tab-4 rect not recorded: %v", s.rects)
	}
	Handle(s, flatte.MouseEvent{X: r.X, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
	if s.active != tabUI {
		t.Fatalf("active after clicking tab-4 = %d, want tabUI", s.active)
	}
}

func TestUITabMouseSelectsRow(t *testing.T) {
	s := shellState(tabUI)
	View(s, flatte.RenderContext{Width: 92})

	r := s.uiList
	if r.H < 3 {
		t.Fatalf("ui-list rect missing/too small: %+v", r)
	}
	// Clicking the first list row must select index 0 (regression: an off-by-one
	// made the top rows unclickable).
	Handle(s, flatte.MouseEvent{X: r.X + 1, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
	if got := s.catalog.Cursor(); got != 0 {
		t.Fatalf("cursor after clicking first row = %d, want 0", got)
	}
	// Clicking the third row selects index 2.
	Handle(s, flatte.MouseEvent{X: r.X + 1, Y: r.Y + 2, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
	if got := s.catalog.Cursor(); got != 2 {
		t.Fatalf("cursor after clicking third row = %d, want 2", got)
	}
}

func TestViewRecordsTabRects(t *testing.T) {
	s := shellState(tabWhat)
	View(s, flatte.RenderContext{Width: 92})
	for _, id := range []string{"tab-0", "tab-4", "content", "footer"} {
		r, ok := s.rects[id]
		if !ok || r.W <= 0 || r.H <= 0 {
			t.Fatalf("rect %q missing or empty: %+v", id, r)
		}
	}
}

func TestShellViewFitsRequestedWidth(t *testing.T) {
	s := shellState(tabWhat)
	frame := flatest.CleanFrame(View(s, flatte.RenderContext{Width: 92}).Content)
	for _, line := range strings.Split(frame, "\n") {
		if w := lipgloss.Width(line); w > 92 {
			t.Fatalf("line width = %d > 92:\n%s", w, frame)
		}
	}
}

func TestGameTabHostsSnake(t *testing.T) {
	s := shellState(tabGame)
	clean := flatest.CleanFrame(View(s, flatte.RenderContext{Width: 92}).Content)
	// The snake app's chrome should render inside the content pane.
	if !strings.Contains(strings.ToLower(clean), "score") {
		t.Fatalf("game tab did not render the snake app:\n%s", clean)
	}
}

func TestTickAdvancesUIWidgets(t *testing.T) {
	s := shellState(tabUI)
	before := s.spin.Frame()
	Tick(s, time.Now())
	if s.frame != 1 {
		t.Fatalf("frame counter after Tick = %d, want 1", s.frame)
	}
	if s.spin.Frame() == before {
		t.Fatalf("spinner did not advance: %d", s.spin.Frame())
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

func TestShellViewMatchesSnapshot(t *testing.T) {
	s := shellState(tabWhat)
	flatest.AssertGolden(t, "testdata/landing.golden", View(s, flatte.RenderContext{Width: 92}).Content)
}
