package window

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
)

func TestKeyEquivalent(t *testing.T) {
	for _, c := range []struct {
		keys []flatte.KeyEvent
		key  string
		mask uint
	}{
		{nil, "", 0},
		{[]flatte.KeyEvent{key('s', flatte.ModCtrl)}, "", 0}, // not ⌘: routed by the host
		{[]flatte.KeyEvent{key('s', flatte.ModCtrl), key('s', flatte.ModSuper)}, "s", macCommand},
		{[]flatte.KeyEvent{key('z', flatte.ModSuper|flatte.ModShift)}, "z", macCommand | macShift},
		{[]flatte.KeyEvent{{Key: flatte.KeyF3, Mod: flatte.ModSuper | flatte.ModAlt}}, string(rune(0xF706)), macCommand | macOption},
		{[]flatte.KeyEvent{{Key: flatte.KeyUp, Mod: flatte.ModSuper | flatte.ModCtrl}}, string(rune(0xF700)), macCommand | macControl},
		{[]flatte.KeyEvent{{Key: flatte.KeyUnknown, Mod: flatte.ModSuper}}, "", 0},
	} {
		k, m := keyEquivalent(c.keys)
		if k != c.key || m != c.mask {
			t.Errorf("%+v: %q %x, want %q %x", c.keys, k, m, c.key, c.mask)
		}
	}
}

func TestBuildMenu(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	var ids []string
	var chosen []string
	barChosen = func(id string) { chosen = append(chosen, id) }
	// Item actions go through the shared application.
	withPool(func() { class("NSApplication").Send(sel("sharedApplication")) })
	defer func() { barChosen = nil }()
	withPool(func() {
		m := buildMenu("File", []flatte.MenuItem{
			{ID: "save", Label: "Save", Keys: []flatte.KeyEvent{key('s', flatte.ModSuper)}},
			{Separator: true},
			{ID: "print", Label: "Print", Disabled: true},
			{ID: "wrap", Label: "Wrap", Checked: true},
			{Label: "Export", Items: []flatte.MenuItem{{ID: "pdf", Label: "PDF"}}},
		}, sel("chooseBar:"), &ids)
		defer m.Send(sel("release"))
		barIDs = ids
		item := func(i int) objc.ID { return m.Send(sel("itemAtIndex:"), i) }
		if n := objc.Send[int](m, sel("numberOfItems")); n != 5 {
			t.Fatalf("%d items", n)
		}
		if got := plainString(item(0).Send(sel("title"))); got != "Save" {
			t.Errorf("title %q", got)
		}
		if got := plainString(item(0).Send(sel("keyEquivalent"))); got != "s" {
			t.Errorf("key equivalent %q", got)
		}
		if !objc.Send[bool](item(1), sel("isSeparatorItem")) {
			t.Error("no separator")
		}
		if objc.Send[bool](item(2), sel("isEnabled")) || !objc.Send[bool](item(0), sel("isEnabled")) {
			t.Error("enabled states wrong")
		}
		if objc.Send[int](item(3), sel("state")) != 1 {
			t.Error("not checked")
		}
		sub := item(4).Send(sel("submenu"))
		if sub == 0 || objc.Send[int](sub, sel("numberOfItems")) != 1 {
			t.Fatal("no submenu")
		}
		if len(ids) != 5 || ids[0] != "save" || ids[4] != "pdf" {
			t.Errorf("ids %q", ids)
		}
		// Choosing items reaches the host by ID, through the submenu too.
		m.Send(sel("performActionForItemAtIndex:"), 0)
		sub.Send(sel("performActionForItemAtIndex:"), 0)
	})
	if len(chosen) != 2 || chosen[0] != "save" || chosen[1] != "pdf" {
		t.Errorf("chosen %q", chosen)
	}
}

func TestMainMenuFrame(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	withPool(func() {
		bar := mainMenu("Demo", []flatte.Menu{{Title: "File", Items: []flatte.MenuItem{{ID: "a", Label: "A"}}}, {Title: "View"}})
		defer bar.Send(sel("release"))
		if n := objc.Send[int](bar, sel("numberOfItems")); n != 4 {
			t.Fatalf("%d menus, want app, File, View, Window", n)
		}
		title := func(i int) string {
			return plainString(bar.Send(sel("itemAtIndex:"), i).Send(sel("submenu")).Send(sel("title")))
		}
		if title(1) != "File" || title(2) != "View" || title(3) != "Window" {
			t.Errorf("menus %q %q %q", title(1), title(2), title(3))
		}
		quit := bar.Send(sel("itemAtIndex:"), 0).Send(sel("submenu")).Send(sel("itemAtIndex:"), 0)
		if plainString(quit.Send(sel("title"))) != "Quit Demo" {
			t.Error("no Quit item")
		}
	})
}
