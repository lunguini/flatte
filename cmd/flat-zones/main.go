package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

const (
	logsZone    = "logs"
	metricsZone = "metrics"
	panelGap    = 2
	panelRows   = 3
	panelTop    = 3
)

type State struct {
	width    int
	selected string
	last     string
	zones    flatui.ZoneMap
}

func NewState() *State {
	s := &State{selected: "none", last: "no clicks yet"}
	s.layout(72)
	return s
}

func (s *State) layout(width int) {
	s.width = width
	bodyWidth := flatui.CardBodyWidth(width)
	panelWidth := max((bodyWidth-panelGap)/2, 8)
	originX, originY := flatui.CardOrigin()

	s.zones.Clear()
	s.zones.Set(logsZone, flatui.Rect{
		X:      originX,
		Y:      originY + panelTop,
		Width:  panelWidth,
		Height: panelRows,
	})
	s.zones.Set(metricsZone, flatui.Rect{
		X:      originX + panelWidth + panelGap,
		Y:      originY + panelTop,
		Width:  panelWidth,
		Height: panelRows,
	})
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch e := ev.(type) {
	case flat.ResizeEvent:
		s.layout(e.Width)
	case flat.KeyEvent:
		if e.Key == flat.KeyEscape || (e.Key == flat.KeyCharacter && (e.Rune == 'q' || e.Rune == 'Q')) {
			fx.Quit()
		}
	case flat.MouseEvent:
		handleMouse(s, e)
	}
}

func handleMouse(s *State, m flat.MouseEvent) {
	if m.Button != flat.MouseLeft || m.Action != flat.MousePress {
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

func View(s *State, ctx flat.RenderContext) flat.Frame {
	left, _ := s.zones.Rect(logsZone)
	right, _ := s.zones.Rect(metricsZone)
	leftRows := panel(logsZone, "event stream", left.Width, s.selected == logsZone)
	rightRows := panel(metricsZone, "dashboards", right.Width, s.selected == metricsZone)

	lines := []string{
		flatui.Title("Flat Zones"),
		flatui.Subtle("explicit hit regions"),
		"",
	}
	for i := range leftRows {
		lines = append(lines, leftRows[i]+strings.Repeat(" ", panelGap)+rightRows[i])
	}
	lines = append(lines,
		"",
		"selected: "+s.selected,
		flatui.Subtle("last: "+s.last),
		flatui.Subtle("click a panel | q/esc quit"),
	)

	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func panel(title, body string, width int, active bool) []string {
	style := panelStyle(active)
	return []string{
		style.Render(fit(" "+strings.ToUpper(title), width)),
		style.Render(fit(" "+body, width)),
		style.Render(fit(" "+selectionLabel(active), width)),
	}
}

func selectionLabel(active bool) string {
	if active {
		return "selected"
	}
	return "idle"
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) > width {
		return s[:max(width, 0)]
	}
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 0))
}

func panelStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if active {
		return style.Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	}
	return style.Background(lipgloss.Color("235"))
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flat.WithMouse(flat.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
