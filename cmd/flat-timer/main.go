package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
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

func Init(s *State, fx flatte.Effects[State]) {
	if err := flatte.Every(fx, "timer.tick", tickInterval, applyTick); err != nil {
		panic(err) // tickInterval is a sample invariant
	}
}

func applyTick(s *State, _ time.Time) {
	s.timer.Tick(tickInterval)
	s.stopwatch.Tick(tickInterval)
	s.progress.SetPercent(s.timer.Percent())
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
	if key.Key == flatte.KeyEscape {
		fx.Quit()
		return
	}
	if key.Key != flatte.KeyCharacter {
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

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// barID names the node the progress bar fills.
const barID = "timer.bar"

// percentLabelWidth is the "  100%" Progress.View puts after the bar.
const percentLabelWidth = 6

// field is one "name: value" row with the names in a fixed-width column, so
// values line up in a proportional font too.
func field(name string, value layout.Node) layout.Node {
	return layout.Row{NodeBase: indent, Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(11)}, Text: name},
		value,
	}}
}

// View shows the progress bar as Progress.Layout: block characters in the
// terminal, a native bar in a window.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	stopState := "stopped"
	if s.stopwatch.Running() {
		stopState = "running"
	}
	timerState := "running"
	if s.timer.Done() {
		timerState = "done"
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Timer", Style: title},
		layout.Label{Text: "countdown + stopwatch sample", Style: subtle},
		layout.Label{},
		field("timer:", layout.Label{Text: fmt.Sprintf("%s remaining (%s)", formatDuration(s.timer.Remaining()), timerState)}),
		field("progress:", layout.Col{NodeBase: layout.NodeBase{ID: barID, W: layout.Grow(1)}, Children: []layout.Node{
			s.progress.Layout(),
		}}),
		field("stopwatch:", layout.Label{Text: fmt.Sprintf("%s (%s)", formatDuration(s.stopwatch.Elapsed()), stopState)}),
		layout.Label{},
		layout.Label{Text: "space start/stop | r reset all | t restart timer | q/esc quit", Style: subtle},
	)}
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
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	app := flatte.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Flat Timer", Width: 72, Height: 20})
	} else {
		err = flatte.Run(context.Background(), app)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
