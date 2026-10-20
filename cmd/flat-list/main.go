package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/cmd/internal/brand"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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

// listID names the list: its rows are "items:<index>", and its solved
// height is the number of rows it shows.
const listID = "items"

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

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		// The list grows into the card's free rows; show as many items.
		if r, ok := e.Rects[listID]; ok {
			s.list.SetHeight(max(r.H, 1))
		}
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		handleMouse(s, e)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		s.list.MoveDown()
	case flatte.KeyUp:
		s.list.MoveUp()
	case flatte.KeyEnter:
		s.chosen = s.list.Cursor()
	case flatte.KeyCharacter:
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

func handleMouse(s *State, m flatte.MouseEvent) {
	switch m.Button {
	case flatte.MouseWheelUp:
		s.list.MoveUp()
	case flatte.MouseWheelDown:
		s.list.MoveDown()
	case flatte.MouseLeft:
		if m.Action != flatte.MousePress {
			return
		}
		// Each row is an ID'd node: the click names the item it hit.
		if i, ok := strings.CutPrefix(m.Target, listID+":"); ok {
			if n, err := strconv.Atoi(i); err == nil {
				s.list.Select(n)
			}
		}
	}
}

// renderRow is one item: the selection marker (a chevron in a window, "> "
// in the terminal), the label, and the chosen mark.
func (s *State) renderRow(i int, selected bool) layout.Node {
	marker := layout.Icon{Text: "  ", Style: itemStyle}
	style := itemStyle
	if selected {
		marker = layout.Icon{Name: layout.IconChevronRight, Text: "> ", Style: activeStyle}
		style = activeStyle
	}
	row := layout.Row{Children: []layout.Node{marker, layout.Label{Text: s.items[i], Style: style}}}
	if i == s.chosen {
		row.Children = append(row.Children, layout.Label{Text: " (selected)", Style: chosenStyle})
	}
	return row
}

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	rows := s.list.Rows(listID, s.renderRow)
	rows.H = layout.Grow(1)
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat List", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: "scrollable selectable list", Style: layout.TextStyle{Role: layout.TextSubtle}},
		layout.Label{},
		rows,
		layout.Label{},
		layout.Label{Text: fmt.Sprintf("j/k move  g/G ends  enter select  q quit    [%d/%d]", s.list.Cursor()+1, s.list.Count()), Style: layout.TextStyle{Role: layout.TextSubtle}},
	)}
}

var (
	itemStyle   = layout.TextStyle{Fg: lipgloss.Color("252")}
	activeStyle = layout.TextStyle{Fg: brand.Teal, Bold: true}
	chosenStyle = layout.TextStyle{Fg: lipgloss.Color("108")}
)

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
