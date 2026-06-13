package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

// items is fixed so goldens stay deterministic; long enough that the list
// scrolls inside a normal terminal.
func items() []string {
	out := make([]string, 30)
	for i := range out {
		out[i] = fmt.Sprintf("Item %02d", i+1)
	}
	return out
}

// listTopLine is the content-line index of the first list row inside the card:
// title, subtitle, and a blank line precede it.
const listTopLine = 3

type State struct {
	items  []string
	list   flatui.List
	chosen int // index confirmed with enter, -1 if none
}

func NewState() *State {
	s := &State{items: items(), chosen: -1}
	s.list.SetCount(len(s.items))
	return s
}

// layout sizes the list to the rows left after the pinned chrome (title,
// subtitle, blank, blank, footer = 5) and the card's top+bottom border (2).
func (s *State) layout(height int) {
	const pinnedRows = 5
	const vChrome = 2
	s.list.SetHeight(max(height-pinnedRows-vChrome, 1))
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	switch e := ev.(type) {
	case flatcore.ResizeEvent:
		s.layout(e.Height)
	case flatcore.KeyEvent:
		handleKey(s, e, fx)
	case flatcore.MouseEvent:
		handleMouse(s, e)
	}
}

func handleKey(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	switch key.Key {
	case flatcore.KeyDown:
		s.list.MoveDown()
	case flatcore.KeyUp:
		s.list.MoveUp()
	case flatcore.KeyEnter:
		s.chosen = s.list.Cursor()
	case flatcore.KeyCharacter:
		switch key.Rune {
		case 'j':
			s.list.MoveDown()
		case 'k':
			s.list.MoveUp()
		case 'g':
			s.list.Select(0)
		case 'G':
			s.list.Select(s.list.Count() - 1)
		case 'q':
			fx.Quit()
		}
	}
}

func handleMouse(s *State, m flatcore.MouseEvent) {
	switch m.Button {
	case flatcore.MouseWheelUp:
		s.list.MoveUp()
	case flatcore.MouseWheelDown:
		s.list.MoveDown()
	case flatcore.MouseLeft:
		if m.Action != flatcore.MousePress {
			return
		}
		// Click row -> item index: the visible row plus the scroll offset,
		// mapped back through the card's top border + the pinned lines.
		_, cardTop := flatui.CardOrigin()
		row := m.Y - cardTop - listTopLine
		if row >= 0 {
			s.list.Select(s.list.Offset() + row)
		}
	}
}

func (s *State) renderRow(i int, selected bool) string {
	marker := "  "
	style := itemStyle()
	if selected {
		marker = "> "
		style = activeStyle()
	}
	label := s.items[i]
	if i == s.chosen {
		label += " " + chosenStyle().Render("(selected)")
	}
	return style.Render(marker + label)
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	footer := flatui.Subtle(fmt.Sprintf(
		"j/k move  g/G ends  enter select  q quit    [%d/%d]",
		s.list.Cursor()+1, s.list.Count()))

	lines := []string{
		flatui.Title("Flat List"),
		flatui.Subtle("scrollable selectable list"),
		"",
	}
	lines = append(lines, strings.Split(s.list.View(s.renderRow), "\n")...)
	lines = append(lines, "", footer)

	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func itemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
}

func activeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
}

func chosenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatcore.WithMouse(flatcore.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
