package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.PasteEvent:
		if s.editing {
			s.fields[s.focused].Input.InsertText(ev.Text)
		}
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if !s.editing {
		handleBlurred(s, key, fx)
		return
	}

	field := &s.fields[s.focused]
	switch key.Key {
	case flatte.KeyTab:
		s.focused = (s.focused + 1) % len(s.fields)
	case flatte.KeyEscape:
		s.editing = false
	case flatte.KeyEnter:
		s.submitted = fmt.Sprintf("name=%s filter=%s", s.fields[0].Input.Value, s.fields[1].Input.Value)
	default:
		field.Input.Edit(key)
	}
}

func handleBlurred(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyEnter:
		s.editing = true
	case flatte.KeyCharacter:
		if key.Rune == 'q' || key.Rune == 'Q' {
			fx.Quit()
		}
	}
}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// View lays out one row per field. The focused field's Input places the
// cursor, so there is no cursor arithmetic here.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	children := []layout.Node{
		layout.Label{Text: "Flat Form", Style: title},
		layout.Label{Text: "multi-input retained state sample", Style: subtle},
		layout.Label{},
	}
	for i, field := range s.fields {
		focused := s.editing && i == s.focused
		prefix := "  "
		if focused {
			prefix = "> "
		}
		children = append(children, layout.Row{Children: []layout.Node{
			layout.Label{Text: prefix + field.Label + ": "},
			field.Input.Layout("form.field."+strconv.Itoa(i), focused),
		}})
	}
	children = append(children, layout.Label{})
	if s.submitted == "" {
		children = append(children, layout.Label{NodeBase: indent, Text: "not submitted", Style: subtle})
	} else {
		children = append(children, layout.Label{NodeBase: indent, Text: s.submitted})
	}
	children = append(children, layout.Label{}, layout.Label{Text: "tab focus | arrows move | esc blur | q quits blurred", Style: subtle})
	return flatte.Frame{Root: flatui.CardLayout(children...)}
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
