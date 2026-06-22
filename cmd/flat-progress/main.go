package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

const defaultProgressInterval = 120 * time.Millisecond

type State struct {
	progress flatui.Progress
	paused   bool
}

func NewState() *State {
	return &State{progress: flatui.NewProgress(24)}
}

func Init(s *State, fx flatte.Effects[State]) {
	flatte.Every(fx, "progress.tick", progressInterval(), applyTick)
}

func applyTick(s *State, _ time.Time) {
	if s.paused {
		return
	}
	s.progress.SetPercent(s.progress.Percent() + 10)
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(ev.Width)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func (s *State) layout(width int) {
	s.progress.SetWidth(max(flatui.CardBodyWidth(width)-8, 0))
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if key.Key != flatte.KeyCharacter {
		return
	}
	switch key.Rune {
	case ' ', 'p', 'P':
		s.paused = !s.paused
	case 'r', 'R':
		s.progress.SetPercent(0)
	case 'q', 'Q':
		fx.Quit()
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	status := "running"
	if s.paused {
		status = "paused"
	}
	lines := []string{
		flatui.Title("Flat Progress"),
		flatui.Subtle("horizontal loader sample"),
		"",
		"  " + s.progress.View(),
		"  state: " + status,
		"",
		flatui.Subtle("space pause | r reset | q quit"),
	}
	return flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func progressInterval() time.Duration {
	value := os.Getenv("FLAT_PROGRESS_INTERVAL")
	if value == "" {
		return defaultProgressInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return defaultProgressInterval
	}
	return interval
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
