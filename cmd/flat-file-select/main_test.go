package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestSelectorCommandUsesConfiguredShellCommand(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "printf ./chosen.txt")

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want true")
	}
	if label != "printf ./chosen.txt" {
		t.Fatalf("label = %q, want configured command", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "printf ./chosen.txt") {
		t.Fatalf("cmd args = %q, want configured command", got)
	}
}

func TestSelectorCommandFallsBackToFDAndFZFOnlyWhenPresent(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restore := stubLookPath(func(name string) (string, error) {
		if name == "fd" || name == "fzf" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	defer restore()

	_, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want true")
	}
	if label != "fd . | fzf" {
		t.Fatalf("label = %q, want fallback pipeline", label)
	}
}

func TestOpenSelectorReportsMissingSelector(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restore := stubLookPath(func(string) (string, error) {
		return "", errors.New("missing")
	})
	defer restore()

	s := NewState()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'o'}, flat.Effects[State]{})

	if s.status != "file selector unavailable" {
		t.Fatalf("status = %q, want unavailable", s.status)
	}
}

func TestOpenSelectorCapturesSelectedPathThroughRun(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "printf ' ./picked.txt\n'")
	state := NewState()
	settled := make(chan struct{})
	state.settled = settled
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flat.Run(context.Background(), flat.App[State]{
			State:  state,
			Handle: Handle,
			View:   View,
		}, flat.WithInput(reader), flat.WithOutput(&out))
	}()

	if _, err := writer.Write([]byte("o")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-settled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for selected state")
	}
	if state.path != "./picked.txt" {
		t.Fatalf("path = %q, want ./picked.txt", state.path)
	}

	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run")
	}
}

func TestInitialFrame(t *testing.T) {
	frame := View(NewState(), flat.RenderContext{Width: 72})
	clean := flatest.CleanFrame(frame.Content)
	for _, want := range []string{"Flat File Select", "status: ready", "selected: (none)", "o open selector"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("frame missing %q:\n%s", want, clean)
		}
	}
}

func stubLookPath(fn func(string) (string, error)) func() {
	previous := lookPath
	lookPath = fn
	return func() { lookPath = previous }
}
