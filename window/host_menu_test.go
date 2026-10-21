package window

import (
	"context"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// fakeMenus records the bars it shows and answers pop-ups with choose.
type fakeMenus struct {
	bars   [][]flatte.Menu
	popups [][]flatte.MenuItem
	at     [2]float64
	choose string
}

func (f *fakeMenus) setBar(m []flatte.Menu) { f.bars = append(f.bars, m) }
func (f *fakeMenus) popUp(items []flatte.MenuItem, x, y float64) string {
	f.popups = append(f.popups, items)
	f.at = [2]float64{x, y}
	return f.choose
}

func key(r rune, mod flatte.Mod) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: mod}
}

// menuHost is a host whose app has a File menu (its Save label is
// s.content, so tests can change the bar) and a context menu on "doc".
func menuHost(t *testing.T) (*host[testState], *testState, *fakeMenus) {
	s := &testState{content: "Save", root: func(s *testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{ID: "doc"}, Text: "document", Selectable: true},
			layout.Label{NodeBase: layout.NodeBase{ID: "plain"}, Text: "plain"},
		}}
	}}
	app := testApp(s)
	app.Menus = func(s *testState) []flatte.Menu {
		return []flatte.Menu{{Title: "File", Items: []flatte.MenuItem{
			{ID: "save", Label: s.content, Keys: []flatte.KeyEvent{key('s', flatte.ModSuper), key('s', flatte.ModCtrl)}},
			{ID: "print", Label: "Print", Keys: []flatte.KeyEvent{key('p', flatte.ModCtrl)}, Disabled: true},
			{ID: "copy", Label: "Copy", Keys: []flatte.KeyEvent{key('c', flatte.ModSuper)}},
		}}}
	}
	app.ContextMenu = func(_ *testState, target string) []flatte.MenuItem {
		if target != "doc" {
			return nil
		}
		return []flatte.MenuItem{{ID: "cut", Label: "Cut"}}
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() {}, func() {})
	t.Cleanup(h.cancel)
	m := &fakeMenus{}
	h.menus, h.text, h.clip = m, &fakeText{}, &fakeClip{}
	h.start(400, 200)
	h.render()
	s.events = nil
	return h, s, m
}

func TestHostMenuBarFollowsState(t *testing.T) {
	h, s, m := menuHost(t)
	if len(m.bars) != 1 || m.bars[0][0].Items[0].Label != "Save" {
		t.Fatalf("bars %+v", m.bars)
	}
	h.render()
	if len(m.bars) != 1 {
		t.Error("an unchanged bar was set again")
	}
	s.content = "Save As"
	h.render()
	if len(m.bars) != 2 || m.bars[1][0].Items[0].Label != "Save As" {
		t.Errorf("changed bar not shown: %+v", m.bars)
	}
	h.menuChosen("save")
	if len(s.events) != 1 || s.events[0] != (flatte.MenuEvent{ID: "save"}) {
		t.Errorf("bar choice delivered %#v", s.events)
	}
}

func TestHostRoutesMenuShortcuts(t *testing.T) {
	h, s, _ := menuHost(t)
	h.key(key('s', flatte.ModCtrl))
	h.key(key('p', flatte.ModCtrl)) // disabled: swallowed
	h.key(key('x', flatte.ModCtrl)) // no item: the app's
	want := []flatte.Event{flatte.MenuEvent{ID: "save"}, key('x', flatte.ModCtrl)}
	if len(s.events) != 2 || s.events[0] != want[0] || s.events[1] != want[1] {
		t.Errorf("delivered %#v, want %#v", s.events, want)
	}
}

func TestHostContextMenu(t *testing.T) {
	h, s, m := menuHost(t)
	doc := h.scene.Rects["doc"]
	m.choose = "cut"
	h.mouse(doc.X+2, doc.Y+1, flatte.MouseRight, flatte.MousePress, 0)
	if len(m.popups) != 1 || m.at != [2]float64{doc.X + 2, doc.Y + 1} {
		t.Fatalf("popups %+v at %v", m.popups, m.at)
	}
	if len(s.events) != 1 || s.events[0] != (flatte.MenuEvent{ID: "cut", Target: "doc"}) {
		t.Fatalf("delivered %#v", s.events)
	}
	// Dismissed: nothing is delivered, not even the click.
	s.events, m.choose = nil, ""
	h.mouse(doc.X+2, doc.Y+1, flatte.MouseRight, flatte.MousePress, 0)
	if len(s.events) != 0 {
		t.Errorf("a dismissed menu delivered %#v", s.events)
	}
	// Control-click is a right-click.
	h.mouse(doc.X+2, doc.Y+1, flatte.MouseLeft, flatte.MousePress, flatte.ModCtrl)
	if len(m.popups) != 3 || len(s.events) != 0 {
		t.Errorf("control-click: %d popups, events %#v", len(m.popups), s.events)
	}
	// No menu for the target: the click is the app's.
	p := h.scene.Rects["plain"]
	h.mouse(p.X+1, p.Y+1, flatte.MouseRight, flatte.MousePress, 0)
	if len(m.popups) != 3 || len(s.events) != 1 {
		t.Errorf("no-menu click: %d popups, events %#v", len(m.popups), s.events)
	}
}

func TestHostMenuCopyCopiesSelection(t *testing.T) {
	h, s, _ := menuHost(t)
	clip := h.clip.(*fakeClip)
	// No selection: Copy is the app's.
	h.menuChosen("copy")
	if len(s.events) != 1 || s.events[0] != (flatte.MenuEvent{ID: "copy"}) {
		t.Fatalf("copy without a selection: %#v", s.events)
	}
	doc := h.scene.Rects["doc"]
	h.mouse(doc.X, doc.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	h.mouse(doc.X+4, doc.Y+1, flatte.MouseLeft, flatte.MouseMotion, 0)
	h.mouse(doc.X+4, doc.Y+1, flatte.MouseLeft, flatte.MouseRelease, 0)
	s.events = nil
	h.menuChosen("copy")
	if clip.text != "docu" || len(s.events) != 0 {
		t.Errorf("copy with a selection: clipboard %q, events %#v", clip.text, s.events)
	}
	// So does ⌘C reaching the view.
	clip.text = ""
	h.key(copyKey)
	if clip.text != "docu" || len(s.events) != 0 {
		t.Errorf("⌘C with a selection: clipboard %q, events %#v", clip.text, s.events)
	}
}
