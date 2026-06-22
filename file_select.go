package flatte

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
)

// ErrNoSelection is returned by SelectFile when the selector command exits
// successfully without printing a selected path.
var ErrNoSelection = errors.New("flat: no file selected")

// FileSelection is the result of a terminal-delegated file picker.
type FileSelection struct {
	Path string
	Err  error
}

// SelectFile releases the terminal, runs cmd as an external file selector,
// restores the terminal, and applies fold with the selected path. The selected
// path is read from stdout and trimmed. If cmd already has Stdout, output is
// still forwarded there while also being captured for the selection result.
//
// This is intentionally terminal-delegated rather than an in-TUI file browser:
// apps can plug in fzf, yazi, ranger, or another command while Flatte owns the
// terminal handoff.
func SelectFile[S any](fx Effects[S], name string, cmd *exec.Cmd, fold func(*S, FileSelection)) {
	if fx.enqueue == nil || cmd == nil {
		return
	}
	var stdout bytes.Buffer
	if cmd.Stdout == nil {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, &stdout)
	}
	Exec(fx, name, cmd, func(s *S, err error) {
		selection := FileSelection{Err: err}
		if err == nil {
			selection.Path = strings.TrimSpace(stdout.String())
			if selection.Path == "" {
				selection.Err = ErrNoSelection
			}
		}
		fold(s, selection)
	})
}
