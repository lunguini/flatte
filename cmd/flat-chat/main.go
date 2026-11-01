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

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

const prompt = "› "

type State struct {
	input flatui.TextField
	sent  int
}

func NewState() *State { return &State{} }

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.PasteEvent:
		s.input.InsertText(ev.Text)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyEscape:
		fx.Quit()
	case flatte.KeyEnter:
		if s.input.Value != "" {
			fx.Printf("you: %s", s.input.Value) // into scrollback, above the frame
			s.input = flatui.TextField{}
			s.sent++
		}
	default:
		// Keep the chat's readline aliases ahead of TextField's native editing.
		if handleWordDeleteKey(key, s.input.DeleteWordLeft, s.input.DeleteWordRight) {
			return
		}
		s.input.Edit(key)
	}
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

// View is the status line over the prompt and input. The input's caret
// places the cursor, so there is no cursor arithmetic here.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: fmt.Sprintf("flat-chat — %d sent | enter send · esc quit", s.sent), Style: layout.TextStyle{Role: layout.TextSubtle}},
		layout.Row{Children: []layout.Node{
			layout.Label{Text: prompt},
			s.input.Layout("chat.input", true),
		}},
	)}
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
