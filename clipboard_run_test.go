package flat

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestRunSetClipboardEmitsOSC52(t *testing.T) {
	output := runClipboardApp(t, func(s *testState, fx Effects[testState]) {
		fx.SetClipboard("hi")
	}, "xq", nil)

	if got := strings.Count(output, ansi.SetSystemClipboard("hi")); got != 1 {
		t.Fatalf("OSC52 write emitted %d time(s), want 1:\n%q", got, output)
	}
}

func TestRunSetClipboardThenQuitStillWrites(t *testing.T) {
	output := runClipboardApp(t, func(s *testState, fx Effects[testState]) {
		fx.SetClipboard("bye")
		fx.Quit()
	}, "x", nil)

	if !strings.Contains(output, ansi.SetSystemClipboard("bye")) {
		t.Fatalf("clipboard write enqueued before quit was dropped:\n%q", output)
	}
}

func TestRunReadClipboardRoundTrip(t *testing.T) {
	var clip string
	output := runClipboardApp(t, func(s *testState, fx Effects[testState]) {
		fx.ReadClipboard()
	}, "x", func(s *testState, ev Event, fx Effects[testState], writer *os.File) {
		if c, ok := ev.(ClipboardEvent); ok {
			clip = c.Text
			fx.Quit()
		}
		// After the request goes out, answer like a supporting terminal:
		// OSC52 response with base64("hello").
		if key, ok := ev.(KeyEvent); ok && key.Rune == 'x' {
			go func() {
				time.Sleep(20 * time.Millisecond)
				_, _ = writer.Write([]byte("\x1b]52;c;aGVsbG8=\x07"))
			}()
		}
	})

	if !strings.Contains(output, "\x1b]52;c;?\x07") {
		t.Fatalf("clipboard read request not emitted:\n%q", output)
	}
	if clip != "hello" {
		t.Fatalf("ClipboardEvent.Text = %q, want hello (decoded by the substrate)", clip)
	}
}

// runClipboardApp runs an app whose 'x' key triggers onX. The optional
// extra handler sees every event plus the input writer (for tests that
// fake a terminal response).
func runClipboardApp(t *testing.T, onX func(*testState, Effects[testState]), input string, extra func(*testState, Event, Effects[testState], *os.File)) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if extra != nil {
					extra(s, ev, fx, writer)
				}
				key, ok := ev.(KeyEvent)
				if !ok || key.Key != KeyCharacter {
					return
				}
				switch key.Rune {
				case 'x':
					onX(s, fx)
				case 'q':
					fx.Quit()
				}
			},
			View: plainView,
		}, WithInput(reader), WithOutput(&out))
	}()

	if _, err := writer.Write([]byte(input)); err != nil {
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
	return out.String()
}
