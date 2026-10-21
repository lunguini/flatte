package flatest

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type menuState struct{ events []flatte.Event }

func menuApp() flatte.App[menuState] {
	return flatte.App[menuState]{
		State: &menuState{},
		Handle: func(s *menuState, ev flatte.Event, _ flatte.Effects[menuState]) {
			if _, ok := ev.(flatte.LayoutEvent); !ok {
				s.events = append(s.events, ev)
			}
		},
		View: func(*menuState, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Col{Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "doc", H: layout.Grow(1)}, Text: "document"},
			}}}
		},
		Menus: func(*menuState) []flatte.Menu {
			return []flatte.Menu{{Title: "File", Items: []flatte.MenuItem{
				{ID: "save", Label: "Save", Keys: []flatte.KeyEvent{{Key: flatte.KeyCharacter, Rune: 's', Mod: flatte.ModCtrl}}},
			}}}
		},
		ContextMenu: func(_ *menuState, target string) []flatte.MenuItem {
			if target != "doc" {
				return nil
			}
			return []flatte.MenuItem{{ID: "copy", Label: "Copy"}, {ID: "paste", Label: "Paste"}}
		},
	}
}

func TestDriverRoutesMenus(t *testing.T) {
	d := Start(menuApp(), 30)
	d.Send(flatte.ResizeEvent{Width: 30, Height: 8})
	s := d.State()
	s.events = nil
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 's', Mod: flatte.ModCtrl})
	if len(s.events) != 1 || s.events[0] != (flatte.MenuEvent{ID: "save"}) {
		t.Fatalf("shortcut delivered %#v", s.events)
	}
	s.events = nil
	f := d.Send(flatte.MouseEvent{X: 4, Y: 2, Button: flatte.MouseRight, Action: flatte.MousePress})
	if len(s.events) != 0 {
		t.Fatalf("the right-click reached the app: %#v", s.events)
	}
	if !strings.Contains(CleanFrame(f.Content), "│ Copy  │") || !strings.Contains(CleanFrame(d.Frame().Content), "│ Paste │") {
		t.Fatalf("menu not painted:\n%s", CleanFrame(f.Content))
	}
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown})
	d.Send(flatte.KeyEvent{Key: flatte.KeyEnter})
	if len(s.events) != 1 || s.events[0] != (flatte.MenuEvent{ID: "paste", Target: "doc"}) {
		t.Fatalf("choice delivered %#v", s.events)
	}
	if strings.Contains(CleanFrame(d.Frame().Content), "Paste") {
		t.Error("menu still painted after choosing")
	}
}
