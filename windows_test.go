package flatte

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

type winState struct {
	windows []Window
	events  []Event
}

func winApp(s *winState) App[winState] {
	return App[winState]{
		State: s,
		Handle: func(s *winState, ev Event, _ Effects[winState]) {
			s.events = append(s.events, ev)
		},
		View: func(s *winState, ctx RenderContext) Frame {
			if ctx.Window == "" {
				return Frame{Content: strings.Repeat(strings.Repeat(".", 40)+"\n", 15) + strings.Repeat(".", 40)}
			}
			return Frame{Title: "Stats " + ctx.Window, Root: layout.Col{Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: ctx.Window + ":words"}, Text: "words: 3"},
				layout.Input{NodeBase: layout.NodeBase{ID: ctx.Window + ":in"}, Value: "abc", Caret: 1, Focused: true},
			}}}
		},
		Windows:     func(s *winState) []Window { return s.windows },
		ContextMenu: func(_ *winState, target string) []MenuItem { return []MenuItem{{ID: "on:" + target, Label: "Item"}} },
	}
}

func TestSyncWindowsFocus(t *testing.T) {
	s := &winState{}
	app := winApp(s)
	var wh WindowHost
	if evs := SyncWindows(&wh, app, 40, 16); evs != nil {
		t.Fatalf("no windows: %v", evs)
	}
	s.windows = []Window{{ID: "a", Width: 20, Height: 4}, {ID: ""}, {ID: "a"}}
	evs := SyncWindows(&wh, app, 40, 16)
	if len(wh.open) != 1 || len(evs) != 1 || evs[0] != (WindowEvent{Window: "a", Kind: WindowFocused}) {
		t.Fatalf("open %d, events %v (empty and duplicate IDs are ignored)", len(wh.open), evs)
	}
	a := wh.open[0]
	if a.w != 20 || a.h != 4 || a.x != 9 || a.y != 5 {
		t.Fatalf("geometry %+v", *a)
	}
	s.windows = append(s.windows, Window{ID: "b", Width: 100, Height: 100})
	evs = SyncWindows(&wh, app, 40, 16)
	b := wh.open[1]
	if len(evs) != 1 || wh.Focus() != "b" || b.w != 38 || b.h != 14 || b.x != 0 || b.y != 0 {
		t.Fatalf("second window: events %v focus %q geometry %+v", evs, wh.Focus(), *b)
	}
	if SyncWindows(&wh, app, 40, 16) != nil {
		t.Fatal("an unchanged declaration refocused")
	}
	s.windows = s.windows[:1]
	if evs := SyncWindows(&wh, app, 40, 16); len(evs) != 1 || wh.Focus() != "a" {
		t.Fatalf("closing the focused window: %v focus %q", evs, wh.Focus())
	}
	s.windows = nil
	if evs := SyncWindows(&wh, app, 40, 16); len(evs) != 1 || evs[0] != (WindowEvent{Kind: WindowFocused}) {
		t.Fatalf("closing the last: %v", evs)
	}
}

func TestPaintAndRouteWindows(t *testing.T) {
	s := &winState{windows: []Window{{ID: "stats", Width: 20, Height: 3}}}
	app := winApp(s)
	var wh WindowHost
	SyncWindows(&wh, app, 40, 16)
	var layouts []LayoutEvent
	base := app.View(s, RenderContext{})
	base.Cursor = &Cursor{}
	f := PaintWindows(&wh, app, base, func(ev LayoutEvent) { layouts = append(layouts, ev) })
	lines := strings.Split(ansi.Strip(f.Content), "\n")
	w := wh.open[0]
	want := []string{
		"╭─ Stats stats ───[×]╮",
		"│words: 3            │",
	}
	if !strings.HasPrefix(lines[w.y][w.x:], want[0]) {
		t.Fatalf("top rule in\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[w.y+1][w.x:], want[1]) {
		t.Fatalf("content line %q", lines[w.y+1][w.x:])
	}
	if len(layouts) != 1 || layouts[0].Window != "stats" || layouts[0].Rects["stats:words"] != (layout.Rect{W: 20, H: 1}) {
		t.Fatalf("layout events %+v", layouts)
	}
	// The focused window's caret places the cursor; the main one's hides.
	if f.Cursor == nil || f.Cursor.X != w.x+2 || f.Cursor.Y != w.y+2 {
		t.Fatalf("cursor %+v, want the window's caret", f.Cursor)
	}
	if fr, ok := wh.WindowFrame("stats"); !ok || fr.Title != "Stats stats" {
		t.Fatalf("WindowFrame %+v", fr)
	}

	// A click in the window is window-local and hit-tested there.
	evs := wh.Route(MouseEvent{X: w.x + 2, Y: w.y + 1, Button: MouseLeft, Action: MousePress, Target: "main"})
	if len(evs) != 1 {
		t.Fatalf("click in the focused window: %v", evs)
	}
	m := evs[0].(MouseEvent)
	if m.Window != "stats" || m.Target != "stats:words" || m.X != 1 || m.Y != 0 {
		t.Fatalf("located %+v", m)
	}
	// Outside: focus moves to the main window, then the click passes.
	evs = wh.Route(MouseEvent{X: 0, Y: 0, Button: MouseLeft, Action: MousePress, Target: "main"})
	if len(evs) != 2 || evs[0] != (WindowEvent{Kind: WindowFocused}) || evs[1].(MouseEvent).Target != "main" {
		t.Fatalf("outside: %v", evs)
	}
	// On the border: focus only. On the close mark: a close request.
	evs = wh.Route(MouseEvent{X: w.x, Y: w.y + 1, Button: MouseLeft, Action: MousePress})
	if len(evs) != 1 || evs[0] != (WindowEvent{Window: "stats", Kind: WindowFocused}) {
		t.Fatalf("border: %v", evs)
	}
	evs = wh.Route(MouseEvent{X: w.x + w.w - 1, Y: w.y, Button: MouseLeft, Action: MousePress})
	if len(evs) != 1 || evs[0] != (WindowEvent{Window: "stats", Kind: WindowCloseRequest}) {
		t.Fatalf("close mark: %v", evs)
	}
	// Motion over the main frame does not move focus.
	if evs := wh.Route(MouseEvent{X: 0, Y: 0, Action: MouseMotion}); len(evs) != 1 || wh.Focus() != "stats" {
		t.Fatalf("motion: %v focus %q", evs, wh.Focus())
	}
	// Keys pass untouched.
	if evs := wh.Route(KeyEvent{Key: KeyEnter}); len(evs) != 1 || evs[0] != (KeyEvent{Key: KeyEnter}) {
		t.Fatalf("key: %v", evs)
	}
}

func TestRouteInputContextMenuInAWindow(t *testing.T) {
	s := &winState{windows: []Window{{ID: "stats", Width: 20, Height: 3}}}
	app := winApp(s)
	var wh WindowHost
	var mh MenuHost
	SyncWindows(&wh, app, 40, 16)
	PaintWindows(&wh, app, app.View(s, RenderContext{}), nil)
	w := wh.open[0]
	if evs := RouteInput(&mh, &wh, app, MouseEvent{X: w.x + 2, Y: w.y + 1, Button: MouseRight, Action: MousePress}, 40, 16); len(evs) != 0 {
		t.Fatalf("the right-click was delivered: %v", evs)
	}
	// The menu opens at the pointer on screen, for the window's node.
	if !mh.IsOpen() || mh.levels[0].x != w.x+2 || mh.levels[0].y != w.y+1 || mh.target != "stats:words" {
		t.Fatalf("menu %+v target %q", mh.levels, mh.target)
	}
	evs := RouteInput(&mh, &wh, app, KeyEvent{Key: KeyEnter}, 40, 16)
	if len(evs) != 1 || evs[0] != (MenuEvent{ID: "on:stats:words", Target: "stats:words"}) {
		t.Fatalf("choice: %v", evs)
	}
	// A plain click in the window reaches the app window-local.
	evs = RouteInput(&mh, &wh, app, MouseEvent{X: w.x + 2, Y: w.y + 1, Button: MouseLeft, Action: MousePress}, 40, 16)
	if len(evs) != 1 || evs[0].(MouseEvent).X != 1 || evs[0].(MouseEvent).Window != "stats" {
		t.Fatalf("click: %v", evs)
	}
}
