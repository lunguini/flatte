package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

const defaultLoadDelay = 250 * time.Millisecond

// State is the single source of truth for the spike.
type State struct {
	models        []string
	cursor        int
	selectedModel string
	loading       bool
	err           error
}

func NewState() *State {
	return &State{loading: true}
}

// modelID is the ID of model row i; a click's Target names the row.
func modelID(i int) string { return "model:" + strconv.Itoa(i) }

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	case flatte.MouseEvent:
		handleMouse(s, ev)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		moveDown(s)
	case flatte.KeyUp:
		moveUp(s)
	case flatte.KeyEnter:
		if len(s.models) > 0 {
			s.selectedModel = s.models[s.cursor]
		}
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j', 'J':
			moveDown(s)
		case 'k', 'K':
			moveUp(s)
		case 'q', 'Q':
			fx.Quit()
		}
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	switch m.Button {
	case flatte.MouseWheelUp:
		moveUp(s)
	case flatte.MouseWheelDown:
		moveDown(s)
	case flatte.MouseLeft:
		if m.Action != flatte.MousePress {
			return
		}
		if i, ok := strings.CutPrefix(m.Target, "model:"); ok {
			if n, err := strconv.Atoi(i); err == nil && n >= 0 && n < len(s.models) {
				s.cursor = n
				s.selectedModel = s.models[n]
			}
		}
	}
}

func moveDown(s *State) {
	if s.cursor < len(s.models)-1 {
		s.cursor++
	}
}

func moveUp(s *State) {
	if s.cursor > 0 {
		s.cursor--
	}
}

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	var rows []layout.Node
	switch {
	case s.loading:
		rows = append(rows, layout.Label{NodeBase: indent, Text: "loading models...", Style: layout.TextStyle{Role: layout.TextSubtle}})
	case s.err != nil:
		rows = append(rows, layout.Label{NodeBase: indent, Text: s.err.Error(), Style: errorStyle})
	default:
		for i, model := range s.models {
			rows = append(rows, modelRow(s, i, model))
		}
	}
	selected := "selected: none"
	if s.selectedModel != "" {
		selected = "selected: " + s.selectedModel
	}
	children := []layout.Node{
		layout.Label{Text: "Flatte", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: "async list selection", Style: layout.TextStyle{Role: layout.TextSubtle}},
		layout.Label{},
	}
	children = append(children, rows...)
	children = append(children,
		layout.Label{},
		layout.Label{Text: selected},
		layout.Label{Text: "j/k or click/wheel | enter select | q quit", Style: layout.TextStyle{Role: layout.TextSubtle}},
	)
	return flatte.Frame{Root: flatui.CardLayout(children...)}
}

// modelRow is one model: the cursor marker (a chevron in a window, "> " in
// the terminal), the name, and the chosen mark, as an ID'd row.
func modelRow(s *State, i int, model string) layout.Node {
	marker, style := layout.Icon{Text: "  ", Style: itemStyle}, itemStyle
	if i == s.cursor {
		marker, style = layout.Icon{Name: layout.IconChevronRight, Text: "> ", Style: activeStyle}, activeStyle
	}
	row := layout.Row{
		NodeBase: layout.NodeBase{ID: modelID(i), Box: layout.Box{Selected: i == s.cursor}},
		Children: []layout.Node{marker, layout.Label{Text: model, Style: style}},
	}
	if model == s.selectedModel {
		row.Children = append(row.Children, layout.Label{Text: " (selected)", Style: selectedStyle})
	}
	return row
}

var (
	indent        = layout.NodeBase{PadLeft: 2}
	itemStyle     = layout.TextStyle{Fg: lipgloss.Color("252")}
	activeStyle   = layout.TextStyle{Fg: brand.Teal, Bold: true}
	selectedStyle = layout.TextStyle{Fg: lipgloss.Color("108")}
	errorStyle    = layout.TextStyle{Fg: brand.Pink}
)

func loadModels(s *State, fx flatte.Effects[State]) {
	flatte.Go(fx, "models.load", fetchModels, func(s *State, models []string, err error) {
		s.loading = false
		if err != nil {
			s.err = err
			return
		}
		s.models = models
		if s.cursor >= len(s.models) {
			s.cursor = max(0, len(s.models)-1)
		}
	})
}

func fetchModels(ctx context.Context) ([]string, error) {
	select {
	case <-time.After(loadDelay()):
		return []string{"haiku", "sonnet", "opus", "freeform"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func loadDelay() time.Duration {
	value := os.Getenv("FLAT_SPIKE_LOAD_DELAY")
	if value == "" {
		return defaultLoadDelay
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return defaultLoadDelay
	}
	return delay
}

func main() {
	state := NewState()
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Init:   loadModels,
		Handle: Handle,
		View:   View,
	}, flatte.WithMouse(flatte.MouseModeCellMotion))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
