package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

const interval = 100 * time.Millisecond

type State struct {
	spinner flatui.Spinner
	label   string
}

func NewState() *State {
	return &State{spinner: flatui.NewSpinner(flatui.SpinnerDots), label: "working..."}
}

// Init starts the animation: Every drives the spinner from the loop goroutine;
// the widget itself owns no timer.
func Init(s *State, fx flatcore.Effects[State]) {
	flatcore.Every(fx, "spin", interval, func(s *State, _ time.Time) {
		s.spinner.Tick()
	})
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	if key, ok := ev.(flatcore.KeyEvent); ok && key.Key == flatcore.KeyCharacter && key.Rune == 'q' {
		fx.Quit()
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	lines := []string{
		flatui.Title("Flat Spinner"),
		flatui.Subtle("activity indicator sample"),
		"",
		"  " + s.spinner.View() + "  " + s.label,
		"",
		flatui.Subtle("q quit"),
	}
	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
