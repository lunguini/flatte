package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

const (
	defaultStreamInterval = 700 * time.Millisecond
	maxVisibleEvents      = 6
)

type StreamEvent struct {
	Level   string
	Message string
	Done    bool
}

type State struct {
	events []StreamEvent
	done   bool
}

var defaultEvents = []StreamEvent{
	{Level: "info", Message: "queued deploy"},
	{Level: "info", Message: "building assets"},
	{Level: "ok", Message: "tests passed"},
	{Level: "warn", Message: "cache miss: regenerating"},
	{Level: "ok", Message: "deployed"},
}

func NewState() *State { return &State{} }

func Init(s *State, fx flat.Effects[State]) {
	flat.Stream(fx, "stream.event", streamSource(defaultEvents, streamInterval()), applyStreamEvent)
}

func streamSource(events []StreamEvent, interval time.Duration) func(context.Context, func(StreamEvent)) {
	return func(ctx context.Context, send func(StreamEvent)) {
		for _, ev := range events {
			if ctx.Err() != nil {
				return
			}
			if interval > 0 {
				timer := time.NewTimer(interval)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			send(ev)
		}
		if ctx.Err() == nil {
			send(StreamEvent{Done: true})
		}
	}
}

func applyStreamEvent(s *State, ev StreamEvent) {
	if ev.Done {
		s.done = true
		return
	}
	s.events = append(s.events, ev)
	if len(s.events) > maxVisibleEvents {
		s.events = s.events[len(s.events)-maxVisibleEvents:]
	}
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
	if !ok || key.Key != flat.KeyCharacter {
		return
	}
	switch key.Rune {
	case 'c', 'C':
		s.events = nil
		s.done = false
	case 'q', 'Q':
		fx.Quit()
	}
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	status := "streaming"
	if s.done {
		status = "complete"
	}
	lines := []string{
		flatui.Title("Flat Stream"),
		flatui.Subtle("flat.Stream dogfood"),
		"",
		"  status: " + status,
		"",
	}
	if len(s.events) == 0 {
		lines = append(lines, "  (waiting for events)")
	} else {
		for _, ev := range s.events {
			lines = append(lines, fmt.Sprintf("  [%s] %s", ev.Level, ev.Message))
		}
	}
	lines = append(lines, "", flatui.Subtle("c clear | q quit"))
	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func streamInterval() time.Duration {
	value := os.Getenv("FLAT_STREAM_INTERVAL")
	if strings.TrimSpace(value) == "" {
		return defaultStreamInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval < 0 {
		return defaultStreamInterval
	}
	return interval
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
