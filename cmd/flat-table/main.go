package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

type State struct {
	tb    flatui.Table
	total int
}

func NewState() *State {
	s := &State{}
	s.tb.SetColumns([]flatui.Column{
		{Title: "ID", Width: 4},
		{Title: "Name", Width: 14},
		{Title: "Status", Width: 6},
	})
	rows := make([][]string, 20)
	for i := range rows {
		rows[i] = []string{
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("service-%02d", i+1),
			status(i),
		}
	}
	s.tb.SetRows(rows)
	s.total = len(rows)
	return s
}

func status(i int) string {
	switch i % 3 {
	case 0:
		return "ok"
	case 1:
		return "warn"
	default:
		return "down"
	}
}

// layout sizes the body to the rows left after the pinned chrome: title,
// subtitle, blank, header, blank, footer = 6, plus the card border (2).
func (s *State) layout(height int) {
	const pinnedRows = 6 // title, subtitle, blank, header, blank, footer
	s.tb.SetHeight(max(flatui.CardBodyHeight(height, pinnedRows), 1))
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(e.Height)
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		switch e.Button {
		case flatte.MouseWheelUp:
			s.tb.MoveUp()
		case flatte.MouseWheelDown:
			s.tb.MoveDown()
		}
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		s.tb.MoveDown()
	case flatte.KeyUp:
		s.tb.MoveUp()
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j':
			s.tb.MoveDown()
		case 'k':
			s.tb.MoveUp()
		case 'g':
			s.tb.Select(0)
		case 'G':
			s.tb.Select(s.total - 1)
		case 'q':
			fx.Quit()
		}
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	selLabel := "none"
	if sel := s.tb.SelectedRow(); len(sel) >= 2 {
		selLabel = sel[1]
	}
	footer := flatui.Subtle(fmt.Sprintf(
		"j/k move  g/G ends  q quit   selected: %s  [%d/%d]",
		selLabel, s.tb.Cursor()+1, s.total))

	body := s.tb.View(func(text string, selected bool) string {
		if selected {
			return activeStyle().Render("> " + text)
		}
		return "  " + text
	})

	lines := []string{
		flatui.Title("Flat Table"),
		flatui.Subtle("scrollable selectable table"),
		"",
		headerStyle().Render("  " + s.tb.Header()), // indent matches the row marker
	}
	lines = append(lines, strings.Split(body, "\n")...)
	lines = append(lines, "", footer)

	return flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func headerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
}

func activeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatte.WithMouse(flatte.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
