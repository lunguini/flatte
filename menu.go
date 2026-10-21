package flatte

import (
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// Menu is a titled list of commands in the menu bar (Frame.Menus).
type Menu struct {
	Title string
	Items []MenuItem
}

// MenuItem is one command of a menu. Choosing it delivers a MenuEvent with
// its ID. Keys are its shortcuts: a host delivers a key equal to one of
// them (Key, Rune, and Mod; letters in either case) as the item's
// MenuEvent instead of a KeyEvent — list several to cover hosts, such as
// ⌘S for a window and Ctrl+S for a terminal. Most terminals can send only
// a plain Ctrl+letter: not with Shift, and not Ctrl+I, M, H, or [ (they
// are Tab, Enter, Backspace, and Esc). A Disabled item is shown
// dimmed and cannot be chosen; its shortcuts do nothing. Checked shows a
// check mark. Separator makes the item a divider (its other fields are
// ignored); Items makes it a submenu.
type MenuItem struct {
	ID                string
	Label             string
	Keys              []KeyEvent
	Checked, Disabled bool
	Separator         bool
	Items             []MenuItem
}

// MenuEvent reports a chosen menu item: from the menu bar, a shortcut, or a
// context menu. Target is the ID of the node whose context menu it came
// from ("" otherwise).
type MenuEvent struct {
	ID     string
	Target string
}

func (MenuEvent) isEvent() {}

// MenuShortcut finds the item whose Keys contain key, searching menus and
// their submenus in order; ok is false when none does. Hosts route
// shortcuts with it; it is exported for apps that list their shortcuts.
func MenuShortcut(menus []Menu, key KeyEvent) (MenuItem, bool) {
	for _, m := range menus {
		if it, ok := itemShortcut(m.Items, key); ok {
			return it, true
		}
	}
	return MenuItem{}, false
}

func itemShortcut(items []MenuItem, key KeyEvent) (MenuItem, bool) {
	for _, it := range items {
		if it.Separator {
			continue
		}
		if len(it.Items) > 0 {
			if sub, ok := itemShortcut(it.Items, key); ok {
				return sub, true
			}
			continue
		}
		for _, k := range it.Keys {
			if sameKey(k, key) {
				return it, true
			}
		}
	}
	return MenuItem{}, false
}

// sameKey compares the key, rune (letters in either case), and modifiers.
func sameKey(a, b KeyEvent) bool {
	return a.Key == b.Key && unicode.ToLower(a.Rune) == unicode.ToLower(b.Rune) && a.Mod == b.Mod
}

// MenuHost is the menu presentation of a host without native menus — the
// terminal runtime and flatest: it routes shortcuts, and it draws and drives
// context menus itself, so apps handle menus the same way in every host. It
// is host plumbing, not application API. The zero value is ready.
type MenuHost struct {
	// levels are the open context menu and its open submenus, outermost
	// first; target is the node it was opened on.
	levels []menuLevel
	target string
}

type menuLevel struct {
	items []MenuItem
	// x, y is the top-left cell of its box; hi the highlighted item (-1:
	// none).
	x, y, w, h, hi int
}

// RouteMenus is m.Route with app's menus for its current state.
func RouteMenus[S any](m *MenuHost, app App[S], ev Event, width, height int) (Event, bool) {
	var menus []Menu
	if app.Menus != nil {
		menus = app.Menus(app.State)
	}
	var context func(string) []MenuItem
	if app.ContextMenu != nil {
		context = func(target string) []MenuItem { return app.ContextMenu(app.State, target) }
	}
	return m.Route(ev, menus, context, width, height)
}

// IsOpen reports whether a context menu is showing.
func (m *MenuHost) IsOpen() bool { return len(m.levels) > 0 }

// Close closes any open context menu.
func (m *MenuHost) Close() { m.levels, m.target = nil, "" }

// Route takes an event before the app does, for a frame of width×height
// cells (mouse events already hit-tested), with the app's menu bar and
// context menu (either may be nil).
// It returns the event to deliver — the event itself, or a MenuEvent — and
// false when nothing is to be delivered (the menu consumed it).
//
// While a context menu is open it takes the keyboard and mouse: ↑/↓ and
// pointer motion highlight, Enter, → or a click chooses or opens a
// submenu, ← and Esc close a level, and a click outside closes it. A
// resize closes it; other events pass. When none is open, a right press
// opens contextMenu(target) at the pointer (if it has items), and a key
// matching a menus shortcut becomes its MenuEvent.
func (m *MenuHost) Route(ev Event, menus []Menu, contextMenu func(target string) []MenuItem, width, height int) (Event, bool) {
	if m.IsOpen() {
		return m.routeOpen(ev, width, height)
	}
	switch e := ev.(type) {
	case KeyEvent:
		if it, ok := MenuShortcut(menus, e); ok {
			if it.Disabled {
				return nil, false
			}
			return MenuEvent{ID: it.ID}, true
		}
	case MouseEvent:
		if e.Button == MouseRight && e.Action == MousePress && contextMenu != nil {
			if items := contextMenu(e.Target); len(items) > 0 {
				m.target = e.Target
				m.open(items, e.X, e.Y, width, height, nil)
				return nil, false
			}
		}
	}
	return ev, true
}

func (m *MenuHost) routeOpen(ev Event, width, height int) (Event, bool) {
	top := &m.levels[len(m.levels)-1]
	switch e := ev.(type) {
	case KeyEvent:
		switch e.Key {
		case KeyUp:
			top.hi = nextItem(top.items, top.hi, -1)
		case KeyDown:
			top.hi = nextItem(top.items, top.hi, 1)
		case KeyEnter, KeyRight:
			if top.hi >= 0 {
				return m.choose(len(m.levels)-1, top.hi, e.Key == KeyRight, width, height)
			}
		case KeyLeft:
			if len(m.levels) > 1 {
				m.levels = m.levels[:len(m.levels)-1]
			}
		case KeyEscape, KeyCtrlC:
			m.levels = m.levels[:len(m.levels)-1]
			if len(m.levels) == 0 {
				m.Close()
			}
		}
		return nil, false
	case MouseEvent:
		li, row := m.at(e.X, e.Y)
		switch {
		case li < 0:
			if e.Action == MousePress {
				m.Close()
			}
		case e.Action == MouseMotion:
			if row >= 0 && selectable(m.levels[li].items[row]) {
				m.levels = m.levels[:li+1]
				m.levels[li].hi = row
			}
		case e.Action == MousePress && (e.Button == MouseLeft || e.Button == MouseRight):
			if row >= 0 {
				return m.choose(li, row, false, width, height)
			}
		}
		return nil, false
	case ResizeEvent:
		m.Close()
	}
	return ev, true
}

// choose acts on item row of level li: a submenu opens beside it; an
// enabled command closes the menu and becomes a MenuEvent. onlyOpen (→)
// does nothing on a command.
func (m *MenuHost) choose(li, row int, onlyOpen bool, width, height int) (Event, bool) {
	l := m.levels[li]
	it := l.items[row]
	if !selectable(it) {
		return nil, false
	}
	m.levels = m.levels[:li+1]
	m.levels[li].hi = row
	if len(it.Items) > 0 {
		m.open(it.Items, l.x+l.w-1, l.y+1+row, width, height, &l)
		return nil, false
	}
	if onlyOpen {
		return nil, false
	}
	target := m.target
	m.Close()
	return MenuEvent{ID: it.ID, Target: target}, true
}

// open shows items with its top-left at (x, y), kept inside width×height:
// a context menu shifts left and up to fit; a submenu (beside parent)
// flips to the parent's left side when it does not fit on its right.
func (m *MenuHost) open(items []MenuItem, x, y, width, height int, parent *menuLevel) {
	w, h := menuSize(items)
	if x+w > width {
		if parent != nil {
			x = parent.x - w + 1
		} else {
			x = width - w
		}
	}
	if y+h > height {
		y = height - h
	}
	m.levels = append(m.levels, menuLevel{items: items, x: max(x, 0), y: max(y, 0), w: w, h: h, hi: nextItem(items, -1, 1)})
}

// at is the level (topmost first) and item row under (x, y); level -1 is
// outside every level, row -1 its border.
func (m *MenuHost) at(x, y int) (level, row int) {
	for i := len(m.levels) - 1; i >= 0; i-- {
		l := m.levels[i]
		if x >= l.x && x < l.x+l.w && y >= l.y && y < l.y+l.h {
			r := y - l.y - 1
			if r < 0 || r >= len(l.items) || x == l.x || x == l.x+l.w-1 {
				r = -1
			}
			return i, r
		}
	}
	return -1, -1
}

func selectable(it MenuItem) bool { return !it.Separator && !it.Disabled }

// nextItem is the next selectable item from i in direction dir, wrapping;
// i itself (or -1) when there is none.
func nextItem(items []MenuItem, i, dir int) int {
	n := len(items)
	for step := 1; step <= n; step++ {
		j := ((i+dir*step)%n + n) % n
		if selectable(items[j]) {
			return j
		}
	}
	return i
}

// Menu cells: a check column, the label, two spaces, the shortcut, and a
// submenu mark, inside a one-cell pad and the border.
func menuColumns(items []MenuItem) (check, label, keys, sub int) {
	for _, it := range items {
		if it.Separator {
			continue
		}
		if it.Checked {
			check = 2
		}
		label = max(label, ansi.StringWidth(it.Label))
		keys = max(keys, ansi.StringWidth(keyLabel(it.Keys)))
		if len(it.Items) > 0 {
			sub = 2
		}
	}
	if keys > 0 {
		keys += 2
	}
	return
}

func menuSize(items []MenuItem) (w, h int) {
	check, label, keys, sub := menuColumns(items)
	return check + label + keys + sub + 4, len(items) + 2
}

var (
	menuBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("244"))
	menuDisabled = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	menuHi       = lipgloss.NewStyle().Reverse(true)
)

func (l menuLevel) render() string {
	check, label, keys, sub := menuColumns(l.items)
	inner := l.w - 2
	rows := make([]string, len(l.items))
	for i, it := range l.items {
		if it.Separator {
			rows[i] = menuDisabled.Render(strings.Repeat("─", inner))
			continue
		}
		mark := ""
		if check > 0 {
			mark = "  "
			if it.Checked {
				mark = "✓ "
			}
		}
		k := keyLabel(it.Keys)
		row := " " + mark + it.Label + strings.Repeat(" ", label-ansi.StringWidth(it.Label))
		if keys > 0 {
			row += strings.Repeat(" ", keys-ansi.StringWidth(k)) + k
		}
		if sub > 0 {
			if len(it.Items) > 0 {
				row += " ›"
			} else {
				row += "  "
			}
		}
		row += " "
		switch {
		case i == l.hi:
			rows[i] = menuHi.Render(row)
		case it.Disabled:
			rows[i] = menuDisabled.Render(row)
		default:
			rows[i] = row
		}
	}
	return menuBorder.Render(strings.Join(rows, "\n"))
}

// Paint draws the open context menu over frame.Content, hiding the cursor
// while it shows.
func (m *MenuHost) Paint(frame Frame) Frame {
	if m.IsOpen() {
		frame.Cursor = nil
	}
	for _, l := range m.levels {
		frame.Content = layout.OverlayAt(frame.Content, l.render(), l.x, l.y)
	}
	return frame
}

// keyLabel is how a terminal spells an item's shortcut: the first of keys
// a terminal can send (not a ⌘ chord), else the first.
func keyLabel(keys []KeyEvent) string {
	if len(keys) == 0 {
		return ""
	}
	k := keys[0]
	for _, c := range keys {
		if !c.Mod.Contains(ModSuper) {
			k = c
			break
		}
	}
	var b strings.Builder
	for _, m := range []struct {
		mod  Mod
		name string
	}{{ModCtrl, "Ctrl+"}, {ModAlt, "Alt+"}, {ModShift, "Shift+"}, {ModSuper, "Cmd+"}} {
		if k.Mod.Contains(m.mod) {
			b.WriteString(m.name)
		}
	}
	b.WriteString(keyName(k))
	return b.String()
}

var keyNames = map[Key]string{
	KeyUp: "↑", KeyDown: "↓", KeyLeft: "←", KeyRight: "→", KeyEnter: "Enter",
	KeyCtrlC: "Ctrl+C", KeyBackspace: "Backspace", KeyTab: "Tab", KeyEscape: "Esc",
	KeyDelete: "Del", KeyHome: "Home", KeyEnd: "End", KeyPageUp: "PgUp", KeyPageDown: "PgDn",
}

func keyName(k KeyEvent) string {
	switch {
	case k.Key == KeyCharacter:
		if k.Rune == ' ' {
			return "Space"
		}
		return string(unicode.ToUpper(k.Rune))
	case k.Key >= KeyF1 && k.Key <= KeyF12:
		return "F" + strconv.Itoa(int(k.Key-KeyF1)+1)
	}
	return keyNames[k.Key]
}
