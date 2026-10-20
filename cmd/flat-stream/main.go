package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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

func Init(s *State, fx flatte.Effects[State]) {
	flatte.Stream(fx, "stream.event", streamSource(defaultEvents, streamInterval()), applyStreamEvent)
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

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok || key.Key != flatte.KeyCharacter {
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

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	status := "streaming"
	if s.done {
		status = "complete"
	}
	children := []layout.Node{
		layout.Label{Text: "Flat Stream", Style: title},
		layout.Label{Text: "flatte.Stream dogfood", Style: subtle},
		layout.Label{},
		layout.Label{NodeBase: indent, Text: "status: " + status},
		layout.Label{},
	}
	if len(s.events) == 0 {
		children = append(children, layout.Label{NodeBase: indent, Text: "(waiting for events)"})
	} else {
		for _, ev := range s.events {
			children = append(children, layout.Label{NodeBase: indent, Text: fmt.Sprintf("[%s] %s", ev.Level, ev.Message)})
		}
	}
	children = append(children, layout.Label{}, layout.Label{Text: "c clear | q quit", Style: subtle})
	return flatte.Frame{Root: flatui.CardLayout(children...)}
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
