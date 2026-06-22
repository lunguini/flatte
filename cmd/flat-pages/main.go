package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
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

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	frame := flatte.Frame{
		Content: viewContent(s, ctx),
		Title:   "Flatte \u2014 " + screenName(s.screen),
	}
	if s.screen == screenSettings {
		originX, originY := flatui.CardOrigin()
		frame.Cursor = &flatte.Cursor{
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

func viewContent(s *State, ctx flatte.RenderContext) string {
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

func viewHome(s *State, ctx flatte.RenderContext) string {
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

func viewDetails(s *State, ctx flatte.RenderContext) string {
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

func viewSettings(s *State, ctx flatte.RenderContext) string {
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
