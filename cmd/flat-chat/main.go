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

	"github.com/charmbracelet/lipgloss"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

const prompt = "› "

type State struct {
	input flatui.TextField
	sent  int
}

func NewState() *State { return &State{} }

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatcore.KeyEscape:
		fx.Quit()
	case flatcore.KeyEnter:
		if s.input.Value != "" {
			fx.Printf("you: %s", s.input.Value) // into scrollback, above the frame
			s.input = flatui.TextField{}
			s.sent++
		}
	case flatcore.KeyBackspace:
		s.input.Backspace()
	case flatcore.KeyDelete:
		s.input.Delete()
	case flatcore.KeyLeft:
		if wordMove(key.Mod) {
			s.input.MoveWordLeft()
		} else {
			s.input.MoveLeft()
		}
	case flatcore.KeyRight:
		if wordMove(key.Mod) {
			s.input.MoveWordRight()
		} else {
			s.input.MoveRight()
		}
	case flatcore.KeyCharacter:
		if handleAltWordKey(key, s.input.MoveWordLeft, s.input.MoveWordRight) {
			return
		}
		s.input.Insert(key.Rune)
	}
}

func wordMove(mod flatcore.Mod) bool {
	return mod.Contains(flatcore.ModAlt) || mod.Contains(flatcore.ModCtrl)
}

func handleAltWordKey(key flatcore.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatcore.ModAlt) {
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

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	lines := []string{
		flatui.Subtle(fmt.Sprintf("flat-chat — %d sent | enter send · esc quit", s.sent)),
		prompt + s.input.Value,
	}
	frame := flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
	ox, oy := flatui.CardOrigin()
	frame.Cursor = &flatcore.Cursor{
		X: ox + lipgloss.Width(prompt) + s.input.CursorColumn(),
		Y: oy + 1, // the subtle status line precedes the input line
	}
	return frame
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatcore.WithInline()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
