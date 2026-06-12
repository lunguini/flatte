package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

type Field struct {
	Label string
	Input flatui.TextField
}

type State struct {
	fields    []Field
	focused   int
	editing   bool
	submitted string
}

func NewState() *State {
	return &State{
		editing: true,
		fields: []Field{
			{Label: "name"},
			{Label: "filter"},
		},
	}
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
	if !ok {
		return
	}
	if !s.editing {
		handleBlurred(s, key, fx)
		return
	}

	field := &s.fields[s.focused]
	switch key.Key {
	case flatcore.KeyTab:
		s.focused = (s.focused + 1) % len(s.fields)
	case flatcore.KeyEscape:
		s.editing = false
	case flatcore.KeyEnter:
		s.submitted = fmt.Sprintf("name=%s filter=%s", s.fields[0].Input.Value, s.fields[1].Input.Value)
	case flatcore.KeyLeft:
		field.Input.MoveLeft()
	case flatcore.KeyRight:
		field.Input.MoveRight()
	case flatcore.KeyBackspace:
		field.Input.Backspace()
	case flatcore.KeyDelete:
		field.Input.Delete()
	case flatcore.KeyCharacter:
		field.Input.Insert(key.Rune)
	}
}

func handleBlurred(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	switch key.Key {
	case flatcore.KeyEnter:
		s.editing = true
	case flatcore.KeyCharacter:
		if key.Rune == 'q' || key.Rune == 'Q' {
			fx.Quit()
		}
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	lines := []string{
		flatui.Title("Flat Form"),
		flatui.Subtle("multi-input retained state sample"),
		"",
	}

	for i, field := range s.fields {
		prefix := "  "
		if s.editing && i == s.focused {
			prefix = "> "
		}
		lines = append(lines, prefix+field.Label+": "+renderField(s, i))
	}

	lines = append(lines, "")
	if s.submitted == "" {
		lines = append(lines, flatui.Subtle("  not submitted"))
	} else {
		lines = append(lines, "  "+s.submitted)
	}
	lines = append(lines, "", flatui.Subtle("tab focus | arrows move | esc blur | q quits blurred"))

	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func renderField(s *State, index int) string {
	field := s.fields[index]
	return field.Input.Render(s.editing && index == s.focused)
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
