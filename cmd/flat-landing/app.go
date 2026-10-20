package main

import (
	"context"
	"fmt"
	"html"
	"image/color"
	"math"
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

	// width/height are the window size from the last ResizeEvent; rects is the
	// geometry last delivered as a LayoutEvent (in window coordinates, so
	// already shifted by pageY).
	width  int
	height int
	rects  map[string]layout.Rect
	// pageY is the whole-page scroll offset: the shell is at least
	// minRowsFor(active) tall and scrolls when the window is shorter.
	pageY int
	// coX/coY are the absolute top-left of the content pane's inner area (inside
	// the border+padding). Mouse events are translated by this offset before
	// being forwarded to a hosted app, whose own rects are frame-local.
	coX   int
	coY   int
	gameW int
	gameH int
	appW  int
	appH  int
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
	// dockerLayout/dockerComp are this host's side of the docker app's
	// layout, as Run keeps them for a top-level app: the rects last delivered
	// as a LayoutEvent and the composition on screen, which forwarded mouse
	// events are hit-tested against.
	dockerLayout map[string]layout.Rect
	dockerComp   layout.Composition

	// UI-tab component picker + animated preview widgets.
	catalog flatui.List
	spin    flatui.Spinner
	prog    flatui.Progress
	timer   flatui.Timer
	demo    demo
}

func NewState() *State {
	s := &State{
		demo:     newDemo(),
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

// adopt is the sole geometry-adoption path: it sizes the active tab's hosted
// app and widgets from the rects last delivered as a LayoutEvent. Handle calls
// it on every LayoutEvent and after every other event, because switching tabs
// starts consuming geometry that may not have changed (and so is not
// delivered again). View only paints the already prepared state.
func (s *State) adopt() {
	r, ok := s.rects["content"]
	if !ok {
		return
	}
	contentWidth := max(innerWidth(contentNodeBase(), r.W), 1)
	contentHeight := max(r.H-2, 1)
	s.coX, s.coY = r.X+2, r.Y+1

	switch s.active {
	case tabGame:
		if s.gameW != contentWidth || s.gameH != contentHeight {
			s.gameW, s.gameH = contentWidth, contentHeight
			snakeapp.Handle(s.game, flatte.ResizeEvent{Width: contentWidth, Height: contentHeight}, s.gameFx)
		}
	case tabApp:
		if s.appW != contentWidth || s.appH != contentHeight {
			s.appW, s.appH = contentWidth, contentHeight
			dockerapp.Handle(s.docker, flatte.ResizeEvent{Width: contentWidth, Height: contentHeight}, s.dockerFx)
		}
		s.settleDocker()
	case tabUI:
		if r, ok := s.rects["ui.list"]; ok {
			s.catalog.SetHeight(max(r.H, 1))
		}
		if r, ok := s.rects["ui.preview"]; ok {
			s.prog.SetWidth(clampInt(previewWrapWidth(r.W)-6, 8, 32))
			s.demo.size(previewWrapWidth(r.W), r.H-4)
		}
	}
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
		s.settleDocker()
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
		// The new geometry arrives as a LayoutEvent before the next paint.
		s.width, s.height = ev.Width, ev.Height
		return
	case flatte.LayoutEvent:
		s.rects = ev.Rects
		s.pageY = s.clampPage(s.pageY)
		s.adopt()
		return
	}
	defer s.adopt()
	switch ev := ev.(type) {
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	case flatte.MouseEvent:
		handleMouse(s, ev)
	case flatte.PasteEvent:
		if s.active == tabUI {
			s.demo.paste(ev.Text)
		}
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

	// A text preview being edited takes the keys (Esc stops editing).
	if s.active == tabUI && s.demo.key(key) {
		return
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
		case flatte.KeyPageDown:
			s.scrollPage(s.rects["page"].H)
			return
		case flatte.KeyPageUp:
			s.scrollPage(-s.rects["page"].H)
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
	if m.Button == flatte.MouseWheelUp || m.Button == flatte.MouseWheelDown {
		// The docker app and the UI catalog scroll themselves; everywhere else
		// the wheel scrolls the page.
		if s.active == tabUI && s.demo.mouse(m, s.rects) {
			return // a preview scrolls itself
		}
		if (s.active == tabApp && m.Target == "content") || (s.active == tabUI && overCatalog(m.Target)) {
			s.forwardMouse(m)
			return
		}
		step := wheelStep
		if m.Button == flatte.MouseWheelUp {
			step = -step
		}
		s.scrollPage(step)
		return
	}
	if m.Action != flatte.MousePress {
		s.forwardMouse(m)
		return
	}
	if tab, ok := tabTarget(m.Target); ok {
		s.active = tab
		return
	}
	s.forwardMouse(m)
}

// wheelStep is how many rows one wheel notch scrolls the page.
const wheelStep = 3

func (s *State) scrollPage(by int) { s.pageY = s.clampPage(s.pageY + by) }

// clampPage bounds a page offset by the window and shell heights, the same
// rule the layout applies when painting.
func (s *State) clampPage(offset int) int {
	view, okV := s.rects["page"]
	shell, okS := s.rects["shell"]
	if !okV || !okS {
		return max(offset, 0)
	}
	return layout.ClampScroll(offset, view.H, shell.H)
}

func (s *State) forwardMouse(m flatte.MouseEvent) {
	switch s.active {
	case tabApp:
		// Translate host coordinates into the docker app's frame-local space so
		// its own rect-based hit-testing lines up with what's on screen.
		local := m
		local.X, local.Y = m.X-s.coX, m.Y-s.coY
		local.Target, local.TargetRect = s.dockerComp.HitTest(local.X, local.Y)
		local.Caret, local.HasCaret = s.dockerComp.CaretAt(local.X, local.Y)
		dockerapp.Handle(s.docker, local, s.dockerFx)
	case tabUI:
		if !overCatalog(m.Target) {
			s.demo.mouse(m, s.rects)
			return
		}
		if m.Action == flatte.MousePress {
			s.demo.editing = ""
		}
		switch m.Button {
		case flatte.MouseWheelDown:
			s.catalog.MoveDown()
		case flatte.MouseWheelUp:
			s.catalog.MoveUp()
		default:
			if i, err := strconv.Atoi(strings.TrimPrefix(m.Target, "ui.item:")); err == nil {
				s.catalog.Select(i)
			}
		}
	}
}

// overCatalog reports whether a pointer target is the UI tab's component
// list or one of its rows.
func overCatalog(target string) bool {
	return target == "ui.list" || strings.HasPrefix(target, "ui.item:")
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

// View returns the shell as a page-sized Scroll: the shell is laid out at
// least minRowsFor(active) tall so a hosted app is never squeezed, and a
// shorter window scrolls it instead of cutting it off.
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	st := newStyles()
	if s.phase == phaseIntro {
		width := max(ctx.Width, s.width)
		return flatte.Frame{Content: trimRightLines(introView(s, st, width, s.height)), Title: "Flatte"}
	}
	return flatte.Frame{
		Root:  layout.Scroll{NodeBase: layout.NodeBase{ID: "page"}, OffsetY: s.pageY, Child: shellTree(s, st)},
		Title: "Flatte",
	}
}

func contentNodeBase() layout.NodeBase {
	return layout.NodeBase{ID: "content", H: layout.Grow(1), Bordered: true, PadLeft: 1, PadRight: 1}
}

func shellTree(s *State, st styles) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{ID: "shell", H: layout.Fixed(max(s.height, minRowsFor(s.active))), Gap: 1, PadLeft: 1, PadRight: 1},
		Children: []layout.Node{
			tabBarRow(s, st),
			contentNode(s, st),
			footerNode(s, st),
		},
	}
}

var tabLabels = []string{"Game", "App", "Flatte", "Layout", "UI"}

// tabBarRow renders the split top bar as one Row of ID'd Labels — apps on
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
		out = append(out, layout.Label{
			NodeBase: layout.NodeBase{ID: fmt.Sprintf("tab-%d", i), W: layout.Fixed(lipgloss.Width(label))},
			Text:     label,
			Style:    style,
		})
	}
	return out
}

// tabTarget maps a hit target ("tab-N") to its tab.
func tabTarget(target string) (landingTab, bool) {
	n, ok := strings.CutPrefix(target, "tab-")
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(n)
	if err != nil || i < 0 || i >= len(tabLabels) {
		return 0, false
	}
	return landingTab(i), true
}

// contentNode is the bordered content pane. The UI tab's split is part of
// the frame tree, so its list rows are hit-tested and sized by the frame
// walk; the other tabs render as one leaf.
func contentNode(s *State, st styles) layout.Node {
	pane := layout.Col{NodeBase: contentNodeBase()}
	switch s.active {
	case tabUI:
		pane.Children = []layout.Node{uiTabNode(s, st)}
	case tabLayout:
		pane.Children = layoutTabNodes(st)
	case tabWhat:
		pane.Children = whatTabNodes(st, contentWidth(s))
	default:
		// A hosted app is its terminal frame: cells, by design.
		return landingBlock{
			NodeBase:      contentNodeBase(),
			RenderContent: func(r layout.Rect, width int) string { return hostedView(s, width) },
		}
	}
	return pane
}

// contentWidth is the text width inside the content pane.
func contentWidth(s *State) int {
	if r, ok := s.rects["content"]; ok {
		return innerWidth(contentNodeBase(), r.W)
	}
	return innerWidth(contentNodeBase(), s.width-2)
}

// hostedView is a hosted app's frame, as its own terminal would show it.
func hostedView(s *State, width int) string {
	if s.active == tabGame {
		return snakeapp.View(s.game, flatte.RenderContext{Width: width}).Content
	}
	frame, _ := flatte.ComposeFrame(dockerapp.View(s.docker, flatte.RenderContext{Width: s.appW}), s.appW, s.appH)
	return frame.Content
}

// prose is a paragraph wrapped by words at its width, in style.
func prose(text string, style layout.TextStyle) layout.Label {
	return layout.Label{Text: text, Style: style, Wrap: true}
}

// pair is a highlighted term and its explanation on one line.
func pair(term, rest string, st styles) layout.Node {
	return layout.Row{Children: []layout.Node{
		layout.Label{Text: term, Style: st.selected},
		layout.Label{Text: rest, Style: st.subtle},
	}}
}

func whatTabNodes(st styles, width int) []layout.Node {
	// Prose reads best at most 82 columns wide.
	text := layout.Col{NodeBase: layout.NodeBase{W: layout.Fixed(max(min(width, 82), 1))}, Children: []layout.Node{
		layout.Label{Text: "What Flatte is", Style: st.section},
		layout.Label{},
		prose("Flatte is a Go-native TUI foundation. You keep one mutable state struct, mutate it directly in Handle, and render one pure frame in View — no Msg/Cmd/Update loop, no message plumbing.", st.body),
		layout.Label{},
		pair("state", "   is your struct — the single source of truth", st),
		pair("view ", "   is a pure function: state in, frame out", st),
		pair("async", "   is one named, self-applying update", st),
		layout.Label{},
		prose("Everything on these tabs — the snake game, the docker TUI, this text — is one Flatte app hosting others. Apps compose because they are just State + Handle + View.", st.body),
		layout.Label{},
		layout.Row{Children: []layout.Node{
			layout.Label{Text: "→ ", Style: st.accent},
			layout.Label{Text: "Ctrl+←/→ or click a tab to explore", Style: st.subtle},
		}},
	}}
	return []layout.Node{text}
}

// layoutTabNodes is the layout tab: prose and a live demo solve spanning
// the content width.
func layoutTabNodes(st styles) []layout.Node {
	box := func(id string, w layout.Size, text string, style layout.TextStyle) layout.Node {
		return layout.Col{NodeBase: layout.NodeBase{ID: id, Bordered: true, W: w, H: layout.Fixed(5), PadLeft: 1}, Children: []layout.Node{
			layout.Label{Text: text, Style: style},
		}}
	}
	return []layout.Node{
		layout.Label{Text: "The layout engine", Style: st.section},
		layout.Label{},
		prose("flatui/layout is the app composition vocabulary. Build a Node tree of Row / Col / Text / Spacer, call SolveAndCompose once, and get composed cells plus per-ID rects from the same walk — so hit-testing and pixels never drift.", st.body),
		layout.Label{},
		layout.Row{NodeBase: layout.NodeBase{Gap: 1, H: layout.Fixed(5)}, Children: []layout.Node{
			box("d1", layout.Grow(1), "sidebar\nGrow(1)", st.accent),
			box("d2", layout.Grow(2), "content\nGrow(2)", st.body),
			box("d3", layout.Fixed(12), "aside\nFixed(12)", st.accent2),
		}},
		layout.Label{},
		prose("This whole page is one solved node tree; the panel above is a live solve at the full content width.", st.subtle),
	}
}

// uiListWidth is the component list's width for a content pane r wide.
func uiListWidth(r layout.Rect) int {
	return max(min(innerWidth(contentNodeBase(), r.W)/3, 20), 12)
}

// previewWrapWidth is the text width inside a preview column w wide.
func previewWrapWidth(w int) int { return max(w-2, 8) }

// uiTabNode is the component picker: a fixed-width list ("ui.list", whose
// rect sizes the catalog; its rows "ui.item:i" map clicks to items) beside
// the live preview, each widget shown as its own node.
func uiTabNode(s *State, st styles) layout.Node {
	listW := uiListWidth(s.rects["content"])
	list := s.catalog.Rows("ui.item", func(i int, selected bool) layout.Node {
		marker, style := layout.Icon{Text: "  "}, st.body
		if selected {
			marker, style = layout.Icon{Name: layout.IconChevronRight, Text: "▸ ", Style: st.selected}, st.selected
		}
		return layout.Row{Children: []layout.Node{marker, layout.Label{Text: showcase[i].Name, Style: style}}}
	})
	list.ID, list.H = "ui.list", layout.Grow(1)
	item := s.selectedItem()
	return layout.Row{
		NodeBase: layout.NodeBase{Gap: 1, H: layout.Grow(1)},
		Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{W: layout.Fixed(listW)}, Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: "Components", Style: st.section},
				list,
			}},
			// Text wraps two columns short of the preview's edge.
			layout.Col{NodeBase: layout.NodeBase{ID: "ui.preview", W: layout.Grow(1), H: layout.Grow(1), PadRight: 2}, Children: append([]layout.Node{
				layout.Row{Children: []layout.Node{
					layout.Label{Text: item.Name, Style: st.section},
					layout.Label{Text: " · " + item.Kind, Style: st.subtle},
				}},
				prose(item.Summary, st.subtle),
				layout.Label{},
			}, previewNodes(s, st, item.Name)...)},
		},
	}
}

// previewNodes shows the selected widget live, as the widget's own nodes:
// the demo's state, so clicks on them act (see demo.mouse).
func previewNodes(s *State, st styles, name string) []layout.Node {
	d := &s.demo
	note := func(text string) layout.Node { return prose(text, st.subtle) }
	switch name {
	case "Spinner":
		return []layout.Node{
			layout.Row{Children: []layout.Node{
				layout.Label{Text: s.spin.View(), Style: st.accent},
				layout.Label{Text: " working…", Style: st.body},
			}},
			note("flatte.Every advances one frame per tick."),
		}
	case "Progress":
		return []layout.Node{
			layout.Row{Children: []layout.Node{
				layout.Meter{NodeBase: layout.NodeBase{W: layout.Fixed(s.prog.Width())}, Value: s.prog.Percent() / 100, Fill: st.accent.Fg, Track: st.accent.Fg},
				layout.Label{Text: fmt.Sprintf("  %3d%%", int(math.Round(s.prog.Percent()))), Style: st.accent},
			}},
			note("Resize-aware, deterministic."),
		}
	case "Timer":
		rem := s.timer.Remaining()
		return []layout.Node{
			layout.Label{Text: fmt.Sprintf("%02d:%02d", int(rem/time.Minute), int((rem%time.Minute)/time.Second)), Style: st.selected},
			note("Countdown · fake-clock testable."),
		}
	case "TextField":
		field := d.field.Layout(demoFieldID, d.editing == demoFieldID)
		field.Style = st.selected
		return []layout.Node{
			layout.Row{Children: []layout.Node{layout.Label{Text: "name ", Style: st.body}, field}},
			note("Grapheme-correct, app-owned cursor. Click to edit; Esc stops."),
		}
	case "Textarea":
		in := d.area.Layout(demoTextareaID, d.editing == demoTextareaID)
		in.Style = st.body
		return []layout.Node{in}
	case "List":
		return []layout.Node{d.list.Rows(demoListID, func(i int, selected bool) layout.Node {
			if selected {
				return layout.Label{Text: "▸ " + demoListItems[i], Style: st.selected}
			}
			return layout.Label{Text: "  " + demoListItems[i], Style: st.body}
		})}
	case "Table":
		return []layout.Node{
			d.table.HeaderLayout(st.section),
			d.table.Rows(demoTableID, func(cells []string, selected bool) layout.Node {
				if selected {
					return d.table.Cells(cells, st.selected)
				}
				return d.table.Cells(cells, st.body)
			}),
		}
	case "Tree":
		return []layout.Node{d.tree.Rows(demoTreeID, func(row flatui.TreeRow, selected bool) layout.Node {
			style := st.body
			if selected {
				style = st.selected
			}
			glyph := layout.Icon{Text: "  ", Style: style}
			switch {
			case row.Expandable && row.Expanded:
				glyph.Name, glyph.Text = layout.IconDisclosureOpen, "▾ "
			case row.Expandable:
				glyph.Name, glyph.Text = layout.IconDisclosureClosed, "▸ "
			}
			return layout.Row{Children: []layout.Node{
				layout.Label{Text: strings.Repeat("  ", row.Depth), Style: style},
				glyph,
				layout.Label{Text: row.Label, Style: style},
			}}
		})}
	case "Viewport":
		return []layout.Node{layout.Col{NodeBase: layout.NodeBase{ID: demoViewportID}, Children: []layout.Node{d.view.Layout()}}}
	case "TabBar":
		return []layout.Node{d.tabs.Layout(), note("Rect-backed hit testing: click a tab.")}
	case "Layout":
		box := func(w layout.Size, text string, style layout.TextStyle) layout.Node {
			return layout.Col{NodeBase: layout.NodeBase{Bordered: true, W: w, H: layout.Fixed(3), PadLeft: 1}, Children: []layout.Node{
				layout.Label{Text: text, Style: style},
			}}
		}
		return []layout.Node{layout.Row{NodeBase: layout.NodeBase{Gap: 1, H: layout.Fixed(3)}, Children: []layout.Node{
			box(layout.Grow(1), "side", st.accent),
			box(layout.Grow(2), "content", st.body),
		}}}
	default:
		return []layout.Node{note("Select a component to preview it live.")}
	}
}

// footerNode is the footer: the active tab's hint and the tab navigation.
func footerNode(s *State, st styles) layout.Node {
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
	width := s.width - 2 // the shell's padding
	if r, ok := s.rects["footer"]; ok {
		width = r.W
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: "footer", H: layout.Fixed(1)}, Children: []layout.Node{
		layout.Label{Text: "▚ ", Style: st.accent},
		layout.Label{Text: truncate(hint, max(width-24, 1)), Style: st.body},
		layout.Label{Text: "  Ctrl+←/→ tabs · click to switch", Style: st.subtle},
	}}
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

// styles are the page's text styles: nodes take them as is, and Render
// gives a string frame (the intro) the same look.
type styles struct {
	lead      layout.TextStyle
	body      layout.TextStyle
	subtle    layout.TextStyle
	section   layout.TextStyle
	selected  layout.TextStyle
	accent    layout.TextStyle
	accent2   layout.TextStyle
	tab       layout.TextStyle
	tabActive layout.TextStyle
}

func newStyles() styles {
	return styles{
		lead:      layout.TextStyle{Fg: brand.Blue},
		body:      layout.TextStyle{Fg: lipgloss.Color("252")},
		subtle:    layout.TextStyle{Fg: lipgloss.Color("245")},
		section:   layout.TextStyle{Bold: true, Fg: brand.Pink},
		selected:  layout.TextStyle{Bold: true, Fg: brand.Teal},
		accent:    layout.TextStyle{Fg: brand.Teal},
		accent2:   layout.TextStyle{Fg: brand.Pink},
		tab:       layout.TextStyle{Fg: lipgloss.Color("245")},
		tabActive: layout.TextStyle{Bold: true, Fg: lipgloss.Color("231"), Bg: brand.Teal},
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
	return flatte.Run(context.Background(), landingApp(), flatte.WithMouse(flatte.MouseModeCellMotion))
}

// landingApp is the page as an app, ticking the intro and hosted apps.
func landingApp() flatte.App[State] {
	return flatte.App[State]{
		State: NewState(),
		Init: func(_ *State, fx flatte.Effects[State]) {
			if err := flatte.Every(fx, "host-tick", tickInterval, tickFold); err != nil {
				panic(err) // tickInterval is a sample invariant
			}
		},
		Handle: Handle,
		View:   View,
	}
}

func tickFold(s *State, now time.Time) { Tick(s, now) }

func exitOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// pageHost does for the browser loop what Run does in a terminal: it settles
// layout (delivering LayoutEvents) before each paint and hit-tests mouse
// events against the frame on screen. It lives outside the js build so tests
// drive the same path the browser does.
type pageHost struct {
	delivered map[string]layout.Rect
	comp      layout.Composition
}

func (h *pageHost) handle(s *State, ev flatte.Event) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		m.Target, m.TargetRect = h.comp.HitTest(m.X, m.Y)
		m.Caret, m.HasCaret = h.comp.CaretAt(m.X, m.Y)
		ev = m
	}
	Handle(s, ev, flatte.Effects[State]{})
}

func (h *pageHost) render(s *State) flatte.Frame {
	var frame flatte.Frame
	frame, h.comp, h.delivered = flatte.SettleFrame(
		func() flatte.Frame { return View(s, flatte.RenderContext{Width: s.width}) },
		func(ev flatte.LayoutEvent) { Handle(s, ev, flatte.Effects[State]{}) },
		h.delivered, s.width, s.height,
	)
	return frame
}

// settleDocker settles the hosted docker app's layout exactly as Run would
// for a top-level app: its frame tree is solved at the content-pane size and
// any changed geometry is delivered to it as a LayoutEvent. It runs after
// every event or tick that reaches docker, so View only composes.
func (s *State) settleDocker() {
	_, s.dockerComp, s.dockerLayout = flatte.SettleFrame(
		func() flatte.Frame { return dockerapp.View(s.docker, flatte.RenderContext{Width: s.appW}) },
		func(ev flatte.LayoutEvent) { dockerapp.Handle(s.docker, ev, s.dockerFx) },
		s.dockerLayout, s.appW, s.appH,
	)
}
