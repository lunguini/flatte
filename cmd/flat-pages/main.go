package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
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

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
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

func handleHome(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	switch key.Key {
	case flatcore.KeyDown:
		homeCursorDown(s)
	case flatcore.KeyUp:
		homeCursorUp(s)
	case flatcore.KeyEnter:
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
	case flatcore.KeyCharacter:
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

func handleDetails(s *State, key flatcore.KeyEvent) {
	switch key.Key {
	case flatcore.KeyDown:
		detailsCursorDown(s)
	case flatcore.KeyUp:
		detailsCursorUp(s)
	case flatcore.KeyEscape:
		s.screen = screenHome
	case flatcore.KeyCharacter:
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

func handleSettings(s *State, key flatcore.KeyEvent) {
	switch key.Key {
	case flatcore.KeyCharacter:
		s.settingsName.Insert(key.Rune)
	case flatcore.KeyBackspace:
		s.settingsName.Backspace()
	case flatcore.KeyDelete:
		s.settingsName.Delete()
	case flatcore.KeyLeft:
		s.settingsName.MoveLeft()
	case flatcore.KeyRight:
		s.settingsName.MoveRight()
	case flatcore.KeyEscape, flatcore.KeyEnter:
		s.screen = screenHome
	}
}

func View(s *State, ctx flatcore.RenderContext) string {
	switch s.screen {
	case screenHome:
		return viewHome(s, ctx)
	case screenDetails:
		return viewDetails(s, ctx)
	case screenSettings:
		return viewSettings(s, ctx)
	default:
		return flatui.Card([]string{"unknown screen"}, ctx.Width)
	}
}

func viewHome(s *State, ctx flatcore.RenderContext) string {
	lines := []string{
		flatui.Title("Flat Pages"),
		flatui.Subtle("multi-screen navigation sample"),
		"",
	}
	for i, item := range homeItems {
		prefix := "  "
		if i == s.homeCursor {
			prefix = "> "
		}
		lines = append(lines, prefix+item)
	}
	lines = append(lines, "", flatui.Subtle("j/k move | enter open | q quit"))
	return flatui.Card(lines, ctx.Width)
}

func viewDetails(s *State, ctx flatcore.RenderContext) string {
	item := details[s.selected]
	lines := []string{
		flatui.Title("Details"),
		flatui.Subtle("screen-specific state without a router"),
		"",
		"  item: " + item,
		"  index: " + fmt.Sprint(s.selected),
		"",
		flatui.Subtle("j/k change item | esc back"),
	}
	return flatui.Card(lines, ctx.Width)
}

func viewSettings(s *State, ctx flatcore.RenderContext) string {
	lines := []string{
		flatui.Title("Settings"),
		flatui.Subtle("settings input is app-owned state"),
		"",
		"  name: " + s.settingsName.Render(true),
		"",
		flatui.Subtle("type edit | arrows move | enter/esc back"),
	}
	return flatui.Card(lines, ctx.Width)
}

func main() {
	state := NewState()
	err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
