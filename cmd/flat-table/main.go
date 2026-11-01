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

// bodyID names the table body; its rows are "rows:<index>", and its solved
// height is the number of rows it shows.
const bodyID = "rows"

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := e.Rects[bodyID]; ok {
			s.tb.SetHeight(max(r.H, 1))
		}
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		switch e.Button {
		case flatte.MouseWheelUp:
			s.tb.MoveUp()
		case flatte.MouseWheelDown:
			s.tb.MoveDown()
		case flatte.MouseLeft:
			// Each row is an ID'd node: the click names the row it hit.
			if i, ok := strings.CutPrefix(e.Target, bodyID+":"); ok && e.Action == flatte.MousePress {
				if n, err := strconv.Atoi(i); err == nil {
					s.tb.Select(n)
				}
			}
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

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	selLabel := "none"
	if sel := s.tb.SelectedRow(); len(sel) >= 2 {
		selLabel = sel[1]
	}
	body := s.tb.Rows(bodyID, func(cells []string, selected bool) layout.Node {
		if selected {
			return layout.Row{Children: []layout.Node{
				layout.Icon{Name: layout.IconChevronRight, Text: "> ", Style: activeStyle},
				s.tb.Cells(cells, activeStyle),
			}}
		}
		return layout.Row{Children: []layout.Node{layout.Icon{Text: "  "}, s.tb.Cells(cells, layout.TextStyle{})}}
	})
	body.ID, body.H = bodyID, layout.Grow(1)
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Table", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: "scrollable selectable table", Style: layout.TextStyle{Role: layout.TextSubtle}},
		layout.Label{},
		// The header is indented to match the row marker.
		layout.Row{NodeBase: layout.NodeBase{PadLeft: 2}, Children: []layout.Node{s.tb.HeaderLayout(headerStyle)}},
		body,
		layout.Label{},
		layout.Label{Text: fmt.Sprintf("j/k move  g/G ends  q quit   selected: %s  [%d/%d]", selLabel, s.tb.Cursor()+1, s.total), Style: layout.TextStyle{Role: layout.TextSubtle}},
	)}
}

var (
	headerStyle = layout.TextStyle{Bold: true, Fg: lipgloss.Color("252")}
	activeStyle = layout.TextStyle{Bold: true, Fg: brand.Teal}
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
