package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

const defaultTickInterval = 300 * time.Millisecond

var spinnerFrames = []string{"-", "\\", "|", "/"}

type State struct {
	ticks       int
	spinner     int
	waiting     bool
	modalOpen   bool
	modalInput  flatui.TextField
	modalResult string
}

func NewState() *State {
	return &State{}
}

func Init(s *State, fx flatte.Effects[State]) {
	if err := flatte.Every(fx, "modal.background.tick", tickInterval(), applyTick); err != nil {
		panic(err) // tickInterval is a sample invariant
	}
}

func applyTick(s *State, _ time.Time) {
	s.ticks++
	if s.waiting {
		s.spinner = (s.spinner + 1) % len(spinnerFrames)
	}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	if s.modalOpen {
		handleModal(s, key)
		return
	}

	switch key.Key {
	case flatte.KeyEnter:
		s.waiting = true
		s.modalOpen = true
		s.modalInput = flatui.TextField{}
	case flatte.KeyCharacter:
		if key.Rune == 'q' || key.Rune == 'Q' {
			fx.Quit()
		}
	}
}

func handleModal(s *State, key flatte.KeyEvent) {
	switch key.Key {
	case flatte.KeyCharacter:
		if handleAltWordKey(key, s.modalInput.MoveWordLeft, s.modalInput.MoveWordRight) {
			return
		}
		s.modalInput.Insert(key.Rune)
	case flatte.KeyBackspace:
		s.modalInput.Backspace()
	case flatte.KeyDelete:
		s.modalInput.Delete()
	case flatte.KeyLeft:
		s.modalInput.MoveLeft()
	case flatte.KeyRight:
		s.modalInput.MoveRight()
	case flatte.KeyEnter:
		s.modalOpen = false
		s.waiting = false
		s.modalResult = "accepted: " + s.modalInput.Value
	case flatte.KeyEscape:
		s.modalOpen = false
		s.waiting = false
		s.modalResult = "cancelled"
	}
}

func handleAltWordKey(key flatte.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatte.ModAlt) {
		return false
	}
	switch key.Rune {
	case 'b', 'B':
		moveLeft()
		return true
	case 'f', 'F':
		moveRight()
		return true
	}
	return false
}

// cardBox is the look flatui.Card gives string frames: a plain border in a
// muted grey.
var cardBox = layout.Box{Border: layout.BorderNormal, BorderColor: lipgloss.Color("240")}

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// View returns the frame as a layout tree. The runtime composes it at the
// window size and places the cursor at the focused Input's caret, so there
// is no cursor arithmetic here.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	loader := "idle"
	if s.waiting {
		loader = "waiting " + spinnerFrames[s.spinner%len(spinnerFrames)]
	}
	result := "none"
	if s.modalResult != "" {
		result = s.modalResult
	}

	children := []layout.Node{
		layout.Label{Text: "Flat Modal", Style: title},
		layout.Label{NodeBase: indent, Text: fmt.Sprintf("background ticks: %d | loader: %s", s.ticks, loader)},
		layout.Label{Text: "modal focus with background updates", Style: subtle},
		layout.Label{NodeBase: indent, Text: "background workspace:"},
		layout.NewSpacer(),
		layout.Label{NodeBase: indent, Text: "modal result: " + result},
		layout.Label{NodeBase: indent, Text: "recent events:\n  - async tick applied\n  - modal owns keyboard focus"},
		layout.Label{},
		layout.Label{Text: "enter open modal | q quit", Style: subtle},
	}
	if s.modalOpen {
		children = append(children,
			layout.Label{Text: "main view is waiting for modal input", Style: subtle},
			modal(s),
		)
	}
	return flatte.Frame{Root: layout.Col{
		NodeBase: layout.NodeBase{PadLeft: 2, PadRight: 2, Box: cardBox},
		Children: children,
	}}
}

func modal(s *State) layout.Node {
	return layout.Col{
		NodeBase: layout.NodeBase{Overlay: true, W: layout.Fixed(32), PadLeft: 2, PadRight: 2, Box: cardBox},
		Children: []layout.Node{
			layout.Label{Text: "Confirm Work", Style: title},
			layout.Label{Text: "modal captures input", Style: subtle},
			layout.Label{},
			layout.Row{
				NodeBase: layout.NodeBase{PadLeft: 2, Gap: 1},
				Children: []layout.Node{
					layout.Label{Text: "name:"},
					s.modalInput.Layout("modal.name", true),
				},
			},
			layout.Label{},
			layout.Label{Text: "enter confirm | esc cancel", Style: subtle},
		},
	}
}

func tickInterval() time.Duration {
	value := os.Getenv("FLAT_MODAL_INTERVAL")
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
	state := NewState()
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
