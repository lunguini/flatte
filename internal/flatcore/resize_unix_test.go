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
	tracer := &recordingTracer{}
	done := make(chan error, 1)
	sawResize := make(chan struct{}, 2)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				switch ev := ev.(type) {
				case ResizeEvent:
					select {
					case sawResize <- struct{}{}:
					default:
					}
				case KeyEvent:
					if ev.Key == KeyCharacter && ev.Rune == 'q' {
						fx.Quit()
					}
				}
			},
			View:   func(s *testState, ctx RenderContext) string { return "x" },
			Tracer: tracer,
		}, WithInput(reader), WithOutput(&out))
	}()

	// The loop delivers an initial ResizeEvent before the first draw; by the
	// time it arrives the SIGWINCH source is already registered.
	select {
	case <-sawResize:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial resize event")
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	// Wait until the SIGWINCH resize has been handled before sending 'q', so
	// the events-vs-resize select ordering cannot flake.
	select {
	case <-sawResize:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SIGWINCH resize event")
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

	var resizes []ResizeEvent
	for _, ev := range tracer.events {
		if resize, ok := ev.(ResizeEvent); ok {
			resizes = append(resizes, resize)
		}
	}
	if len(resizes) < 2 {
		t.Fatalf("traced resize events = %#v, want initial + SIGWINCH", resizes)
	}
	for _, resize := range resizes {
		// Output is a bytes.Buffer, not a terminal: sizes are the fallback.
		if resize.Width != 72 || resize.Height != 24 {
			t.Fatalf("resize event = %#v, want fallback 72x24", resize)
		}
	}
}
