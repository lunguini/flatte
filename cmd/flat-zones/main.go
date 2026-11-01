package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/cmd/internal/brand"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

const (
	logsZone    = "logs"
	metricsZone = "metrics"
	panelGap    = 2
)

type State struct {
	selected string
	last     string
	zones    flatui.ZoneMap
}

func NewState() *State {
	return &State{selected: "none", last: "no clicks yet"}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		// The panels are ID'd nodes: the solved frame reports their rects,
		// and those are the hit regions.
		s.zones.Clear()
		for _, id := range []string{logsZone, metricsZone} {
			if r, ok := e.Rects[id]; ok {
				s.zones.Set(id, r)
			}
		}
	case flatte.KeyEvent:
		if e.Key == flatte.KeyEscape || (e.Key == flatte.KeyCharacter && (e.Rune == 'q' || e.Rune == 'Q')) {
			fx.Quit()
		}
	case flatte.MouseEvent:
		handleMouse(s, e)
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	if m.Button != flatte.MouseLeft || m.Action != flatte.MousePress {
		return
	}
	id, ok := s.zones.At(m.X, m.Y)
	if !ok {
		s.last = "outside"
		return
	}
	localX, localY, _ := s.zones.Local(id, m.X, m.Y)
	s.selected = id
	s.last = fmt.Sprintf("%s local %d,%d", id, localX, localY)
}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
)

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Zones", Style: title},
		layout.Label{Text: "explicit hit regions", Style: subtle},
		layout.Label{},
		layout.Row{NodeBase: layout.NodeBase{Gap: panelGap}, Children: []layout.Node{
			panel(logsZone, "event stream", s.selected == logsZone),
			panel(metricsZone, "dashboards", s.selected == metricsZone),
		}},
		layout.Label{},
		layout.Label{Text: "selected: " + s.selected},
		layout.Label{Text: "last: " + s.last, Style: subtle},
		layout.Label{Text: "click a panel | q/esc quit", Style: subtle},
	)}
}

// panel is a filled three-line panel with the zone's ID; it takes half the
// row.
func panel(id, body string, active bool) layout.Node {
	text, bg := layout.TextStyle{Fg: lipgloss.Color("252")}, lipgloss.Color("235")
	if active {
		text, bg = layout.TextStyle{Fg: brand.Teal, Bold: true}, lipgloss.Color("57")
	}
	state := "idle"
	if active {
		state = "selected"
	}
	return layout.Col{
		NodeBase: layout.NodeBase{ID: id, W: layout.Grow(1), PadLeft: 1, Box: layout.Box{Background: bg}},
		Children: []layout.Node{
			layout.Label{Text: strings.ToUpper(id) + "\n" + body + "\n" + state, Style: text},
		},
	}
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
