package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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
func Init(s *State, fx flatte.Effects[State]) {
	if err := flatte.Every(fx, "spin", interval, func(s *State, _ time.Time) {
		s.spinner.Tick()
	}); err != nil {
		panic(err) // interval is a sample invariant
	}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	if key, ok := ev.(flatte.KeyEvent); ok && key.Key == flatte.KeyCharacter && key.Rune == 'q' {
		fx.Quit()
	}
}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Spinner", Style: title},
		layout.Label{Text: "activity indicator sample", Style: subtle},
		layout.Label{},
		layout.Label{NodeBase: indent, Text: s.spinner.View() + "  " + s.label},
		layout.Label{},
		layout.Label{Text: "q quit", Style: subtle},
	)}
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
