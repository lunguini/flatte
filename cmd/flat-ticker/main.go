package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

const defaultTickInterval = time.Second

type State struct {
	ticks  int
	paused bool
}

func Init(s *State, fx flatcore.Effects[State]) {
	flatcore.Every(fx, "ticker.tick", tickInterval(), applyTick)
}

func applyTick(s *State, _ time.Time) {
	if !s.paused {
		s.ticks++
	}
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
	if !ok || key.Key != flatcore.KeyCharacter {
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

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
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
	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
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
	err := flatcore.Run(context.Background(), flatcore.App[State]{
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
