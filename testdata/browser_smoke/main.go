package main

import (
	"context"
	"fmt"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	flattehost "github.com/lunguini/flatte/host"
)

type state struct {
	width     int
	layout    bool
	focused   bool
	commands  string
	chosen    string
	paste     string
	clicks    int
	wheels    int
	clipboard string
	media     bool
}

func main() {
	media := make(chan flatte.MediaEvent, 1)
	media <- flatte.MediaEvent{Command: flatte.MediaNext}
	app := flatte.App[state]{
		State: new(state),
		Init: func(_ *state, fx flatte.Effects[state]) {
			fx.SetClipboard("browser clipboard")
			fx.ReadClipboard()
			fx.Notify(flatte.Notification{Title: "Browser smoke", Body: "DOM host"})
		},
		Menus: func(*state) []flatte.Menu {
			return []flatte.Menu{{Items: []flatte.MenuItem{
				{ID: "command-save", Keys: []flatte.KeyEvent{{Key: flatte.KeyCharacter, Rune: 's', Mod: flatte.ModSuper}}},
				{ID: "control-save", Keys: []flatte.KeyEvent{{Key: flatte.KeyCharacter, Rune: 's', Mod: flatte.ModCtrl}}},
			}}}
		},
		ContextMenu: func(_ *state, target string) []flatte.MenuItem {
			if target == "smoke" {
				return []flatte.MenuItem{{ID: "open", Label: "Open"}}
			}
			return nil
		},
		Handle: func(s *state, ev flatte.Event, fx flatte.Effects[state]) {
			switch ev := ev.(type) {
			case flatte.ResizeEvent:
				s.width = ev.Width
			case flatte.LayoutEvent:
				s.layout = ev.Rects["smoke"].W > 0
			case flatte.FocusEvent:
				s.focused = ev.Focused
			case flatte.MenuEvent:
				if ev.ID == "open" {
					s.chosen = ev.ID + ":" + ev.Target
				} else {
					if s.commands != "" {
						s.commands += ","
					}
					s.commands += ev.ID
				}
			case flatte.PasteEvent:
				s.paste = ev.Text
			case flatte.ClipboardEvent:
				s.clipboard = ev.Text
			case flatte.MediaEvent:
				s.media = ev.Command == flatte.MediaNext
			case flatte.MouseEvent:
				if ev.Target != "smoke" {
					break
				}
				switch ev.Button {
				case flatte.MouseLeft:
					if ev.Action == flatte.MousePress {
						s.clicks++
					}
				case flatte.MouseWheelDown, flatte.MouseWheelUp:
					s.wheels++
				}
			case flatte.KeyEvent:
				if ev.Key == flatte.KeyEscape {
					fx.Quit()
				}
			}
		},
		View: func(s *state, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Col{Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "smoke"}, Text: "browser smoke"},
				layout.Label{Text: fmt.Sprintf("width=%d layout=%t focus=%t graphical=%t", s.width, s.layout, s.focused, ctx.Graphical)},
				layout.Label{Text: "commands=" + s.commands},
				layout.Label{Text: "chosen=" + s.chosen},
				layout.Label{Text: fmt.Sprintf("paste=%s clicks=%d", s.paste, s.clicks)},
				layout.Label{Text: fmt.Sprintf("wheels=%d clipboard=%s media=%t", s.wheels, s.clipboard, s.media)},
			}}}
		},
	}
	if err := flatte.Run(context.Background(), app,
		flatte.WithMouse(flatte.MouseModeCellMotion),
		flattehost.WithHostMedia(media, func(*flatte.NowPlaying) {}),
	); err != nil {
		panic(err)
	}
}
