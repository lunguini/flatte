package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
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

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
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

func handleHome(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	switch key.Key {
	case flat.KeyDown:
		homeCursorDown(s)
	case flat.KeyUp:
		homeCursorUp(s)
	case flat.KeyEnter:
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
	case flat.KeyCharacter:
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

func handleDetails(s *State, key flat.KeyEvent) {
	switch key.Key {
	case flat.KeyDown:
		detailsCursorDown(s)
	case flat.KeyUp:
		detailsCursorUp(s)
	case flat.KeyEscape:
		s.screen = screenHome
	case flat.KeyCharacter:
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

func handleSettings(s *State, key flat.KeyEvent) {
	switch key.Key {
	case flat.KeyCharacter:
		s.settingsName.Insert(key.Rune)
	case flat.KeyBackspace:
		s.settingsName.Backspace()
	case flat.KeyDelete:
		s.settingsName.Delete()
	case flat.KeyLeft:
		s.settingsName.MoveLeft()
	case flat.KeyRight:
		s.settingsName.MoveRight()
	case flat.KeyEscape, flat.KeyEnter:
		s.screen = screenHome
	}
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	frame := flat.Frame{
		Content: viewContent(s, ctx),
		Title:   "Flatte \u2014 " + screenName(s.screen),
	}
	if s.screen == screenSettings {
		originX, originY := flatui.CardOrigin()
		frame.Cursor = &flat.Cursor{
			X: originX + lipgloss.Width("  name: ") + s.settingsName.CursorColumn(),
			Y: originY + 3, // title, subtle, blank precede the name row
		}
	}
	return frame
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

func viewContent(s *State, ctx flat.RenderContext) string {
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

func viewHome(s *State, ctx flat.RenderContext) string {
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

func viewDetails(s *State, ctx flat.RenderContext) string {
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

func viewSettings(s *State, ctx flat.RenderContext) string {
	lines := []string{
		flatui.Title("Settings"),
		flatui.Subtle("settings input is app-owned state"),
		"",
		"  name: " + s.settingsName.Value,
		"",
		flatui.Subtle("type edit | arrows move | enter/esc back"),
	}
	return flatui.Card(lines, ctx.Width)
}

func main() {
	state := NewState()
	err := flat.Run(context.Background(), flat.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
