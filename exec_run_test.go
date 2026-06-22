package flatte

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunExecReleasesTerminalAndAppliesNamedFold(t *testing.T) {
	output, state, tracer, foldErr := runExecApp(t, exec.Command("true"))

	if state.count != 7 {
		t.Fatalf("count = %d, want 7 (exec fold applied)", state.count)
	}
	if foldErr != nil {
		t.Fatalf("fold error = %v, want nil for `true`", foldErr)
	}
	if !slices.Contains(tracer.updates, "edit") {
		t.Fatalf("traced updates = %v, want to include \"edit\"", tracer.updates)
	}
	enterAlt := strings.Index(output, "\x1b[?1049h")
	exitAlt := strings.Index(output, "\x1b[?1049l")
	reenterAlt := indexAfter(output, "\x1b[?1049h", exitAlt)
	if enterAlt == -1 || exitAlt < enterAlt || reenterAlt == -1 {
		t.Fatalf("expected enter -> exit -> re-enter alt screen around exec (enter=%d exit=%d reenter=%d):\n%q",
			enterAlt, exitAlt, reenterAlt, output)
	}
}

func TestRunExecDeliversCommandError(t *testing.T) {
	_, state, _, foldErr := runExecApp(t, exec.Command("false"))

	if foldErr == nil {
		t.Fatal("fold error = nil, want exit error from `false`")
	}
	if state.count != 7 {
		t.Fatalf("count = %d, want 7 (fold still applied on error)", state.count)
	}
}

func runExecApp(t *testing.T, cmd *exec.Cmd) (string, *testState, *recordingTracer, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	// The command must not inherit the input pipe (it would steal bytes);
	// tests give it explicit empty stdio.
	cmd.Stdin = strings.NewReader("")
	var cmdOut bytes.Buffer
	cmd.Stdout = &cmdOut
	cmd.Stderr = &cmdOut

	state := testState{}
	tracer := &recordingTracer{}
	var foldErr error
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
				case 'x':
					Exec(fx, "edit", cmd, func(s *testState, err error) {
						s.count = 7
						foldErr = err
						close(folded)
					})
				case 'q':
					fx.Quit()
				}
			},
			View: plainView,
		}, WithInput(reader), WithOutput(&out))
	}()

	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-folded:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the exec fold")
	}
	// 'q' rides the restarted input pipeline.
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run after exec")
	}
	return out.String(), &state, tracer, foldErr
}
