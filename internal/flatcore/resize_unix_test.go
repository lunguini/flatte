//go:build unix

package flatcore

import (
	"bytes"
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRunDeliversResizeEvent(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	sawResize := make(chan struct{}, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				s.events = append(s.events, ev.Key)
				if ev.Key == KeyResize {
					select {
					case sawResize <- struct{}{}:
					default:
					}
				}
				if ev.Key == KeyCharacter && ev.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) string { return "x" },
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	// Wait until the resize event has been handled before sending 'q', so
	// the events-vs-resize select ordering cannot flake.
	select {
	case <-sawResize:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resize event")
	}
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Run")
	}

	if len(state.events) < 2 || state.events[0] != KeyResize {
		t.Fatalf("events = %#v, want KeyResize first", state.events)
	}
}
