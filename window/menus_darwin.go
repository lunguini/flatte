package window

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
)

// Native menus: the menu bar and context menus are NSMenus whose items
// target one FlatteMenuTarget; an item's tag indexes the IDs of the menu it
// was built for. Main thread only.

var (
	menuTargetOnce sync.Once
	menuTarget     objc.ID
	menuTargetErr  error

	// barIDs are the IDs of the menu bar's items, by tag; barChosen
	// delivers a choice from it.
	barIDs    []string
	barChosen func(id string)
	// popupIDs and popupChosen are the open context menu's.
	popupIDs    []string
	popupChosen string
)

func newMenuTarget() (objc.ID, error) {
	menuTargetOnce.Do(func() {
		cls, err := objc.RegisterClass("FlatteMenuTarget", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("chooseBar:"), Fn: func(_ objc.ID, _ objc.SEL, item objc.ID) {
				if tag := objc.Send[int](item, sel("tag")); tag >= 0 && tag < len(barIDs) && barChosen != nil {
					barChosen(barIDs[tag])
				}
			}},
			{Cmd: sel("choosePopup:"), Fn: func(_ objc.ID, _ objc.SEL, item objc.ID) {
				if tag := objc.Send[int](item, sel("tag")); tag >= 0 && tag < len(popupIDs) {
					popupChosen = popupIDs[tag]
				}
			}},
		})
		if err != nil {
			menuTargetErr = err
			return
		}
		menuTarget = objc.ID(cls).Send(sel("new"))
	})
	return menuTarget, menuTargetErr
}

// nativeMenus is the window's menuBackend.
type nativeMenus struct {
	title string // the app's name, for Quit
}

func (n nativeMenus) setBar(menus []flatte.Menu) {
	withPool(func() {
		bar := mainMenu(n.title, menus)
		class("NSApplication").Send(sel("sharedApplication")).Send(sel("setMainMenu:"), bar)
		bar.Send(sel("release"))
	})
}

func (n nativeMenus) popUp(id string, items []flatte.MenuItem, x, y float64) string {
	var view objc.ID
	for v, st := range states {
		if st.id == id {
			view = v
		}
	}
	if view == 0 {
		return ""
	}
	popupIDs, popupChosen = nil, ""
	withPool(func() {
		m := buildMenu("", items, sel("choosePopup:"), &popupIDs)
		m.Send(sel("popUpMenuPositioningItem:atLocation:inView:"), objc.ID(0), CGPoint{X: x, Y: y}, view)
		m.Send(sel("release"))
	})
	chosen := popupChosen
	popupIDs, popupChosen = nil, ""
	return chosen
}

// mainMenu is the menu bar: the application menu (Quit), the app's menus,
// and the Window menu (Close). The caller releases it.
func mainMenu(title string, menus []flatte.Menu) objc.ID {
	barIDs = nil
	bar := alloc("NSMenu").Send(sel("init"))
	add := func(m objc.ID) {
		item := alloc("NSMenuItem").Send(sel("init"))
		item.Send(sel("setSubmenu:"), m)
		bar.Send(sel("addItem:"), item)
		item.Send(sel("release"))
		m.Send(sel("release"))
	}
	appMenu := alloc("NSMenu").Send(sel("init"))
	quit := alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"), nsString("Quit "+title), sel("terminate:"), nsString("q"))
	appMenu.Send(sel("addItem:"), quit)
	quit.Send(sel("release"))
	add(appMenu)
	for _, m := range menus {
		add(buildMenu(m.Title, m.Items, sel("chooseBar:"), &barIDs))
	}
	// The Window menu: AppKit's own actions, sent to the key window.
	winMenu := alloc("NSMenu").Send(sel("initWithTitle:"), nsString("Window"))
	for _, it := range []struct {
		title, action, key string
		mask               uint
	}{
		{"Minimize", "performMiniaturize:", "m", macCommand},
		{"Zoom", "performZoom:", "", 0},
		{"", "", "", 0},
		{"Enter Full Screen", "toggleFullScreen:", "f", macCommand | macControl},
		{"", "", "", 0},
		{"Close", "performClose:", "w", macCommand},
	} {
		if it.title == "" {
			winMenu.Send(sel("addItem:"), class("NSMenuItem").Send(sel("separatorItem")))
			continue
		}
		item := alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"), nsString(it.title), sel(it.action), nsString(it.key))
		if it.key != "" {
			item.Send(sel("setKeyEquivalentModifierMask:"), it.mask)
		}
		winMenu.Send(sel("addItem:"), item)
		item.Send(sel("release"))
	}
	class("NSApplication").Send(sel("sharedApplication")).Send(sel("setWindowsMenu:"), winMenu)
	add(winMenu)
	return bar
}

// buildMenu makes an NSMenu of items whose commands send action to the
// menu target, tagged with their index in ids. The caller releases it.
func buildMenu(title string, items []flatte.MenuItem, action objc.SEL, ids *[]string) objc.ID {
	target, _ := newMenuTarget()
	m := alloc("NSMenu").Send(sel("initWithTitle:"), nsString(title))
	m.Send(sel("setAutoenablesItems:"), false)
	for _, it := range items {
		if it.Separator {
			m.Send(sel("addItem:"), class("NSMenuItem").Send(sel("separatorItem")))
			continue
		}
		key, mask := keyEquivalent(it.Keys)
		item := alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"), nsString(it.Label), action, nsString(key))
		if key != "" {
			item.Send(sel("setKeyEquivalentModifierMask:"), mask)
		}
		item.Send(sel("setTarget:"), target)
		item.Send(sel("setTag:"), len(*ids))
		*ids = append(*ids, it.ID)
		item.Send(sel("setEnabled:"), !it.Disabled)
		if it.Checked {
			item.Send(sel("setState:"), 1) // NSControlStateValueOn
		}
		if len(it.Items) > 0 {
			sub := buildMenu(it.Label, it.Items, action, ids)
			item.Send(sel("setAction:"), objc.SEL(0))
			item.Send(sel("setSubmenu:"), sub)
			sub.Send(sel("release"))
		}
		m.Send(sel("addItem:"), item)
		item.Send(sel("release"))
	}
	return m
}

// keyEquivalent is the native shortcut for keys: the first ⌘ chord, as
// AppKit's key equivalent and modifier mask; "" when there is none (other
// shortcuts reach the view as keys and the host routes them).
func keyEquivalent(keys []flatte.KeyEvent) (string, uint) {
	for _, k := range keys {
		if !k.Mod.Contains(flatte.ModSuper) {
			continue
		}
		var key string
		switch {
		case k.Key == flatte.KeyCharacter:
			key = string(k.Rune)
		case k.Key >= flatte.KeyF1 && k.Key <= flatte.KeyF12:
			key = string(rune(0xF704 + int(k.Key-flatte.KeyF1)))
		default:
			r, ok := equivalentKeys[k.Key]
			if !ok {
				continue
			}
			key = string(r)
		}
		mask := uint(macCommand)
		for _, m := range []struct {
			mod  flatte.Mod
			flag uint
		}{{flatte.ModShift, macShift}, {flatte.ModCtrl, macControl}, {flatte.ModAlt, macOption}} {
			if k.Mod.Contains(m.mod) {
				mask |= m.flag
			}
		}
		return key, mask
	}
	return "", 0
}

// equivalentKeys are AppKit's characters for non-text keys.
var equivalentKeys = map[flatte.Key]rune{
	flatte.KeyEnter: '\r', flatte.KeyTab: '\t', flatte.KeyEscape: 0x1b, flatte.KeyBackspace: 0x08,
	flatte.KeyDelete: 0xF728, flatte.KeyUp: 0xF700, flatte.KeyDown: 0xF701, flatte.KeyLeft: 0xF702,
	flatte.KeyRight: 0xF703, flatte.KeyHome: 0xF729, flatte.KeyEnd: 0xF72B,
	flatte.KeyPageUp: 0xF72C, flatte.KeyPageDown: 0xF72D,
}
