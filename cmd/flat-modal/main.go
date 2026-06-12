package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
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

func Init(s *State, fx flatcore.Effects[State]) {
	flatcore.Every(fx, "modal.background.tick", tickInterval(), applyTick)
}

func applyTick(s *State, _ time.Time) {
	s.ticks++
	if s.waiting {
		s.spinner = (s.spinner + 1) % len(spinnerFrames)
	}
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	key, ok := ev.(flatcore.KeyEvent)
	if !ok {
		return
	}
	if s.modalOpen {
		handleModal(s, key)
		return
	}

	switch key.Key {
	case flatcore.KeyEnter:
		s.waiting = true
		s.modalOpen = true
		s.modalInput = flatui.TextField{}
	case flatcore.KeyCharacter:
		if key.Rune == 'q' || key.Rune == 'Q' {
			fx.Quit()
		}
	}
}

func handleModal(s *State, key flatcore.KeyEvent) {
	switch key.Key {
	case flatcore.KeyCharacter:
		s.modalInput.Insert(key.Rune)
	case flatcore.KeyBackspace:
		s.modalInput.Backspace()
	case flatcore.KeyDelete:
		s.modalInput.Delete()
	case flatcore.KeyLeft:
		s.modalInput.MoveLeft()
	case flatcore.KeyRight:
		s.modalInput.MoveRight()
	case flatcore.KeyEnter:
		s.modalOpen = false
		s.waiting = false
		s.modalResult = "accepted: " + s.modalInput.Value
	case flatcore.KeyEscape:
		s.modalOpen = false
		s.waiting = false
		s.modalResult = "cancelled"
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	base := viewMain(s, ctx)
	if !s.modalOpen {
		return flatcore.Frame{Content: base}
	}
	modal := viewModal(s, ctx)
	frame := flatcore.Frame{Content: flatui.Overlay(base, modal)}
	overlayX, overlayY := flatui.OverlayOrigin(base, modal)
	cardX, cardY := flatui.CardOrigin()
	frame.Cursor = &flatcore.Cursor{
		X: overlayX + cardX + lipgloss.Width("  name: ") + s.modalInput.CursorColumn(),
		Y: overlayY + cardY + 3, // title, subtle, blank precede the name row
	}
	return frame
}

func viewMain(s *State, ctx flatcore.RenderContext) string {
	loader := "idle"
	if s.waiting {
		loader = "waiting " + spinnerFrames[s.spinner%len(spinnerFrames)]
	}

	result := "none"
	if s.modalResult != "" {
		result = s.modalResult
	}

	lines := []string{
		flatui.Title("Flat Modal"),
		fmt.Sprintf("  background ticks: %d | loader: %s", s.ticks, loader),
		flatui.Subtle("modal focus with background updates"),
		"  background workspace:",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"  modal result: " + result,
		"  recent events:",
		"    - async tick applied",
		"    - modal owns keyboard focus",
		"",
		flatui.Subtle("enter open modal | q quit"),
	}
	if s.modalOpen {
		lines = append(lines, flatui.Subtle("main view is waiting for modal input"))
	}
	return flatui.Card(lines, ctx.Width)
}

func viewModal(s *State, ctx flatcore.RenderContext) string {
	lines := []string{
		flatui.Title("Confirm Work"),
		flatui.Subtle("modal captures input"),
		"",
		"  name: " + s.modalInput.Value,
		"",
		flatui.Subtle("enter confirm | esc cancel"),
	}
	return flatui.Card(lines, min(ctx.Width, 32))
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
