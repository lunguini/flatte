// flat-chat dogfoods native scrollback: each sent message is fx.Print'd into
// the terminal's real scrollback (which you scroll with the terminal/mouse)
// while the input box stays pinned at the bottom — the Claude-Code model. Run
// with WithInline so the frame lives in normal terminal flow, not the alt
// screen.
package main

import (
	"context"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

const prompt = "› "

type State struct {
	input flatui.TextField
	sent  int
}

func NewState() *State { return &State{} }

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		fx.Quit()
	case flatte.KeyEnter:
		if s.input.Value != "" {
			fx.Printf("you: %s", s.input.Value) // into scrollback, above the frame
			s.input = flatui.TextField{}
			s.sent++
		}
	case flatte.KeyBackspace:
		if wordMove(key.Mod) {
			s.input.DeleteWordLeft()
		} else {
			s.input.Backspace()
		}
	case flatte.KeyDelete:
		if wordMove(key.Mod) {
			s.input.DeleteWordRight()
		} else {
			s.input.Delete()
		}
	case flatte.KeyLeft:
		if wordMove(key.Mod) {
			s.input.MoveWordLeft()
		} else {
			s.input.MoveLeft()
		}
	case flatte.KeyRight:
		if wordMove(key.Mod) {
			s.input.MoveWordRight()
		} else {
			s.input.MoveRight()
		}
	case flatte.KeyCharacter:
		if handleWordDeleteKey(key, s.input.DeleteWordLeft, s.input.DeleteWordRight) {
			return
		}
		if handleAltWordKey(key, s.input.MoveWordLeft, s.input.MoveWordRight) {
			return
		}
		s.input.Insert(key.Rune)
	}
}

func wordMove(mod flatte.Mod) bool {
	return mod.Contains(flatte.ModAlt) || mod.Contains(flatte.ModCtrl)
}

func handleAltWordKey(key flatte.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatte.ModAlt) {
		return false
	}
	switch key.Rune {
	case 'b', 'B':
		moveLeft()
		return true
	case 'f', 'F':
		moveRight()
		return true
	}
	return false
}

func handleWordDeleteKey(key flatte.KeyEvent, deleteLeft, deleteRight func()) bool {
	if key.Mod.Contains(flatte.ModCtrl) && (key.Rune == 'w' || key.Rune == 'W' || key.Rune == 'h' || key.Rune == 'H') {
		deleteLeft()
		return true
	}
	if key.Mod.Contains(flatte.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
		deleteRight()
		return true
	}
	return false
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	lines := []string{
		flatui.Subtle(fmt.Sprintf("flat-chat — %d sent | enter send · esc quit", s.sent)),
		prompt + s.input.Value,
	}
	frame := flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
	ox, oy := flatui.CardOrigin()
	frame.Cursor = &flatte.Cursor{
		X: ox + lipgloss.Width(prompt) + s.input.CursorColumn(),
		Y: oy + 1, // the subtle status line precedes the input line
	}
	return frame
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatte.WithInline()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
