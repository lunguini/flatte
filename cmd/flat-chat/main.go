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

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

const prompt = "› "

type State struct {
	input flatui.TextField
	sent  int
}

func NewState() *State { return &State{} }

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flat.KeyEscape:
		fx.Quit()
	case flat.KeyEnter:
		if s.input.Value != "" {
			fx.Printf("you: %s", s.input.Value) // into scrollback, above the frame
			s.input = flatui.TextField{}
			s.sent++
		}
	case flat.KeyBackspace:
		if wordMove(key.Mod) {
			s.input.DeleteWordLeft()
		} else {
			s.input.Backspace()
		}
	case flat.KeyDelete:
		if wordMove(key.Mod) {
			s.input.DeleteWordRight()
		} else {
			s.input.Delete()
		}
	case flat.KeyLeft:
		if wordMove(key.Mod) {
			s.input.MoveWordLeft()
		} else {
			s.input.MoveLeft()
		}
	case flat.KeyRight:
		if wordMove(key.Mod) {
			s.input.MoveWordRight()
		} else {
			s.input.MoveRight()
		}
	case flat.KeyCharacter:
		if handleWordDeleteKey(key, s.input.DeleteWordLeft, s.input.DeleteWordRight) {
			return
		}
		if handleAltWordKey(key, s.input.MoveWordLeft, s.input.MoveWordRight) {
			return
		}
		s.input.Insert(key.Rune)
	}
}

func wordMove(mod flat.Mod) bool {
	return mod.Contains(flat.ModAlt) || mod.Contains(flat.ModCtrl)
}

func handleAltWordKey(key flat.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flat.ModAlt) {
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

func handleWordDeleteKey(key flat.KeyEvent, deleteLeft, deleteRight func()) bool {
	if key.Mod.Contains(flat.ModCtrl) && (key.Rune == 'w' || key.Rune == 'W' || key.Rune == 'h' || key.Rune == 'H') {
		deleteLeft()
		return true
	}
	if key.Mod.Contains(flat.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
		deleteRight()
		return true
	}
	return false
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	lines := []string{
		flatui.Subtle(fmt.Sprintf("flat-chat — %d sent | enter send · esc quit", s.sent)),
		prompt + s.input.Value,
	}
	frame := flat.Frame{Content: flatui.Card(lines, ctx.Width)}
	ox, oy := flatui.CardOrigin()
	frame.Cursor = &flat.Cursor{
		X: ox + lipgloss.Width(prompt) + s.input.CursorColumn(),
		Y: oy + 1, // the subtle status line precedes the input line
	}
	return frame
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flat.WithInline()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
