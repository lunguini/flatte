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

const defaultTickInterval = time.Second

type State struct {
	ticks  int
	paused bool
}

func Init(s *State, fx flatte.Effects[State]) {
	if err := flatte.Every(fx, "ticker.tick", tickInterval(), applyTick); err != nil {
		panic(err) // tickInterval is a sample invariant
	}
}

func applyTick(s *State, _ time.Time) {
	if !s.paused {
		s.ticks++
	}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok || key.Key != flatte.KeyCharacter {
		return
	}
	switch key.Rune {
	case ' ', 'p', 'P':
		s.paused = !s.paused
	case 'r', 'R':
		s.ticks = 0
	case 'q', 'Q':
		fx.Quit()
	}
}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	status := "running"
	if s.paused {
		status = "paused"
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Ticker", Style: title},
		layout.Label{Text: "streaming update sample", Style: subtle},
		layout.Label{},
		layout.Label{NodeBase: indent, Text: fmt.Sprintf("ticks: %d\nstate: %s", s.ticks, status)},
		layout.Label{},
		layout.Label{Text: "space/p pause | r reset | q quit", Style: subtle},
	)}
}

func tickInterval() time.Duration {
	value := os.Getenv("FLAT_TICKER_INTERVAL")
	if value == "" {
		return defaultTickInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return defaultTickInterval
	}
	return interval
}

func main() {
	state := &State{}
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Init:   Init,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
