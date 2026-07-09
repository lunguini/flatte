package main

import (
	"context"
	"fmt"
	"html"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/cmd/internal/dockerapp"
	"github.com/lunguini/flatte/cmd/internal/snakeapp"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

// tickInterval is the host cadence for the intro animation and the hosted apps.
const tickInterval = 120 * time.Millisecond

// introTicks is how many ticks the ASCII intro plays before the shell reveals.
const introTicks = 22

type phase int

const (
	phaseIntro phase = iota
	phaseShell
)

type landingTab int

const (
	tabGame   landingTab = iota // hosts the real flat-snake app
	tabApp                      // hosts the real flat-docker app
	tabWhat                     // what Flatte is
	tabLayout                   // the layout engine
	tabUI                       // component picker + live preview
)

// leftTabs and rightTabs split the top bar: apps on the left, docs on the
// right. Indices within each bar map to the landingTab values above.
// rightTabs feeds the TabBar widget preview on the UI tab.
var rightTabs = []flatui.TabItem{{ID: "what", Label: "Flatte"}, {ID: "layout", Label: "Layout"}, {ID: "ui", Label: "UI"}}

// tabIDs maps each landingTab to its URL-fragment id for deep-linking.
var tabIDs = []string{"game", "app", "what", "layout", "ui"}

type showcaseItem struct {
	Name    string
	Kind    string
	Summary string
}

var showcase = []showcaseItem{
	{Name: "TextField", Kind: "input", Summary: "Grapheme-correct single-line input with app-owned focus policy."},
	{Name: "Textarea", Kind: "input", Summary: "Multiline editing, soft-wrap, selection, and real cursor placement."},
	{Name: "List", Kind: "selection", Summary: "Selection and scroll state over app-owned rows."},
	{Name: "Table", Kind: "data", Summary: "Aligned columns backed by List selection and keep-visible scrolling."},
	{Name: "Tree", Kind: "navigation", Summary: "Expandable hierarchy without a router or framework-owned focus."},
	{Name: "Viewport", Kind: "scrolling", Summary: "Scrollable content window with hard-wrap and clipping support."},
	{Name: "Progress", Kind: "feedback", Summary: "Styled progress bars with deterministic text output."},
	{Name: "Spinner", Kind: "feedback", Summary: "Every-driven animation without goroutine-per-component state."},
	{Name: "Timer", Kind: "time", Summary: "Countdown and stopwatch helpers driven by fake-clock-testable updates."},
	{Name: "TabBar", Kind: "navigation", Summary: "Layout-aware tabs with rect-backed hit testing."},
	{Name: "Layout", Kind: "composition", Summary: "One solve-and-compose pass for geometry, clipping, chrome, and overlays."},
}

type State struct {
	phase      phase
	frame      int
	introFrame int
	active     landingTab

	width  int
	height int
	rects  map[string]layout.Rect
	// coX/coY are the absolute top-left of the content pane's inner area (inside
	// the border+padding). Mouse events are translated by this offset before
	// being forwarded to a hosted app, whose own rects are frame-local.
	coX int
	coY int
	// uiList is the absolute rect of the UI-tab component list rows. It lives in
	// a field, not s.rects, because View replaces s.rects with the compose
	// result after contentView runs (which would drop a map entry set here).
	uiList layout.Rect
	// lastTick is the wall time of the previous host Tick, used to feed hosted
	// apps the real elapsed duration (the browser's setInterval can fire slower
	// than requested, which would otherwise slow the snake down).
	lastTick time.Time

	// hosted real apps. Each runs unchanged through its own Handle/View; a
	// sandboxed Effects keeps its async dormant so the host drives timing.
	game     *snakeapp.State
	gameFx   flatte.Effects[snakeapp.State]
	docker   *dockerapp.State
	dockerFx flatte.Effects[dockerapp.State]

	// UI-tab component picker + animated preview widgets.
	catalog flatui.List
	spin    flatui.Spinner
	prog    flatui.Progress
	timer   flatui.Timer
}

func NewState() *State {
	s := &State{
		width:    96,
		height:   32,
		rects:    map[string]layout.Rect{},
		game:     snakeapp.NewGame(1),
		gameFx:   sandboxFx[snakeapp.State](),
		docker:   dockerapp.NewStateFromSession(dockerapp.SessionState{}),
		dockerFx: sandboxFx[dockerapp.State](),
		spin:     flatui.NewSpinner(flatui.SpinnerDots),
		prog:     flatui.NewProgress(24),
		timer:    flatui.NewTimer(90 * time.Second),
	}
	s.catalog.SetCount(len(showcase))
	s.catalog.SetHeight(10)
	return s
}

// noopClock satisfies flatte.Clock but never fires, so Every/ScopeEvery armed
// under a sandboxed Effects stay dormant.
type noopClock struct{}

func (noopClock) Tick(context.Context, time.Duration, func(time.Time)) {}

// sandboxFx builds an Effects whose Quit is a no-op, whose goroutine dispatch is
// dropped, and whose clock never ticks — so a hosted app's Handle mutates state
// in place but its async effects and quit are neutralized.
func sandboxFx[T any]() flatte.Effects[T] {
	ch := make(chan flatte.StateUpdate[T], 1)
	return flatte.NewHarnessEffects[T](context.Background(), ch, func() {}, func(func()) {}, noopClock{})
}

func (s *State) layout(width, height int) {
	s.width = max(width, 64)
	s.height = max(height, 20)
}

// Tick advances the intro animation or the active hosted app. It is driven by
// flatte.Every in the terminal and setInterval in the browser (Flatte's async
// engine does not run under WASM), so the same stepping works on both paths.
func Tick(s *State, now time.Time) {
	s.frame++
	// Real elapsed time since the previous tick, clamped so a backgrounded tab
	// or the first tick can't teleport a hosted app forward.
	elapsed := tickInterval
	if !s.lastTick.IsZero() {
		if d := now.Sub(s.lastTick); d > 0 && d < 500*time.Millisecond {
			elapsed = d
		}
	}
	s.lastTick = now

	if s.phase == phaseIntro {
		// The banner animates in and then holds on "press any key" — the intro
		// waits for the user rather than auto-advancing. introFrame caps once the
		// reveal is complete; the color shimmer keeps running off s.frame.
		if s.introFrame < introTicks {
			s.introFrame++
		}
		return
	}
	switch s.active {
	case tabGame:
		snakeapp.TickElapsed(s.game, elapsed)
	case tabApp:
		dockerapp.Tick(s.docker, now, s.frame)
	case tabUI:
		s.spin.Tick()
		p := float64(s.frame%80) / 40.0
		if p > 1 {
			p = 2 - p
		}
		s.prog.SetPercent(p)
		s.timer.Tick(tickInterval)
		if s.timer.Done() {
			s.timer = flatui.NewTimer(90 * time.Second)
		}
	}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(ev.Width, ev.Height)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	case flatte.MouseEvent:
		handleMouse(s, ev)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	// The intro swallows the first key and enters the shell.
	if s.phase == phaseIntro {
		s.phase = phaseShell
		return
	}

	// Host quit (terminal only; the browser page never quits).
	if key.Key == flatte.KeyCharacter && (key.Rune == 'c' || key.Rune == 'C') && key.Mod.Contains(flatte.ModCtrl) {
		fx.Quit()
		return
	}

	// Global tab navigation that never collides with the hosted apps.
	if key.Mod.Contains(flatte.ModCtrl) {
		switch key.Key {
		case flatte.KeyLeft:
			s.prevTab()
			return
		case flatte.KeyRight:
			s.nextTab()
			return
		}
	}

	// The doc/UI tabs don't steer anything, so plain arrows and Tab switch too.
	if s.active >= tabWhat {
		switch key.Key {
		case flatte.KeyLeft:
			s.prevTab()
			return
		case flatte.KeyRight:
			s.nextTab()
			return
		case flatte.KeyTab:
			if key.Mod.Contains(flatte.ModShift) {
				s.prevTab()
			} else {
				s.nextTab()
			}
			return
		}
	}

	// Route everything else to the active tab's content.
	switch s.active {
	case tabGame:
		snakeapp.Handle(s.game, key, s.gameFx)
	case tabApp:
		dockerapp.Handle(s.docker, key, s.dockerFx)
	case tabUI:
		s.handleUIKey(key)
	}
}

func (s *State) handleUIKey(key flatte.KeyEvent) {
	switch key.Key {
	case flatte.KeyUp:
		s.catalog.MoveUp()
	case flatte.KeyDown:
		s.catalog.MoveDown()
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'k', 'K':
			s.catalog.MoveUp()
		case 'j', 'J':
			s.catalog.MoveDown()
		}
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	if s.phase == phaseIntro {
		if m.Action == flatte.MousePress {
			s.phase = phaseShell
		}
		return
	}
	if m.Action != flatte.MousePress {
		// Forward wheel to the hosted app so docker/game panes scroll.
		s.forwardMouse(m)
		return
	}
	if tab, ok := tabHit(s.rects, m.X, m.Y); ok {
		s.active = tab
		return
	}
	s.forwardMouse(m)
}

func (s *State) forwardMouse(m flatte.MouseEvent) {
	switch s.active {
	case tabApp:
		// Translate host coordinates into the docker app's frame-local space so
		// its own rect-based hit-testing lines up with what's on screen.
		local := m
		local.X, local.Y = m.X-s.coX, m.Y-s.coY
		dockerapp.Handle(s.docker, local, s.dockerFx)
	case tabUI:
		r := s.uiList
		if !r.Contains(m.X, m.Y) {
			return
		}
		switch m.Button {
		case flatte.MouseWheelDown:
			s.catalog.MoveDown()
		case flatte.MouseWheelUp:
			s.catalog.MoveUp()
		default:
			s.catalog.Select(s.catalog.Offset() + (m.Y - r.Y))
		}
	}
}

func (s *State) nextTab() { s.active = (s.active + 1) % 5 }
func (s *State) prevTab() { s.active = (s.active + 4) % 5 }

func (s *State) selectedItem() showcaseItem { return showcase[s.catalog.Cursor()] }

// ---------------------------------------------------------------- rendering

// hostedAppRows is the frame height the embedded apps need. flat-snake is a
// fixed 24-row frame (its border is the collision wall, so it can't shrink);
// flat-docker reads well at the same height. The shell reserves this plus its
// own chrome so a hosted app's frame is never clipped inside the content pane.
const hostedAppRows = 24

func minRowsFor(active landingTab) int {
	if active == tabGame || active == tabApp {
		// content border (2) + tab bar (1) + footer (1) + two Col gaps (2).
		return hostedAppRows + 6
	}
	return 22
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	st := newStyles()
	width := max(max(ctx.Width, s.width), 64)
	height := max(s.height, minRowsFor(s.active))
	if s.phase == phaseIntro {
		return flatte.Frame{Content: trimRightLines(introView(s, st, width, height)), Title: "Flatte"}
	}
	content, rects := layout.SolveAndCompose(shellTree(s, st), width, height)
	s.rects = rects
	return flatte.Frame{Content: trimRightLines(content), Title: "Flatte"}
}

func shellTree(s *State, st styles) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{ID: "page", Gap: 1, PadLeft: 1, PadRight: 1},
		Children: []layout.Node{
			tabBarRow(s, st),
			landingBlock{
				NodeBase:      layout.NodeBase{ID: "content", H: layout.Grow(1), Bordered: true, PadLeft: 1, PadRight: 1},
				RenderContent: func(r layout.Rect, width int) string { return contentView(s, st, r, width) },
			},
			landingBlock{
				NodeBase:      layout.NodeBase{ID: "footer", H: layout.Fixed(1)},
				RenderContent: func(_ layout.Rect, width int) string { return footerView(s, st, width) },
			},
		},
	}
}

var tabLabels = []string{"Game", "App", "Flatte", "Layout", "UI"}

// tabBarRow renders the split top bar as one Row of ID'd Text leaves — apps on
// the left, docs on the right, a Grow spacer between. A single tab is
// highlighted across both groups, which the two-TabBar model can't express
// (SetActive can't clear a group), and each leaf's rect drives hit testing.
func tabBarRow(s *State, st styles) layout.Node {
	children := make([]layout.Node, 0, 6)
	children = append(children, tabNodes(s, st, 0, 2)...)
	children = append(children, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	children = append(children, tabNodes(s, st, 2, 5)...)
	return layout.Row{NodeBase: layout.NodeBase{ID: "tabbar", H: layout.Fixed(1), Gap: 1}, Children: children}
}

func tabNodes(s *State, st styles, lo, hi int) []layout.Node {
	out := make([]layout.Node, 0, hi-lo)
	for i := lo; i < hi; i++ {
		label := " " + tabLabels[i] + " "
		style := st.tab
		if landingTab(i) == s.active {
			style = st.tabActive
		}
		out = append(out, layout.Text{
			NodeBase: layout.NodeBase{ID: fmt.Sprintf("tab-%d", i), W: layout.Fixed(lipgloss.Width(label))},
			String:   style.Render(label),
		})
	}
	return out
}

func tabHit(rects map[string]layout.Rect, x, y int) (landingTab, bool) {
	for i := 0; i < len(tabLabels); i++ {
		if r, ok := rects[fmt.Sprintf("tab-%d", i)]; ok && r.Contains(x, y) {
			return landingTab(i), true
		}
	}
	return 0, false
}

func contentView(s *State, st styles, r layout.Rect, width int) string {
	height := max(r.H-2, 1)
	// Inner top-left of the content pane: border (1) + PadLeft (1) on x, border
	// (1) on y. Hosted apps render from here, so this is the mouse offset.
	s.coX, s.coY = r.X+2, r.Y+1
	switch s.active {
	case tabGame:
		snakeapp.Handle(s.game, flatte.ResizeEvent{Width: width, Height: height}, s.gameFx)
		return snakeapp.View(s.game, flatte.RenderContext{Width: width}).Content
	case tabApp:
		dockerapp.Handle(s.docker, flatte.ResizeEvent{Width: width, Height: height}, s.dockerFx)
		return dockerapp.View(s.docker, flatte.RenderContext{Width: width}).Content
	case tabLayout:
		return layoutTabView(st, width, height)
	case tabUI:
		return uiTabView(s, st, r, width, height)
	default:
		return whatTabView(st, width)
	}
}

func whatTabView(st styles, width int) string {
	w := max(min(width, 82), 1)
	rows := []string{
		st.section.Render("What Flatte is"),
		"",
		st.body.Render(wrapText("Flatte is a Go-native TUI foundation. You keep one mutable state struct, mutate it directly in Handle, and render one pure frame in View — no Msg/Cmd/Update loop, no message plumbing.", w)),
		"",
		st.selected.Render("state") + st.subtle.Render("   is your struct — the single source of truth"),
		st.selected.Render("view ") + st.subtle.Render("   is a pure function: state in, frame out"),
		st.selected.Render("async") + st.subtle.Render("   is one named, self-applying update"),
		"",
		st.body.Render(wrapText("Everything on these tabs — the snake game, the docker TUI, this text — is one Flatte app hosting others. Apps compose because they are just State + Handle + View.", w)),
		"",
		st.accent.Render("→ ") + st.subtle.Render("Ctrl+←/→ or click a tab to explore"),
	}
	return strings.Join(rows, "\n")
}

func layoutTabView(st styles, width, height int) string {
	// The demo spans the full content width so it reads as a real, functional
	// solve; prose wraps to the same width.
	w := max(width, 1)
	demo, _ := layout.SolveAndCompose(layout.Row{
		NodeBase: layout.NodeBase{Gap: 1, H: layout.Fixed(5)},
		Children: []layout.Node{
			layout.Text{NodeBase: layout.NodeBase{ID: "d1", Bordered: true, W: layout.Grow(1), H: layout.Fixed(5), PadLeft: 1}, String: st.accent.Render("sidebar\nGrow(1)")},
			layout.Text{NodeBase: layout.NodeBase{ID: "d2", Bordered: true, W: layout.Grow(2), H: layout.Fixed(5), PadLeft: 1}, String: st.body.Render("content\nGrow(2)")},
			layout.Text{NodeBase: layout.NodeBase{ID: "d3", Bordered: true, W: layout.Fixed(12), H: layout.Fixed(5), PadLeft: 1}, String: st.accent2.Render("aside\nFixed(12)")},
		},
	}, w, 5)
	rows := []string{
		st.section.Render("The layout engine"),
		"",
		st.body.Render(wrapText("flatui/layout is the app composition vocabulary. Build a Node tree of Row / Col / Text / Spacer, call SolveAndCompose once, and get composed cells plus per-ID rects from the same walk — so hit-testing and pixels never drift.", w)),
		"",
		demo,
		"",
		st.subtle.Render(wrapText("This whole page is one solved node tree; the panel above is a live solve at the full content width.", w)),
	}
	_ = height
	return strings.Join(rows, "\n")
}

func uiTabView(s *State, st styles, r layout.Rect, width, height int) string {
	listW := max(min(width/3, 20), 12)
	item := s.selectedItem()
	s.catalog.SetHeight(max(height-1, 1))
	list := s.catalog.View(func(i int, selected bool) string {
		prefix, style := "  ", st.body
		if selected {
			prefix, style = "▸ ", st.selected
		}
		return style.Render(truncate(prefix+showcase[i].Name, listW))
	})
	// Record the list-rows rect for mouse selection. The nested Row sits at the
	// content inner origin (coX, coY); the "Components" header takes one row, so
	// the first list row is at coY+1.
	s.uiList = layout.Rect{X: s.coX, Y: s.coY + 1, W: listW, H: max(height-1, 1)}
	_ = r

	previewW := max(width-listW-3, 8)
	preview := strings.Join([]string{
		st.section.Render(item.Name) + " " + st.subtle.Render("· "+item.Kind),
		st.subtle.Render(wrapText(item.Summary, previewW)),
		"",
		previewBody(s, st, item.Name, previewW, max(height-4, 1)),
	}, "\n")

	col, _ := layout.SolveAndCompose(layout.Row{
		NodeBase: layout.NodeBase{Gap: 1, H: layout.Fixed(max(height, 1))},
		Children: []layout.Node{
			layout.Text{NodeBase: layout.NodeBase{W: layout.Fixed(listW), H: layout.Grow(1)}, String: st.section.Render("Components") + "\n" + list},
			layout.Text{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)}, String: preview},
		},
	}, max(width, 1), max(height, 1))
	return col
}

// previewBody renders the selected widget in one or more live forms.
func previewBody(s *State, st styles, name string, width, height int) string {
	width, height = max(width, 1), max(height, 1)
	switch name {
	case "Spinner":
		return st.accent.Render(s.spin.View()) + " " + st.body.Render("working…") + "\n" +
			st.subtle.Render(wrapText("flatte.Every advances one frame per tick.", width))
	case "Progress":
		s.prog.SetWidth(clampInt(width-6, 8, 32))
		return st.accent.Render(s.prog.View()) + "\n" + st.subtle.Render(wrapText("Resize-aware, deterministic.", width))
	case "Timer":
		rem := s.timer.Remaining()
		return st.selected.Render(fmt.Sprintf("%02d:%02d", int(rem/time.Minute), int((rem%time.Minute)/time.Second))) + "\n" +
			st.subtle.Render(wrapText("Countdown · fake-clock testable.", width))
	case "TextField":
		return st.body.Render("name ") + st.selected.Render("Ada Lovelace") + st.accent.Render("▌") + "\n" +
			st.subtle.Render(wrapText("Grapheme-correct, app-owned cursor.", width))
	case "Textarea":
		var ta flatui.Textarea
		ta.SetSize(width, height)
		ta.SetValue("Multiline editing\nwith soft wrap and\nreal cursor cells.")
		return st.body.Render(ta.View())
	case "List":
		items := []string{"inbox", "starred", "sent", "drafts", "archive"}
		var l flatui.List
		l.SetHeight(min(height, len(items)))
		l.SetCount(len(items))
		l.Select(1)
		return l.View(func(i int, selected bool) string {
			if selected {
				return st.selected.Render("▸ " + items[i])
			}
			return st.body.Render("  " + items[i])
		})
	case "Table":
		var t flatui.Table
		t.SetColumns([]flatui.Column{{Title: "STEP", Width: 6}, {Title: "KIND", Width: 6}, {Title: "STATE", Width: 5}})
		t.SetRows([][]string{{"loop", "core", "ok"}, {"view", "pure", "ok"}, {"async", "named", "ok"}})
		t.SetHeight(4)
		body := t.View(func(text string, selected bool) string {
			if selected {
				return st.selected.Render(text)
			}
			return st.body.Render(text)
		})
		return st.section.Render(t.Header()) + "\n" + body
	case "Tree":
		tr := flatui.NewTree([]flatui.TreeNode{
			{ID: "state", Label: "State", Children: []flatui.TreeNode{{ID: "count", Label: "count"}, {ID: "items", Label: "items"}}},
			{ID: "view", Label: "View"},
		})
		tr.Toggle("state")
		tr.SetHeight(height)
		return tr.View(func(row flatui.TreeRow, selected bool) string {
			glyph := "  "
			if row.Expandable {
				if row.Expanded {
					glyph = "▾ "
				} else {
					glyph = "▸ "
				}
			}
			line := strings.Repeat("  ", row.Depth) + glyph + row.Label
			if selected {
				return st.selected.Render(line)
			}
			return st.body.Render(line)
		})
	case "Viewport":
		var vp flatui.Viewport
		vp.SetSize(width, height)
		vp.SetWrappedContent("Viewport keeps body content scrollable while the chrome stays pinned. Wheel and paging are app bindings, not framework policy.")
		return st.body.Render(vp.View())
	case "TabBar":
		bar := flatui.NewTabBar(rightTabs...).WithGlyphs(flatui.TabGlyphsSafe).WithColors(brand.Teal, lipgloss.Color("240"), nil)
		bar.SetActive(1)
		content, _ := layout.SolveAndCompose(bar.Layout(), width, 1)
		return content + "\n" + st.subtle.Render(wrapText("Rect-backed hit testing.", width))
	case "Layout":
		content, _ := layout.SolveAndCompose(layout.Row{
			NodeBase: layout.NodeBase{Gap: 1, H: layout.Fixed(3)},
			Children: []layout.Node{
				layout.Text{NodeBase: layout.NodeBase{Bordered: true, W: layout.Grow(1), H: layout.Fixed(3), PadLeft: 1}, String: st.accent.Render("side")},
				layout.Text{NodeBase: layout.NodeBase{Bordered: true, W: layout.Grow(2), H: layout.Fixed(3), PadLeft: 1}, String: st.body.Render("content")},
			},
		}, width, 3)
		return content
	default:
		return st.subtle.Render(wrapText("Select a component to preview it live.", width))
	}
}

func footerView(s *State, st styles, width int) string {
	var hint string
	switch s.active {
	case tabGame:
		hint = "Game · arrows/wasd steer · p pause · r restart"
	case tabApp:
		hint = "App · arrows move · tab focus · : palette · click to interact"
	case tabUI:
		hint = "UI · j/k or ↑/↓ pick a component"
	case tabLayout:
		hint = "Layout · one solved node tree, rects + pixels together"
	default:
		hint = "Flatte · direct mutation, pure views, no messages"
	}
	nav := st.subtle.Render("  Ctrl+←/→ tabs · click to switch")
	return st.accent.Render("▚ ") + st.body.Render(truncate(hint, max(width-24, 1))) + nav
}

// ------------------------------------------------------------------- intro

// bannerGlyphs is a 5-row block font for the letters in "FLATTE".
var bannerGlyphs = map[rune][5]string{
	'F': {"█████", "█    ", "████ ", "█    ", "█    "},
	'L': {"█    ", "█    ", "█    ", "█    ", "█████"},
	'A': {" ███ ", "█   █", "█████", "█   █", "█   █"},
	'T': {"█████", "  █  ", "  █  ", "  █  ", "  █  "},
	'E': {"█████", "█    ", "████ ", "█    ", "█████"},
}

func bannerRows(word string) []string {
	rows := make([]string, 5)
	for i := range rows {
		parts := make([]string, 0, len(word))
		for _, r := range word {
			parts = append(parts, bannerGlyphs[r][i])
		}
		rows[i] = strings.Join(parts, " ")
	}
	return rows
}

var bannerSweep = []color.Color{
	lipgloss.Color("#30C2B8"), lipgloss.Color("#49819A"),
	lipgloss.Color("#EF227D"), lipgloss.Color("#852467"),
}

func introView(s *State, st styles, width, height int) string {
	rows := bannerRows("FLATTE")
	full := utf8.RuneCountInString(rows[0])
	reveal := (s.introFrame + 1) * 4 // typed-on reveal, first slice visible immediately
	pad := strings.Repeat(" ", max((width-full)/2, 0))

	var banner []string
	for _, row := range rows {
		var b strings.Builder
		b.WriteString(pad)
		for i, r := range []rune(row) {
			if i >= reveal {
				break
			}
			if r == ' ' {
				b.WriteByte(' ')
				continue
			}
			c := bannerSweep[((i/3)+s.frame/2)%len(bannerSweep)]
			b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
		}
		banner = append(banner, b.String())
	}

	sub := ""
	if s.introFrame > introTicks/2 {
		sub = center(st.lead.Render("a Go-native TUI foundation"), width)
	}
	hint := ""
	if s.introFrame >= introTicks {
		// Blink the prompt once the reveal has finished and the intro is waiting.
		label := "press any key to continue"
		if (s.frame/4)%2 == 0 {
			hint = center(st.accent.Render("▚ "+label+" ▚"), width)
		} else {
			hint = center(st.subtle.Render("  "+label+"  "), width)
		}
	}

	// Vertically center the block.
	block := append([]string{}, banner...)
	block = append(block, "", sub, "", hint)
	topPad := max((height-len(block))/2, 0)
	out := make([]string, 0, topPad+len(block))
	for i := 0; i < topPad; i++ {
		out = append(out, "")
	}
	return strings.Join(append(out, block...), "\n")
}

func center(s string, width int) string {
	pad := max((width-lipgloss.Width(s))/2, 0)
	return strings.Repeat(" ", pad) + s
}

// -------------------------------------------------------------------- styles

type styles struct {
	logo      lipgloss.Style
	title     lipgloss.Style
	lead      lipgloss.Style
	body      lipgloss.Style
	subtle    lipgloss.Style
	section   lipgloss.Style
	selected  lipgloss.Style
	accent    lipgloss.Style
	accent2   lipgloss.Style
	tab       lipgloss.Style
	tabActive lipgloss.Style
}

func newStyles() styles {
	base := lipgloss.NewStyle()
	return styles{
		logo:      base.Bold(true).Foreground(brand.Teal),
		title:     base.Bold(true).Foreground(lipgloss.Color("252")),
		lead:      base.Foreground(brand.Blue),
		body:      base.Foreground(lipgloss.Color("252")),
		subtle:    base.Foreground(lipgloss.Color("245")),
		section:   base.Bold(true).Foreground(brand.Pink),
		selected:  base.Bold(true).Foreground(brand.Teal),
		accent:    base.Foreground(brand.Teal),
		accent2:   base.Foreground(brand.Pink),
		tab:       base.Foreground(lipgloss.Color("245")),
		tabActive: base.Bold(true).Foreground(lipgloss.Color("231")).Background(brand.Teal),
	}
}

// ---------------------------------------------------------------- layout leaf

type landingBlock struct {
	layout.NodeBase
	RenderContent func(layout.Rect, int) string
}

func (b landingBlock) Size() (layout.Size, layout.Size) { return b.NodeBase.Size() }

func (b landingBlock) Render(r layout.Rect) string {
	width := max(innerWidth(b.NodeBase, r.W), 1)
	content := ""
	if b.RenderContent != nil {
		content = b.RenderContent(r, width)
	}
	return layout.Text{NodeBase: b.NodeBase, String: content}.Render(r)
}

func innerWidth(base layout.NodeBase, width int) int {
	inset := base.PadLeft + base.PadRight + base.Pad*2
	if base.Bordered {
		inset += 2
	}
	return width - inset
}

// --------------------------------------------------------------- text utils

func wrapText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		if lipgloss.Width(next) > width && line != "" {
			lines = append(lines, line)
			line = word
			continue
		}
		line = next
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	out := ""
	for _, r := range s {
		if lipgloss.Width(out+string(r)) > width {
			break
		}
		out += string(r)
	}
	return out
}

func trimRightLines(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func clampInt(n, low, high int) int {
	if n < low {
		return low
	}
	if n > high {
		return high
	}
	return n
}

// ------------------------------------------------------------- ANSI to HTML

func cssPixels(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "px")
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

type sgrState struct {
	bold bool
	fg   string
}

func ansiToHTML(s string) string {
	var out strings.Builder
	state := sgrState{}
	open := false
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			end := i + 2
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end < len(s) {
				if open {
					out.WriteString("</span>")
					open = false
				}
				state = applySGR(state, s[i+2:end])
				if state.bold || state.fg != "" {
					out.WriteString(`<span style="`)
					if state.bold {
						out.WriteString("font-weight:700;")
					}
					if state.fg != "" {
						out.WriteString("color:")
						out.WriteString(state.fg)
						out.WriteByte(';')
					}
					out.WriteString(`">`)
					open = true
				}
				i = end + 1
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			out.WriteRune(r)
			i++
			continue
		}
		out.WriteString(html.EscapeString(string(r)))
		i += size
	}
	if open {
		out.WriteString("</span>")
	}
	return out.String()
}

func applySGR(state sgrState, params string) sgrState {
	if params == "" {
		return sgrState{}
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}
		switch code {
		case 0:
			state = sgrState{}
		case 1:
			state.bold = true
		case 22:
			state.bold = false
		case 39:
			state.fg = ""
		case 38:
			if i+4 < len(parts) && parts[i+1] == "2" {
				state.fg = rgb(atoi(parts[i+2]), atoi(parts[i+3]), atoi(parts[i+4]))
				i += 4
			} else if i+2 < len(parts) && parts[i+1] == "5" {
				state.fg = xterm256(atoi(parts[i+2]))
				i += 2
			}
		}
	}
	return state
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func xterm256(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	base := []string{"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0", "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}
	if n < len(base) {
		return base[n]
	}
	if n >= 232 {
		level := 8 + (n-232)*10
		return rgb(level, level, level)
	}
	n -= 16
	return rgb(cubeLevel(n/36), cubeLevel((n/6)%6), cubeLevel(n%6))
}

func cubeLevel(n int) int {
	if n == 0 {
		return 0
	}
	return 55 + n*40
}

func rgb(r, g, b int) string { return "#" + hex2(r) + hex2(g) + hex2(b) }

func hex2(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	const digits = "0123456789abcdef"
	return string([]byte{digits[n>>4], digits[n&15]})
}

// ---------------------------------------------------------------- terminal

func runTerminal() error {
	app := flatte.App[State]{
		State:  NewState(),
		Init:   func(_ *State, fx flatte.Effects[State]) { flatte.Every(fx, "host-tick", tickInterval, tickFold) },
		Handle: Handle,
		View:   View,
	}
	return flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion))
}

func tickFold(s *State, now time.Time) { Tick(s, now) }

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
