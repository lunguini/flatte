package flatcore

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunSuspendReleasesAndRestoresTerminal(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	suspended := make(chan struct{})
	suspendCalls := 0
	fakeSuspend := func() {
		suspendCalls++
		close(suspended)
	}

	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok || key.Key != KeyCharacter {
					return
				}
				switch key.Rune {
				case 'x':
					fx.Suspend()
				case 'q':
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "suspendable"}
			},
		}, WithInput(reader), WithOutput(&out), withSuspendProcess(fakeSuspend))
	}()

	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-suspended:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for suspendProcess")
	}
	// 'q' goes through the RESTARTED input pipeline: quitting proves the
	// reader survived the release/restore cycle.
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run after resume")
	}

	if suspendCalls != 1 {
		t.Fatalf("suspendProcess called %d time(s), want 1", suspendCalls)
	}
	output := out.String()
	enterAlt := strings.Index(output, "\x1b[?1049h")
	exitAlt := strings.Index(output, "\x1b[?1049l")
	reenterAlt := indexAfter(output, "\x1b[?1049h", exitAlt)
	if enterAlt == -1 || exitAlt < enterAlt || reenterAlt == -1 {
		t.Fatalf("expected enter -> exit -> re-enter alt screen (enter=%d exit=%d reenter=%d):\n%q",
			enterAlt, exitAlt, reenterAlt, output)
	}
	if got := strings.Count(output, "suspendable"); got < 2 {
		t.Fatalf("frame painted %d time(s), want initial + post-resume repaint:\n%q", got, output)
	}
	// The release must restore the cursor for the shell (a show before the
	// alt-screen re-entry; the resume hides it again for the repaint).
	if show := indexAfter(output, "\x1b[?25h", enterAlt); show == -1 || show > reenterAlt {
		t.Fatalf("cursor not restored for the shell during suspend:\n%q", output)
	}
}
