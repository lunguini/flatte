// Command flat-scroll shows layout content taller than its window: a service
// list and a log, each a layout.Scroll whose offset is app state.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

// wheelStep is how many rows one wheel notch scrolls a pane.
const wheelStep = 3

// logLines is the length of every service's log; fixed so goldens stay
// deterministic.
const logLines = 60

type service struct{ name, status string }

func services() []service {
	statuses := []string{"running", "running", "restarting", "stopped"}
	out := make([]service, 40)
	for i := range out {
		out[i] = service{name: fmt.Sprintf("service-%02d", i+1), status: statuses[i%len(statuses)]}
	}
	return out
}

// State owns both scroll offsets. Geometry comes from the last LayoutEvent;
// row positions are read content-relative, so they hold at any offset.
type State struct {
	services []service
	selected int
	listY    int
	logY     int
	rects    map[string]layout.Rect
}

func NewState() *State {
	return &State{services: services(), rects: map[string]layout.Rect{}}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.LayoutEvent:
		prev := s.rects["list"]
		s.rects = ev.Rects
		// A resized window keeps the selection in view. Scrolling moves only
		// the content, never the window, so wheeling the selection away is
		// left alone.
		if s.rects["list"].H != prev.H {
			s.revealSelected()
		}
		s.listY = s.clamp("list", s.listY)
		s.logY = s.clamp("log", s.logY)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	case flatte.MouseEvent:
		handleMouse(s, ev)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		s.selectService(s.selected + 1)
	case flatte.KeyUp:
		s.selectService(s.selected - 1)
	case flatte.KeyPageDown:
		s.logY = s.clamp("log", s.logY+s.rects["log"].H)
	case flatte.KeyPageUp:
		s.logY = s.clamp("log", s.logY-s.rects["log"].H)
	case flatte.KeyHome:
		s.logY = 0
	case flatte.KeyEnd:
		s.logY = s.clamp("log", logLines)
	case flatte.KeyCtrlC:
		fx.Quit()
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j':
			s.selectService(s.selected + 1)
		case 'k':
			s.selectService(s.selected - 1)
		case 'g':
			s.selectService(0)
		case 'G':
			s.selectService(len(s.services) - 1)
		case 'q':
			fx.Quit()
		}
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	pane := paneOf(m.Target)
	switch m.Button {
	case flatte.MouseWheelUp, flatte.MouseWheelDown:
		step := wheelStep
		if m.Button == flatte.MouseWheelUp {
			step = -step
		}
		switch pane {
		case "list":
			s.listY = s.clamp("list", s.listY+step)
		case "log":
			s.logY = s.clamp("log", s.logY+step)
		}
	case flatte.MouseLeft:
		if m.Action != flatte.MousePress {
			return
		}
		if i, ok := strings.CutPrefix(m.Target, "row."); ok {
			if n, err := strconv.Atoi(i); err == nil {
				s.selectService(n)
			}
		}
	}
}

// paneOf maps a hit target to the scroll pane it sits in.
func paneOf(target string) string {
	switch {
	case target == "list" || target == "list.content" || strings.HasPrefix(target, "row."):
		return "list"
	case target == "log" || target == "log.content":
		return "log"
	}
	return ""
}

func (s *State) selectService(i int) {
	i = min(max(i, 0), len(s.services)-1)
	if i == s.selected {
		return
	}
	s.selected = i
	s.logY = 0
	s.revealSelected()
}

// revealSelected scrolls the list the least amount that shows the selected
// row. It uses content-relative row geometry, which is the same at every
// offset, so the rects from the last layout are always good enough.
func (s *State) revealSelected() {
	view, okV := s.rects["list"]
	content, okC := s.rects["list.content"]
	row, okR := s.rects[rowID(s.selected)]
	if !okV || !okC || !okR {
		return
	}
	top := row.Y - content.Y
	switch {
	case top < s.listY:
		s.listY = top
	case top+row.H > s.listY+view.H:
		s.listY = top + row.H - view.H
	}
}

// clamp bounds a pane's offset by its window and content heights, the same
// rule the layout applies when painting.
func (s *State) clamp(pane string, offset int) int {
	view, okV := s.rects[pane]
	content, okC := s.rects[pane+".content"]
	if !okV || !okC {
		return max(offset, 0)
	}
	return layout.ClampScroll(offset, view.H, content.H)
}

func rowID(i int) string { return "row." + strconv.Itoa(i) }

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	svc := s.services[s.selected]
	title := flatui.Title("Flat Scroll") + flatui.Subtle(fmt.Sprintf("  ·  %d services, %d log lines each", len(s.services), logLines))
	return flatte.Frame{
		Root: layout.Col{Children: []layout.Node{
			layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: title},
			layout.Row{
				NodeBase: layout.NodeBase{H: layout.Grow(1), Gap: 1},
				Children: []layout.Node{
					// The border sits on a wrapping Col so each Scroll's rect is
					// exactly its window, which is what clamp reads.
					layout.Col{
						NodeBase: layout.NodeBase{W: layout.Fixed(26), Bordered: true},
						Children: []layout.Node{layout.Scroll{
							NodeBase: layout.NodeBase{ID: "list"},
							OffsetY:  s.listY,
							Child:    layout.Col{NodeBase: layout.NodeBase{ID: "list.content"}, Children: s.rows()},
						}},
					},
					layout.Col{
						NodeBase: layout.NodeBase{W: layout.Grow(1), Bordered: true},
						Children: []layout.Node{
							layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: headerStyle().Render(svc.name) + flatui.Subtle("  "+svc.status)},
							layout.Scroll{
								NodeBase: layout.NodeBase{ID: "log"},
								OffsetY:  s.logY,
								Child:    layout.Text{NodeBase: layout.NodeBase{ID: "log.content"}, String: logText(s.selected)},
							},
						},
					},
				},
			},
			layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: s.footer()},
		}},
		Title: "flat-scroll — " + svc.name,
	}
}

func (s *State) rows() []layout.Node {
	rows := make([]layout.Node, len(s.services))
	for i, svc := range s.services {
		marker, style := "  ", itemStyle()
		if i == s.selected {
			marker, style = "> ", activeStyle()
		}
		rows[i] = layout.Text{
			NodeBase: layout.NodeBase{ID: rowID(i), H: layout.Fixed(1), PadLeft: 1},
			String:   style.Render(marker+svc.name) + " " + statusStyle(svc.status).Render(svc.status),
		}
	}
	return rows
}

func (s *State) footer() string {
	first, last := s.logY+1, min(s.logY+s.rects["log"].H, logLines)
	return flatui.Subtle(fmt.Sprintf(
		"j/k select · wheel scrolls pane · pgup/pgdn log · q quit  [%d/%d · log %d–%d/%d]",
		s.selected+1, len(s.services), first, last, logLines))
}

// logText is a deterministic log for service i.
func logText(i int) string {
	levels := []string{"INFO ", "INFO ", "DEBUG", "WARN "}
	messages := []string{"health check ok", "accepted connection", "flushed write buffer", "slow request (412ms)", "rotated log file"}
	lines := make([]string, logLines)
	for n := range lines {
		lines[n] = fmt.Sprintf("line %02d  12:%02d:%02d  %s  %s", n+1, (n+i)/60%60, (n*7+i)%60, levels[(n+i)%len(levels)], messages[(n*3+i)%len(messages)])
	}
	return strings.Join(lines, "\n")
}

func headerStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(brand.Teal) }
func itemStyle() lipgloss.Style   { return lipgloss.NewStyle().Foreground(lipgloss.Color("252")) }
func activeStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(brand.Teal) }

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "stopped":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	case "restarting":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
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
