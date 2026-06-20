package flat

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSelectFileCapturesTrimmedPathAndAppliesNamedFold(t *testing.T) {
	output, state, tracer, selection, label := runSelectFileApp(t, exec.Command("printf", "  ./README.md\n"))

	if selection.Err != nil {
		t.Fatalf("selection error = %v, want nil", selection.Err)
	}
	if selection.Path != "./README.md" {
		t.Fatalf("selection path = %q, want trimmed path", selection.Path)
	}
	if label != "./README.md" {
		t.Fatalf("label = %q, want selected path", label)
	}
	if state.count != 1 {
		t.Fatalf("state count = %d, want fold applied", state.count)
	}
	if !slices.Contains(tracer.updates, "pick") {
		t.Fatalf("traced updates = %v, want to include pick", tracer.updates)
	}
	assertTerminalReleasedAroundSelection(t, output)
}

func TestSelectFileReportsNoSelectionForEmptyOutput(t *testing.T) {
	_, state, _, selection, label := runSelectFileApp(t, exec.Command("true"))

	if !errors.Is(selection.Err, ErrNoSelection) {
		t.Fatalf("selection error = %v, want ErrNoSelection", selection.Err)
	}
	if selection.Path != "" {
		t.Fatalf("selection path = %q, want empty", selection.Path)
	}
	if label != "no selection" {
		t.Fatalf("label = %q, want no selection", label)
	}
	if state.count != 1 {
		t.Fatalf("state count = %d, want fold applied", state.count)
	}
}

func TestSelectFileReportsCommandError(t *testing.T) {
	_, state, _, selection, label := runSelectFileApp(t, exec.Command("false"))

	if selection.Err == nil || errors.Is(selection.Err, ErrNoSelection) {
		t.Fatalf("selection error = %v, want command error", selection.Err)
	}
	if selection.Path != "" {
		t.Fatalf("selection path = %q, want empty on command error", selection.Path)
	}
	if label != "error" {
		t.Fatalf("label = %q, want error", label)
	}
	if state.count != 1 {
		t.Fatalf("state count = %d, want fold applied", state.count)
	}
}

func runSelectFileApp(t *testing.T, cmd *exec.Cmd) (string, *testState, *recordingTracer, FileSelection, string) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	cmd.Stdin = strings.NewReader("")
	cmd.Stderr = &bytes.Buffer{}

	state := testState{}
	tracer := &recordingTracer{}
	var got FileSelection
	var label string
	folded := make(chan struct{})
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State:  &state,
			Tracer: tracer,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok || key.Key != KeyCharacter {
					return
				}
				switch key.Rune {
				case 'p':
					SelectFile(fx, "pick", cmd, func(s *testState, selection FileSelection) {
						got = selection
						s.count++
						switch {
						case selection.Err == nil:
							label = selection.Path
						case errors.Is(selection.Err, ErrNoSelection):
							label = "no selection"
						default:
							label = "error"
						}
						close(folded)
					})
				case 'q':
					fx.Quit()
				}
			},
			View: plainView,
		}, WithInput(reader), WithOutput(&out))
	}()

	if _, err := writer.Write([]byte("p")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-folded:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the file selection fold")
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
		t.Fatal("timed out waiting for Run after file selection")
	}
	return out.String(), &state, tracer, got, label
}

func assertTerminalReleasedAroundSelection(t *testing.T, output string) {
	t.Helper()
	enterAlt := strings.Index(output, "\x1b[?1049h")
	exitAlt := strings.Index(output, "\x1b[?1049l")
	reenterAlt := indexAfter(output, "\x1b[?1049h", exitAlt)
	if enterAlt == -1 || exitAlt < enterAlt || reenterAlt == -1 {
		t.Fatalf("expected enter -> exit -> re-enter alt screen around selection (enter=%d exit=%d reenter=%d):\n%q",
			enterAlt, exitAlt, reenterAlt, output)
	}
}
