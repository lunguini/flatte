package flatte

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func ch(r rune, mod Mod) KeyEvent { return KeyEvent{Key: KeyCharacter, Rune: r, Mod: mod} }

var testMenus = []Menu{
	{Title: "File", Items: []MenuItem{
		{ID: "save", Label: "Save", Keys: []KeyEvent{ch('s', ModSuper), ch('s', ModCtrl)}},
		{Separator: true, Keys: []KeyEvent{ch('x', 0)}},
		{ID: "print", Label: "Print", Keys: []KeyEvent{ch('p', ModCtrl)}, Disabled: true},
		{Label: "Export", Items: []MenuItem{{ID: "pdf", Label: "PDF", Keys: []KeyEvent{{Key: KeyF5}}}}},
	}},
}

func TestMenuShortcut(t *testing.T) {
	for _, c := range []struct {
		key  KeyEvent
		want string
		ok   bool
	}{
		{ch('s', ModSuper), "save", true},
		{ch('S', ModCtrl), "save", true}, // either case
		{ch('s', ModCtrl|ModShift), "", false},
		{ch('p', ModCtrl), "print", true},
		{KeyEvent{Key: KeyF5}, "pdf", true}, // in a submenu
		{ch('x', 0), "", false},             // separators have no keys
	} {
		it, ok := MenuShortcut(testMenus, c.key)
		if ok != c.ok || it.ID != c.want {
			t.Errorf("%+v: %q %v, want %q %v", c.key, it.ID, ok, c.want, c.ok)
		}
	}
}

func TestMenuHostRoutesShortcuts(t *testing.T) {
	var m MenuHost
	ev, ok := m.Route(ch('s', ModCtrl), testMenus, nil, 80, 24)
	if !ok || ev != (MenuEvent{ID: "save"}) {
		t.Errorf("ctrl+s: %#v %v", ev, ok)
	}
	if ev, ok := m.Route(ch('p', ModCtrl), testMenus, nil, 80, 24); ok {
		t.Errorf("a disabled item's key was delivered: %#v", ev)
	}
	if ev, ok := m.Route(ch('q', 0), testMenus, nil, 80, 24); !ok || ev != ch('q', 0) {
		t.Errorf("other keys pass: %#v %v", ev, ok)
	}
}

var ctxItems = []MenuItem{
	{ID: "cut", Label: "Cut", Keys: []KeyEvent{ch('x', ModCtrl)}},
	{ID: "copy", Label: "Copy", Checked: true},
	{Separator: true},
	{ID: "gone", Label: "Paste", Disabled: true},
	{Label: "More", Items: []MenuItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}},
}

func ctxFor(target string) []MenuItem {
	if target == "none" {
		return nil
	}
	return ctxItems
}

func rightClick(x, y int, target string) MouseEvent {
	return MouseEvent{X: x, Y: y, Button: MouseRight, Action: MousePress, Target: target}
}

func TestMenuHostContextMenuKeys(t *testing.T) {
	var m MenuHost
	route := func(ev Event) (Event, bool) { return m.Route(ev, nil, ctxFor, 40, 12) }
	if ev, ok := route(rightClick(3, 2, "none")); !ok || m.IsOpen() || ev.(MouseEvent).Target != "none" {
		t.Fatal("no menu for the target: the click passes")
	}
	if _, ok := route(rightClick(3, 2, "doc")); ok || !m.IsOpen() {
		t.Fatal("right-click did not open the menu")
	}
	l := m.levels[0]
	if l.x != 3 || l.y != 2 || l.hi != 0 {
		t.Fatalf("level %+v", l)
	}
	// While open, other events go nowhere but resize does.
	if _, ok := route(ch('z', 0)); ok {
		t.Error("a key leaked to the app")
	}
	// Down skips the separator and the disabled item.
	route(KeyEvent{Key: KeyDown})
	route(KeyEvent{Key: KeyDown})
	if m.levels[0].hi != 4 {
		t.Fatalf("highlight %d, want More (4)", m.levels[0].hi)
	}
	route(KeyEvent{Key: KeyDown}) // wraps
	if m.levels[0].hi != 0 {
		t.Fatalf("wrap: %d", m.levels[0].hi)
	}
	route(KeyEvent{Key: KeyUp})
	route(KeyEvent{Key: KeyRight}) // opens More
	if len(m.levels) != 2 || m.levels[1].x != l.x+l.w-1 || m.levels[1].y != l.y+1+4 {
		t.Fatalf("submenu %+v under %+v", m.levels, l)
	}
	route(KeyEvent{Key: KeyLeft})
	if len(m.levels) != 1 {
		t.Fatal("← did not close the submenu")
	}
	route(KeyEvent{Key: KeyEnter}) // reopens
	route(KeyEvent{Key: KeyDown})
	ev, ok := route(KeyEvent{Key: KeyEnter})
	if !ok || ev != (MenuEvent{ID: "b", Target: "doc"}) || m.IsOpen() {
		t.Fatalf("choose: %#v %v open %v", ev, ok, m.IsOpen())
	}
	// Escape closes one level at a time.
	route(rightClick(0, 0, "doc"))
	route(KeyEvent{Key: KeyUp})
	route(KeyEvent{Key: KeyEnter})
	route(KeyEvent{Key: KeyEscape})
	if len(m.levels) != 1 {
		t.Fatal("esc closed more than the submenu")
	}
	route(KeyEvent{Key: KeyEscape})
	if m.IsOpen() || m.target != "" {
		t.Fatal("esc did not close the menu")
	}
	route(rightClick(0, 0, "doc"))
	if ev, ok := route(ResizeEvent{Width: 10, Height: 5}); !ok || m.IsOpen() || ev != (ResizeEvent{Width: 10, Height: 5}) {
		t.Fatal("resize must close the menu and pass")
	}
}

func TestMenuHostContextMenuMouse(t *testing.T) {
	var m MenuHost
	route := func(ev Event) (Event, bool) { return m.Route(ev, nil, ctxFor, 40, 12) }
	route(rightClick(5, 1, "doc"))
	l := m.levels[0]
	// Motion highlights; over the separator or a disabled item it does not.
	route(MouseEvent{X: l.x + 2, Y: l.y + 2, Action: MouseMotion})
	if m.levels[0].hi != 1 {
		t.Fatalf("hover: %d", m.levels[0].hi)
	}
	route(MouseEvent{X: l.x + 2, Y: l.y + 3, Action: MouseMotion})
	route(MouseEvent{X: l.x + 2, Y: l.y + 4, Action: MouseMotion})
	if m.levels[0].hi != 1 {
		t.Fatalf("hover moved onto a separator or disabled item: %d", m.levels[0].hi)
	}
	// A click on the border or a disabled item does nothing.
	if _, ok := route(MouseEvent{X: l.x, Y: l.y + 1, Button: MouseLeft, Action: MousePress}); ok || !m.IsOpen() {
		t.Fatal("border click")
	}
	if _, ok := route(MouseEvent{X: l.x + 2, Y: l.y + 4, Button: MouseLeft, Action: MousePress}); ok || !m.IsOpen() {
		t.Fatal("disabled click")
	}
	ev, ok := route(MouseEvent{X: l.x + 2, Y: l.y + 1, Button: MouseLeft, Action: MousePress})
	if !ok || ev != (MenuEvent{ID: "cut", Target: "doc"}) || m.IsOpen() {
		t.Fatalf("click: %#v %v", ev, ok)
	}
	// A click outside closes it without reaching the app.
	route(rightClick(5, 1, "doc"))
	if _, ok := route(MouseEvent{X: 39, Y: 11, Button: MouseLeft, Action: MousePress}); ok || m.IsOpen() {
		t.Fatal("outside click")
	}
}

func TestMenuHostKeepsMenusOnScreen(t *testing.T) {
	var m MenuHost
	m.Route(rightClick(38, 10, "doc"), nil, ctxFor, 40, 12)
	l := m.levels[0]
	if l.x+l.w != 40 || l.y+l.h != 12 {
		t.Fatalf("menu at %+v not inside 40×12", l)
	}
	// The submenu does not fit on the right: it flips left.
	m.Route(KeyEvent{Key: KeyUp}, nil, ctxFor, 40, 12)
	m.Route(KeyEvent{Key: KeyEnter}, nil, ctxFor, 40, 12)
	s := m.levels[1]
	if s.x+s.w != l.x+1 || s.y+s.h > 12 {
		t.Fatalf("submenu %+v beside %+v", s, l)
	}
}

func TestMenuHostPaint(t *testing.T) {
	var m MenuHost
	base := Frame{Content: strings.Repeat(strings.Repeat(".", 30)+"\n", 9) + strings.Repeat(".", 30), Cursor: &Cursor{X: 1, Y: 1}}
	if got := m.Paint(base); got != base {
		t.Fatal("a closed menu changed the frame")
	}
	m.Route(rightClick(2, 1, "doc"), nil, ctxFor, 30, 10)
	got := m.Paint(base)
	if got.Cursor != nil {
		t.Error("the cursor shows over the menu")
	}
	want := strings.Join([]string{
		"..............................",
		"..╭───────────────────╮.......",
		"..│   Cut    Ctrl+X   │.......",
		"..│ ✓ Copy            │.......",
		"..│───────────────────│.......",
		"..│   Paste           │.......",
		"..│   More          › │.......",
		"..╰───────────────────╯.......",
		"..............................",
		"..............................",
	}, "\n")
	if s := ansi.Strip(got.Content); s != want {
		t.Errorf("painted:\n%s\nwant:\n%s", s, want)
	}
	// The highlighted row is reversed.
	if !regexp.MustCompile(`\x1b\[([0-9]+;)*7(;[0-9]+)*m   Cut`).MatchString(got.Content) {
		t.Error("no highlight")
	}
}

func TestKeyLabel(t *testing.T) {
	for _, c := range []struct {
		keys []KeyEvent
		want string
	}{
		{nil, ""},
		{[]KeyEvent{ch('s', ModSuper), ch('s', ModCtrl)}, "Ctrl+S"},
		{[]KeyEvent{ch('z', ModSuper|ModShift)}, "Shift+Cmd+Z"},
		{[]KeyEvent{{Key: KeyF12}}, "F12"},
		{[]KeyEvent{{Key: KeyDelete, Mod: ModAlt}}, "Alt+Del"},
		{[]KeyEvent{ch(' ', ModCtrl)}, "Ctrl+Space"},
	} {
		if got := keyLabel(c.keys); got != c.want {
			t.Errorf("%+v: %q, want %q", c.keys, got, c.want)
		}
	}
}
