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

const defaultProgressInterval = 120 * time.Millisecond

type State struct {
	progress flatui.Progress
	paused   bool
}

func NewState() *State {
	return &State{progress: flatui.NewProgress(24)}
}

func Init(s *State, fx flatte.Effects[State]) {
	if err := flatte.Every(fx, "progress.tick", progressInterval(), applyTick); err != nil {
		panic(err) // progressInterval is a sample invariant
	}
}

func applyTick(s *State, _ time.Time) {
	if s.paused {
		return
	}
	s.progress.SetPercent(s.progress.Percent() + 10)
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.LayoutEvent:
		// The bar fills the rect the frame solved for it, in either host.
		if r, ok := ev.Rects[barID]; ok {
			s.progress.SetWidth(max(r.W-percentLabelWidth, 0))
		}
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
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

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// barID names the node the progress bar fills.
const barID = "progress.bar"

// percentLabelWidth is the "  100%" Progress.View puts after the bar.
const percentLabelWidth = 6

// View shows the bar as a Text leaf: its block characters are cells (a
// window draws them as a cell grid).
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	status := "running"
	if s.paused {
		status = "paused"
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Progress", Style: title},
		layout.Label{Text: "horizontal loader sample", Style: subtle},
		layout.Label{},
		layout.Row{NodeBase: indent, Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: barID, W: layout.Grow(1)}, Children: []layout.Node{
				layout.Text{String: s.progress.View()},
			}},
		}},
		layout.Label{NodeBase: indent, Text: "state: " + status},
		layout.Label{},
		layout.Label{Text: "space pause | r reset | q quit", Style: subtle},
	)}
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
