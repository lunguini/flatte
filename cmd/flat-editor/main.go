package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

type State struct {
	ta flatui.Textarea
}

func NewState() *State {
	s := &State{}
	s.ta.SetValue("Edit me.\nMultiple lines.\nGrapheme-correct.")
	return s
}

// layout sizes the textarea to the rows left after the pinned chrome (title,
// subtitle, blank, blank, footer = 5) and the card's top+bottom border (2).
func (s *State) layout(width, height int) {
	const pinnedRows = 5 // title, subtitle, blank, blank, footer
	s.ta.SetSize(
		max(flatui.CardBodyWidth(width), 1),
		max(flatui.CardBodyHeight(height, pinnedRows), 1),
	)
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	switch e := ev.(type) {
	case flatcore.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flatcore.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	switch key.Key {
	case flatcore.KeyEscape:
		fx.Quit()
	case flatcore.KeyEnter:
		s.ta.InsertNewline()
	case flatcore.KeyBackspace:
		s.ta.Backspace()
	case flatcore.KeyDelete:
		s.ta.Delete()
	case flatcore.KeyLeft:
		s.ta.MoveLeft()
	case flatcore.KeyRight:
		s.ta.MoveRight()
	case flatcore.KeyUp:
		s.ta.MoveUp()
	case flatcore.KeyDown:
		s.ta.MoveDown()
	case flatcore.KeyCharacter:
		s.ta.Insert(key.Rune)
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	lines := []string{
		flatui.Title("Flat Editor"),
		flatui.Subtle("multi-line textarea sample"),
		"",
	}
	lines = append(lines, strings.Split(s.ta.View(), "\n")...)
	lines = append(lines, "", flatui.Subtle("arrows move  enter newline  esc quit"))

	frame := flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
	// Place the hardware cursor: card origin + the three pinned lines (title,
	// subtitle, blank) that precede the textarea body + the cell within it.
	ox, oy := flatui.CardOrigin()
	cx, cy := s.ta.CursorCell()
	frame.Cursor = &flatcore.Cursor{X: ox + cx, Y: oy + 3 + cy}
	return frame
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
