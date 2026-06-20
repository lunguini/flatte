package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

type State struct {
	status  string
	path    string
	settled chan struct{}
}

func NewState() *State {
	return &State{status: "ready"}
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
	if !ok || key.Key != flat.KeyCharacter {
		return
	}
	switch key.Rune {
	case 'o', 'O':
		openSelector(s, fx)
	case 'q', 'Q':
		fx.Quit()
	}
}

func openSelector(s *State, fx flat.Effects[State]) {
	cmd, label, ok := selectorCommand()
	if !ok {
		s.status = "file selector unavailable"
		return
	}
	s.status = "running " + label + "..."
	flat.SelectFile(fx, "file.select", cmd, func(s *State, selection flat.FileSelection) {
		switch {
		case selection.Err == nil:
			s.path = selection.Path
			s.status = "selected"
		case errors.Is(selection.Err, flat.ErrNoSelection):
			s.status = "no selection"
		default:
			s.status = "selector: " + selection.Err.Error()
		}
		if s.settled != nil {
			close(s.settled)
			s.settled = nil
		}
	})
}

func selectorCommand() (*exec.Cmd, string, bool) {
	if configured := os.Getenv("FLAT_FILE_SELECTOR"); configured != "" {
		return shellCommand(configured), configured, true
	}
	if _, err := lookPath("fd"); err != nil {
		return nil, "", false
	}
	if _, err := lookPath("fzf"); err != nil {
		return nil, "", false
	}
	return shellCommand("fd . | fzf"), "fd . | fzf", true
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

var lookPath = exec.LookPath

func View(s *State, ctx flat.RenderContext) flat.Frame {
	path := s.path
	if path == "" {
		path = "(none)"
	}
	lines := []string{
		flatui.Title("Flat File Select"),
		flatui.Subtle("terminal-delegated selector"),
		"",
		"  status: " + s.status,
		"  selected: " + path,
		"",
		flatui.Subtle("o open selector | q quit"),
	}
	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
