package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type screen int

const (
	screenHome screen = iota
	screenDetails
	screenSettings
)

type State struct {
	screen       screen
	homeCursor   int
	selected     int
	settingsName flatui.TextField
}

var homeItems = []string{"Open details", "Open selected details", "Settings"}
var details = []string{"Haiku", "Sonnet", "Opus"}

func NewState() *State {
	return &State{}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch s.screen {
	case screenHome:
		handleHome(s, key, fx)
	case screenDetails:
		handleDetails(s, key)
	case screenSettings:
		handleSettings(s, key)
	}
}

func handleHome(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		homeCursorDown(s)
	case flatte.KeyUp:
		homeCursorUp(s)
	case flatte.KeyEnter:
		switch s.homeCursor {
		case 0:
			s.selected = s.homeCursor
			s.screen = screenDetails
		case 1:
			s.selected = s.homeCursor
			if s.selected >= len(details) {
				s.selected = len(details) - 1
			}
			s.screen = screenDetails
		case 2:
			s.screen = screenSettings
			s.settingsName.SetCursor(len(s.settingsName.Value))
		}
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j', 'J':
			homeCursorDown(s)
		case 'k', 'K':
			homeCursorUp(s)
		case 'q', 'Q':
			fx.Quit()
		}
	}
}

func homeCursorDown(s *State) {
	if s.homeCursor < len(homeItems)-1 {
		s.homeCursor++
	}
}

func homeCursorUp(s *State) {
	if s.homeCursor > 0 {
		s.homeCursor--
	}
}

func handleDetails(s *State, key flatte.KeyEvent) {
	switch key.Key {
	case flatte.KeyDown:
		detailsCursorDown(s)
	case flatte.KeyUp:
		detailsCursorUp(s)
	case flatte.KeyEscape:
		s.screen = screenHome
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j', 'J':
			detailsCursorDown(s)
		case 'k', 'K':
			detailsCursorUp(s)
		}
	}
}

func detailsCursorDown(s *State) {
	if s.selected < len(details)-1 {
		s.selected++
	}
}

func detailsCursorUp(s *State) {
	if s.selected > 0 {
		s.selected--
	}
}

func handleSettings(s *State, key flatte.KeyEvent) {
	switch key.Key {
	case flatte.KeyCharacter:
		s.settingsName.Insert(key.Rune)
	case flatte.KeyBackspace:
		s.settingsName.Backspace()
	case flatte.KeyDelete:
		s.settingsName.Delete()
	case flatte.KeyLeft:
		s.settingsName.MoveLeft()
	case flatte.KeyRight:
		s.settingsName.MoveRight()
	case flatte.KeyEscape, flatte.KeyEnter:
		s.screen = screenHome
	}
}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// View shows the current screen. On the settings screen the name Input
// places the cursor, so there is no cursor arithmetic here.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	return flatte.Frame{
		Root:  flatui.CardLayout(screenContent(s)...),
		Title: "Flatte \u2014 " + screenName(s.screen),
	}
}

func screenName(sc screen) string {
	switch sc {
	case screenDetails:
		return "details"
	case screenSettings:
		return "settings"
	default:
		return "home"
	}
}

func screenContent(s *State) []layout.Node {
	switch s.screen {
	case screenHome:
		return viewHome(s)
	case screenDetails:
		return viewDetails(s)
	case screenSettings:
		return viewSettings(s)
	default:
		return []layout.Node{layout.Label{Text: "unknown screen"}}
	}
}

func viewHome(s *State) []layout.Node {
	children := []layout.Node{
		layout.Label{Text: "Flat Pages", Style: title},
		layout.Label{Text: "multi-screen navigation sample", Style: subtle},
		layout.Label{},
	}
	for i, item := range homeItems {
		prefix := "  "
		if i == s.homeCursor {
			prefix = "> "
		}
		children = append(children, layout.Label{Text: prefix + item})
	}
	return append(children, layout.Label{}, layout.Label{Text: "j/k move | enter open | q quit", Style: subtle})
}

func viewDetails(s *State) []layout.Node {
	item := details[s.selected]
	return []layout.Node{
		layout.Label{Text: "Details", Style: title},
		layout.Label{Text: "screen-specific state without a router", Style: subtle},
		layout.Label{},
		layout.Label{NodeBase: indent, Text: "item: " + item + "\nindex: " + fmt.Sprint(s.selected)},
		layout.Label{},
		layout.Label{Text: "j/k change item | esc back", Style: subtle},
	}
}

func viewSettings(s *State) []layout.Node {
	return []layout.Node{
		layout.Label{Text: "Settings", Style: title},
		layout.Label{Text: "settings input is app-owned state", Style: subtle},
		layout.Label{},
		layout.Row{NodeBase: indent, Children: []layout.Node{
			layout.Label{Text: "name: "},
			s.settingsName.Layout("settings.name", true),
		}},
		layout.Label{},
		layout.Label{Text: "type edit | arrows move | enter/esc back", Style: subtle},
	}
}

func main() {
	state := NewState()
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
