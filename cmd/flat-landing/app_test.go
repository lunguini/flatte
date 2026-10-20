package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/dockerapp"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

// shellAt returns a state past the intro on the given tab, sized to a w×h
// window and settled through the host the browser uses.
func shellAt(active landingTab, w, h int) (*State, *pageHost) {
	s := NewState()
	s.phase = phaseShell
	s.active = active
	host := &pageHost{}
	host.handle(s, flatte.ResizeEvent{Width: w, Height: h})
	host.render(s)
	return s, host
}

// shellState is shellAt in a 92×28 window.
func shellState(active landingTab) (*State, *pageHost) { return shellAt(active, 92, 28) }

// send delivers ev through the host and settles the next frame.
func send(s *State, host *pageHost, ev flatte.Event) flatte.Frame {
	host.handle(s, ev)
	return host.render(s)
}

func press(x, y int) flatte.MouseEvent {
	return flatte.MouseEvent{X: x, Y: y, Button: flatte.MouseLeft, Action: flatte.MousePress}
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
	s, _ := shellState(tabGame)

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
	s, _ := shellState(tabUI)

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
	s, _ := shellState(tabGame)
	// A plain Right on the game tab must NOT switch tabs (it steers the snake).
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	if s.active != tabGame {
		t.Fatalf("active after plain Right on game tab = %d, want tabGame", s.active)
	}
}

func TestMouseSwitchesTab(t *testing.T) {
	s, host := shellState(tabGame)

	r := s.rects["tab-4"] // the UI tab
	if r.W == 0 {
		t.Fatalf("tab-4 rect not delivered: %v", s.rects)
	}
	send(s, host, press(r.X, r.Y))
	if s.active != tabUI {
		t.Fatalf("active after clicking tab-4 = %d, want tabUI", s.active)
	}
}

func TestUITabMouseSelectsRow(t *testing.T) {
	s, host := shellState(tabUI)

	r := s.rects["ui.list"]
	if r.H < 3 {
		t.Fatalf("ui-list rect missing/too small: %+v", r)
	}
	// Clicking the first list row must select index 0 (regression: an off-by-one
	// made the top rows unclickable).
	send(s, host, press(r.X+1, r.Y))
	if got := s.catalog.Cursor(); got != 0 {
		t.Fatalf("cursor after clicking first row = %d, want 0", got)
	}
	// Clicking the third row selects index 2.
	send(s, host, press(r.X+1, r.Y+2))
	if got := s.catalog.Cursor(); got != 2 {
		t.Fatalf("cursor after clicking third row = %d, want 2", got)
	}
}

func TestLayoutAdoptedBeforePaint(t *testing.T) {
	s, _ := shellState(tabWhat)
	for _, id := range []string{"tab-0", "tab-4", "content", "footer", "page", "shell"} {
		r, ok := s.rects[id]
		if !ok || r.W <= 0 || r.H <= 0 {
			t.Fatalf("rect %q missing or empty: %+v", id, r)
		}
	}
}

// TestTabSwitchAdoptsUnchangedGeometry: the content pane's rect is the same on
// both tabs, so no LayoutEvent follows the switch; the newly active tab must
// still size its widgets from the rects already delivered.
func TestTabSwitchAdoptsUnchangedGeometry(t *testing.T) {
	s, host := shellState(tabWhat)
	before := s.rects["content"]
	send(s, host, flatte.KeyEvent{Key: flatte.KeyRight}) // → Layout
	send(s, host, flatte.KeyEvent{Key: flatte.KeyRight}) // → UI
	if s.active != tabUI || s.rects["content"] != before {
		t.Fatalf("setup: active %d, content %+v (was %+v)", s.active, s.rects["content"], before)
	}
	if s.catalog.Height() <= 1 || s.rects["ui.list"].H != s.catalog.Height() {
		t.Fatalf("UI tab not sized from delivered geometry: list height %d, ui.list %+v", s.catalog.Height(), s.rects["ui.list"])
	}
}

func TestViewDoesNotMutateLayoutState(t *testing.T) {
	s, _ := shellState(tabUI)
	rects := make(map[string]layout.Rect, len(s.rects))
	for id, r := range s.rects {
		rects[id] = r
	}
	coX, coY := s.coX, s.coY
	listHeight, progressWidth := s.catalog.Height(), s.prog.Width()
	gameW, gameH, appW, appH := s.gameW, s.gameH, s.appW, s.appH

	_ = View(s, flatte.RenderContext{Width: 92})

	if !reflect.DeepEqual(s.rects, rects) || s.coX != coX || s.coY != coY {
		t.Fatalf("View mutated retained geometry")
	}
	if s.catalog.Height() != listHeight || s.prog.Width() != progressWidth {
		t.Fatalf("View resized UI widgets: list=%d/%d progress=%d/%d",
			s.catalog.Height(), listHeight, s.prog.Width(), progressWidth)
	}
	if s.gameW != gameW || s.gameH != gameH || s.appW != appW || s.appH != appH {
		t.Fatalf("View resized hosted apps")
	}
}

func TestShellViewFitsRequestedWidth(t *testing.T) {
	s, host := shellState(tabWhat)
	frame := flatest.CleanFrame(host.render(s).Content)
	for _, line := range strings.Split(frame, "\n") {
		if w := lipgloss.Width(line); w > 92 {
			t.Fatalf("line width = %d > 92:\n%s", w, frame)
		}
	}
}

func TestGameTabHostsSnake(t *testing.T) {
	s, host := shellState(tabGame)
	clean := flatest.CleanFrame(host.render(s).Content)
	// The snake app's chrome should render inside the content pane.
	if !strings.Contains(strings.ToLower(clean), "score") {
		t.Fatalf("game tab did not render the snake app:\n%s", clean)
	}
}

func TestTickAdvancesUIWidgets(t *testing.T) {
	s, _ := shellState(tabUI)
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

// The goldens were captured from the pre-Scroll View, which composed the
// shell itself at max(window, minRowsFor(tab)) rows. In a window at least
// that tall the runtime-composed page must be identical.
func TestShellViewMatchesSnapshot(t *testing.T) {
	for _, tc := range []struct {
		golden string
		tab    landingTab
		h      int
	}{
		{"testdata/landing.golden", tabWhat, 28},
		{"testdata/landing-layout.golden", tabLayout, 28},
		{"testdata/landing-ui.golden", tabUI, 28},
		{"testdata/landing-app.golden", tabApp, 32},
		{"testdata/landing-game.golden", tabGame, 32},
	} {
		s, host := shellAt(tc.tab, 92, tc.h)
		flatest.AssertGolden(t, tc.golden, host.render(s).Content)
	}
}

// TestTerminalRunMatchesHost: the terminal runtime (via flatest, which settles
// exactly as Run does) paints the same page as the browser host.
func TestTerminalRunMatchesHost(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 92)
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' '}) // leave the intro
	d.Send(flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModCtrl})
	d.Send(flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModCtrl}) // → Flatte
	d.Send(flatte.ResizeEvent{Width: 92, Height: 28})
	flatest.AssertGolden(t, "testdata/landing.golden", d.Frame().Content)
}

// pageLines is the window onto a taller page golden at offset y.
func pageLines(t *testing.T, golden string, y, rows int) string {
	t.Helper()
	full := strings.Split(string(mustRead(t, golden)), "\n")
	if y+rows > len(full) {
		t.Fatalf("%s has %d lines, need %d..%d", golden, len(full), y, y+rows)
	}
	return strings.Join(full[y:y+rows], "\n")
}

// TestShortWindowScrollsPage: in a window shorter than the shell, the page is
// the pre-Scroll frame (which overflowed the window) seen through a scrolled
// window: every row is reachable and none is altered.
func TestShortWindowScrollsPage(t *testing.T) {
	s, host := shellAt(tabWhat, 92, 16) // the shell needs 22 rows
	if got, want := flatest.CleanFrame(host.render(s).Content), pageLines(t, "testdata/landing-tall.golden", 0, 16); got != want {
		t.Fatalf("top of page:\n%s\nwant:\n%s", got, want)
	}
	send(s, host, flatte.MouseEvent{X: 40, Y: 5, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.pageY != wheelStep {
		t.Fatalf("pageY after one wheel notch = %d, want %d", s.pageY, wheelStep)
	}
	if got, want := flatest.CleanFrame(host.render(s).Content), pageLines(t, "testdata/landing-tall.golden", wheelStep, 16); got != want {
		t.Fatalf("after wheel:\n%s\nwant:\n%s", got, want)
	}
	// Paging past the end stops at the last row; one page up responds at once.
	send(s, host, flatte.KeyEvent{Key: flatte.KeyPageDown})
	frame := send(s, host, flatte.KeyEvent{Key: flatte.KeyPageDown})
	if s.pageY != 22-16 {
		t.Fatalf("pageY after paging down = %d, want 6", s.pageY)
	}
	if got, want := flatest.CleanFrame(frame.Content), pageLines(t, "testdata/landing-tall.golden", 6, 16); got != want {
		t.Fatalf("bottom of page:\n%s\nwant:\n%s", got, want)
	}
	send(s, host, flatte.KeyEvent{Key: flatte.KeyPageUp})
	if s.pageY != 0 {
		t.Fatalf("pageY after page up = %d, want 0", s.pageY)
	}
	// Growing the window past the shell leaves nothing to scroll.
	send(s, host, flatte.KeyEvent{Key: flatte.KeyPageDown})
	send(s, host, flatte.ResizeEvent{Width: 92, Height: 30})
	if s.pageY != 0 {
		t.Fatalf("pageY after growing the window = %d, want 0", s.pageY)
	}
}

// TestShortWindowAppTab: the hosted docker app keeps the wheel over its
// pane, the page scrolls from outside it, and forwarded clicks stay aligned
// with what is on screen at any page offset.
func TestShortWindowAppTab(t *testing.T) {
	s, host := shellAt(tabApp, 92, 20) // the shell needs 30 rows
	if got, want := flatest.CleanFrame(host.render(s).Content), pageLines(t, "testdata/landing-app-tall.golden", 0, 20); got != want {
		t.Fatalf("top of page:\n%s\nwant:\n%s", got, want)
	}
	tab := s.rects["tab-0"]
	send(s, host, flatte.MouseEvent{X: tab.X, Y: tab.Y, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.pageY != wheelStep {
		t.Fatalf("wheel over the tab bar: pageY = %d, want %d", s.pageY, wheelStep)
	}
	if got, want := flatest.CleanFrame(host.render(s).Content), pageLines(t, "testdata/landing-app-tall.golden", wheelStep, 20); got != want {
		t.Fatalf("scrolled page:\n%s\nwant:\n%s", got, want)
	}
	// Over the docker pane the wheel is docker's: it moves docker's
	// selection and leaves the page where it is.
	content := s.rects["content"]
	send(s, host, flatte.MouseEvent{X: content.X + 4, Y: content.Y + 8, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.pageY != wheelStep {
		t.Fatalf("wheel over the docker pane scrolled the page to %d", s.pageY)
	}
	if got := flatest.CleanFrame(host.render(s).Content); !strings.Contains(got, "●> api-server") {
		t.Fatalf("wheel over the docker pane did not reach docker:\n%s", got)
	}
	r, ok := s.dockerComp.Rects["header:images"]
	if !ok {
		t.Fatalf("docker header tab not solved: %v", s.dockerComp.Rects)
	}
	send(s, host, press(s.coX+r.X, s.coY+r.Y))
	if title := dockerapp.View(s.docker, flatte.RenderContext{}).Title; !strings.Contains(title, "images") {
		t.Fatalf("docker screen after a click on the scrolled page = %q, want images", title)
	}
}

// TestAppTabHostsDocker: the docker app renders inside the content pane with
// its list sized from its own settled layout (rows are visible).
func TestAppTabHostsDocker(t *testing.T) {
	s, host := shellState(tabApp)
	clean := flatest.CleanFrame(host.render(s).Content)
	for _, want := range []string{"containers", "nginx-proxy", "api-server"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("app tab missing %q:\n%s", want, clean)
		}
	}
}

// TestAppTabForwardsMouseWithTarget: a click forwarded into the hosted docker
// app carries the target from docker's own on-screen frame, so its tab strip
// switches screens.
func TestAppTabForwardsMouseWithTarget(t *testing.T) {
	s, host := shellState(tabApp)
	r, ok := s.dockerComp.Rects["header:images"]
	if !ok {
		t.Fatalf("docker header tab not solved: %v", s.dockerComp.Rects)
	}
	send(s, host, press(s.coX+r.X, s.coY+r.Y))
	if title := dockerapp.View(s.docker, flatte.RenderContext{}).Title; !strings.Contains(title, "images") {
		t.Fatalf("docker screen after forwarded click = %q, want images", title)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestUITabWheelOwnership: the wheel over the component list moves the
// catalog; over the preview it scrolls the page instead.
func TestUITabWheelOwnership(t *testing.T) {
	s, host := shellAt(tabUI, 92, 16) // the shell needs 22 rows: the page scrolls
	list := s.rects["ui.list"]
	send(s, host, flatte.MouseEvent{X: list.X + 1, Y: list.Y + 1, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.catalog.Cursor() != 1 || s.pageY != 0 {
		t.Fatalf("wheel over the list: cursor %d, pageY %d, want 1, 0", s.catalog.Cursor(), s.pageY)
	}
	preview := s.rects["ui.preview"]
	send(s, host, flatte.MouseEvent{X: preview.X + 2, Y: preview.Y + 2, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.catalog.Cursor() != 1 || s.pageY != wheelStep {
		t.Fatalf("wheel over the preview: cursor %d, pageY %d, want 1, %d", s.catalog.Cursor(), s.pageY, wheelStep)
	}
	// After the page scrolled, a click still selects the row painted there.
	list = s.rects["ui.list"]
	send(s, host, press(list.X+1, list.Y+3))
	if s.catalog.Cursor() != 3 {
		t.Fatalf("click on the fourth row of the scrolled page selected %d", s.catalog.Cursor())
	}
}

// pick selects a catalog item on the UI tab.
func pick(t *testing.T, s *State, host *pageHost, name string) {
	t.Helper()
	for i, item := range showcase {
		if item.Name == name {
			s.catalog.Select(i)
			host.render(s)
			return
		}
	}
	t.Fatalf("no showcase item %q", name)
}

func clickAt(s *State, host *pageHost, id string, dx int) {
	r := s.rects[id]
	send(s, host, flatte.MouseEvent{X: r.X + dx, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MousePress})
}

func TestUIPreviewsAreClickable(t *testing.T) {
	s, host := shellState(tabUI)

	pick(t, s, host, "TabBar")
	clickAt(s, host, demoTabsID+":ui", 1)
	if s.demo.tabs.Active() != 2 {
		t.Fatalf("tab bar click: active %d, want 2", s.demo.tabs.Active())
	}

	pick(t, s, host, "List")
	clickAt(s, host, demoListID+":3", 0)
	if s.demo.list.Cursor() != 3 {
		t.Fatalf("list click: cursor %d, want 3", s.demo.list.Cursor())
	}

	pick(t, s, host, "Table")
	clickAt(s, host, demoTableID+":2", 0)
	if s.demo.table.Cursor() != 2 {
		t.Fatalf("table click: cursor %d, want 2", s.demo.table.Cursor())
	}

	pick(t, s, host, "Tree")
	clickAt(s, host, demoTreeID+":view", 0)
	if s.demo.tree.CursorID() != "view" {
		t.Fatalf("tree click: cursor %q, want view", s.demo.tree.CursorID())
	}
	clickAt(s, host, demoTreeID+":state", 0)
	clickAt(s, host, demoTreeID+":state", 0) // again: closes it
	if len(s.demo.tree.VisibleRows()) != 2 {
		t.Fatalf("tree toggle: %d rows, want the two roots", len(s.demo.tree.VisibleRows()))
	}

	pick(t, s, host, "Viewport")
	r := s.rects[demoViewportID]
	send(s, host, flatte.MouseEvent{X: r.X, Y: r.Y, Button: flatte.MouseWheelDown, Action: flatte.MousePress})
	if s.demo.view.Offset() == 0 && s.demo.view.TotalLines() > s.demo.view.VisibleLines() {
		t.Fatal("wheel over the viewport preview did not scroll it")
	}
	if s.pageY != 0 {
		t.Fatalf("wheel over a preview scrolled the page to %d", s.pageY)
	}
}

func TestUITextPreviewsEdit(t *testing.T) {
	s, host := shellState(tabUI)
	pick(t, s, host, "TextField")
	clickAt(s, host, demoFieldID, 3) // after "Ada"
	if s.demo.editing != demoFieldID || s.demo.field.Cursor != 3 {
		t.Fatalf("field click: editing %q cursor %d", s.demo.editing, s.demo.field.Cursor)
	}
	frame := send(s, host, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'})
	if s.demo.field.Value != "Adaj Lovelace" || s.catalog.Cursor() != 0 {
		t.Fatalf("typing while editing: value %q catalog %d", s.demo.field.Value, s.catalog.Cursor())
	}
	if frame.Cursor == nil {
		t.Fatal("editing preview shows no cursor")
	}
	send(s, host, flatte.KeyEvent{Key: flatte.KeyEscape})
	send(s, host, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'})
	if s.demo.editing != "" || s.catalog.Cursor() != 1 {
		t.Fatalf("after Esc: editing %q catalog %d, want j to pick again", s.demo.editing, s.catalog.Cursor())
	}

	pick(t, s, host, "Textarea")
	clickAt(s, host, demoTextareaID, 0)
	send(s, host, flatte.PasteEvent{Text: "> "})
	if !strings.HasPrefix(s.demo.area.Value(), "> Multiline") {
		t.Fatalf("paste into the text area: %q", s.demo.area.Value())
	}
	// A click on the catalog stops editing.
	clickAt(s, host, "ui.item:0", 2)
	if s.demo.editing != "" {
		t.Fatalf("catalog click left editing %q", s.demo.editing)
	}
}
