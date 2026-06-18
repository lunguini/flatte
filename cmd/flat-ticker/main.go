package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

const defaultTickInterval = time.Second

type State struct {
	ticks  int
	paused bool
}

func Init(s *State, fx flat.Effects[State]) {
	flat.Every(fx, "ticker.tick", tickInterval(), applyTick)
}

func applyTick(s *State, _ time.Time) {
	if !s.paused {
		s.ticks++
	}
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
	if !ok || key.Key != flat.KeyCharacter {
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

func View(s *State, ctx flat.RenderContext) flat.Frame {
	status := "running"
	if s.paused {
		status = "paused"
	}

	lines := []string{
		flatui.Title("Flat Ticker"),
		flatui.Subtle("streaming update sample"),
		"",
		fmt.Sprintf("  ticks: %d", s.ticks),
		"  state: " + status,
		"",
		flatui.Subtle("space/p pause | r reset | q quit"),
	}
	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
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
	err := flat.Run(context.Background(), flat.App[State]{
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
