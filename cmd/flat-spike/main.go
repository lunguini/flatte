package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
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

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatcore.KeyDown:
		moveDown(s)
	case flatcore.KeyUp:
		moveUp(s)
	case flatcore.KeyEnter:
		if len(s.models) > 0 {
			s.selectedModel = s.models[s.cursor]
		}
	case flatcore.KeyCharacter:
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

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
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
		flatui.Subtle("j/k move | enter select | q quit"),
	}

	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
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

func loadModels(s *State, fx flatcore.Effects[State]) {
	flatcore.Go(fx, "models.load", fetchModels, func(s *State, models []string, err error) {
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
	err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  state,
		Init:   loadModels,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
