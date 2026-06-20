package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

const (
	tickInterval    = time.Second
	countdownLength = 10 * time.Second
)

type State struct {
	timer     flatui.Timer
	stopwatch flatui.Stopwatch
	progress  flatui.Progress
}

func NewState() *State {
	return &State{
		timer:    flatui.NewTimer(countdownLength),
		progress: flatui.NewProgress(24),
	}
}

func Init(s *State, fx flat.Effects[State]) {
	flat.Every(fx, "timer.tick", tickInterval, applyTick)
}

func applyTick(s *State, _ time.Time) {
	s.timer.Tick(tickInterval)
	s.stopwatch.Tick(tickInterval)
	s.progress.SetPercent(s.timer.Percent())
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch ev := ev.(type) {
	case flat.ResizeEvent:
		s.layout(ev.Width)
	case flat.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func (s *State) layout(width int) {
	s.progress.SetWidth(max(flatui.CardBodyWidth(width)-20, 0))
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	if key.Key == flat.KeyEscape {
		fx.Quit()
		return
	}
	if key.Key != flat.KeyCharacter {
		return
	}
	switch key.Rune {
	case ' ', 's', 'S':
		s.stopwatch.Toggle()
	case 'r', 'R':
		s.timer.Reset()
		s.stopwatch.Reset()
		s.progress.SetPercent(0)
	case 't', 'T':
		s.timer.Reset()
		s.progress.SetPercent(0)
	case 'q', 'Q':
		fx.Quit()
	}
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	stopState := "stopped"
	if s.stopwatch.Running() {
		stopState = "running"
	}
	timerState := "running"
	if s.timer.Done() {
		timerState = "done"
	}
	lines := []string{
		flatui.Title("Flat Timer"),
		flatui.Subtle("countdown + stopwatch sample"),
		"",
		fmt.Sprintf("  timer:     %s remaining (%s)", formatDuration(s.timer.Remaining()), timerState),
		"  progress:  " + s.progress.View(),
		fmt.Sprintf("  stopwatch: %s (%s)", formatDuration(s.stopwatch.Elapsed()), stopState),
		"",
		flatui.Subtle("space start/stop | r reset all | t restart timer | q/esc quit"),
	}
	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Second).Seconds())
	minutes := total / 60
	seconds := total % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
