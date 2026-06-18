package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
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

// listTopLine is the content-line index of the first model row: the
// title, subtitle, and a blank line precede it.
const listTopLine = 3

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch ev := ev.(type) {
	case flat.KeyEvent:
		handleKey(s, ev, fx)
	case flat.MouseEvent:
		handleMouse(s, ev)
	}
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	switch key.Key {
	case flat.KeyDown:
		moveDown(s)
	case flat.KeyUp:
		moveUp(s)
	case flat.KeyEnter:
		if len(s.models) > 0 {
			s.selectedModel = s.models[s.cursor]
		}
	case flat.KeyCharacter:
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

func handleMouse(s *State, m flat.MouseEvent) {
	switch m.Button {
	case flat.MouseWheelUp:
		moveUp(s)
	case flat.MouseWheelDown:
		moveDown(s)
	case flat.MouseLeft:
		if m.Action != flat.MousePress {
			return
		}
		// Map the click row back to a model index through the same layout
		// arithmetic the cursor uses: card top border + the lines above
		// the list.
		_, cardTop := flatui.CardOrigin()
		row := m.Y - cardTop - listTopLine
		if row >= 0 && row < len(s.models) {
			s.cursor = row
			s.selectedModel = s.models[row]
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

func View(s *State, ctx flat.RenderContext) flat.Frame {
	rows := make([]string, 0, len(s.models))
	if s.loading {
		rows = append(rows, flatui.Subtle("  loading models..."))
	} else if s.err != nil {
		rows = append(rows, errorStyle().Render("  "+s.err.Error()))
	} else {
		for i, model := range s.models {
			prefix := "  "
			style := itemStyle()
			if i == s.cursor {
				prefix = "> "
				style = activeStyle()
			}
			row := prefix + model
			if model == s.selectedModel {
				row += " " + selectedStyle().Render("(selected)")
			}
			rows = append(rows, style.Render(row))
		}
	}

	selected := "selected: none"
	if s.selectedModel != "" {
		selected = "selected: " + s.selectedModel
	}

	lines := []string{
		flatui.Title("Flatte"),
		flatui.Subtle("async list selection"),
		"",
		strings.Join(rows, "\n"),
		"",
		selected,
		flatui.Subtle("j/k or click/wheel | enter select | q quit"),
	}

	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func itemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
}

func activeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("229"))
}

func selectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("108"))
}

func errorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("203"))
}

func loadModels(s *State, fx flat.Effects[State]) {
	flat.Go(fx, "models.load", fetchModels, func(s *State, models []string, err error) {
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
	err := flat.Run(context.Background(), flat.App[State]{
		State:  state,
		Init:   loadModels,
		Handle: Handle,
		View:   View,
	}, flat.WithMouse(flat.MouseModeCellMotion))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
