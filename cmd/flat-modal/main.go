package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
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
	flatte.Every(fx, "modal.background.tick", tickInterval(), applyTick)
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

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	base := viewMain(s, ctx)
	if !s.modalOpen {
		return flatte.Frame{Content: base}
	}
	modal := viewModal(s, ctx)
	frame := flatte.Frame{Content: flatui.Overlay(base, modal)}
	overlayX, overlayY := flatui.OverlayOrigin(base, modal)
	cardX, cardY := flatui.CardOrigin()
	frame.Cursor = &flatte.Cursor{
		X: overlayX + cardX + lipgloss.Width("  name: ") + s.modalInput.CursorColumn(),
		Y: overlayY + cardY + 3, // title, subtle, blank precede the name row
	}
	return frame
}

func viewMain(s *State, ctx flatte.RenderContext) string {
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

func viewModal(s *State, ctx flatte.RenderContext) string {
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
