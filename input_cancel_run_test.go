package flatte

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

// scriptedReader is an uncancelable input source that can still deliver: Read
// parks until a chunk is pushed. Like blockingReader it is not an *os.File, so
// it takes the substrate's uncancelable fallback path on every platform.
type scriptedReader struct {
	chunks chan []byte
	rest   []byte
}

func newScriptedReader() *scriptedReader {
	return &scriptedReader{chunks: make(chan []byte, 4)}
}

func (r *scriptedReader) Read(p []byte) (int, error) {
	if len(r.rest) == 0 {
		chunk, ok := <-r.chunks
		if !ok {
			return 0, io.EOF
		}
		r.rest = chunk
	}
	n := copy(p, r.rest)
	r.rest = r.rest[n:]
	return n, nil
}

func (r *scriptedReader) send(t *testing.T, s string) {
	t.Helper()
	select {
	case r.chunks <- []byte(s):
	case <-time.After(2 * time.Second):
		t.Fatalf("input %q was never consumed", s)
	}
}

// Suspend releases the terminal and restores it, which stops and restarts the
// input pipeline. A pipeline that could not be stopped must be *reused* rather
// than replaced: its goroutine is still parked in Read on the source, so a
// second pipeline on top of it would race the parked one for the same bytes
// and the key pressed after resume would go to a reader nobody is listening
// to. This is what kept the exec/suspend/select-file tests failing on Windows
// after the cancel-join fix.
func TestRunReusesUncancelableInputAcrossSuspend(t *testing.T) {
	in := newScriptedReader()
	defer close(in.chunks)

	suspended := make(chan struct{})
	fakeSuspend := func() { close(suspended) }

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
		}, WithInput(in), WithOutput(&out), withSuspendProcess(fakeSuspend))
	}()

	in.send(t, "x")
	select {
	case <-suspended:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for suspendProcess")
	}

	// 'q' lands after the release/restore cycle: it can only be seen if the
	// surviving pipeline still feeds the loop.
	in.send(t, "q")
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("input after resume never reached the app: the uncancelable pipeline was replaced instead of reused")
	}
}

// blockingReader is an input source with no interruptible read: it parks in
// Read until released. It is deliberately not an *os.File, so the substrate
// falls back to an uncancelable reader whose Cancel reports false — the same
// shape a pipe takes on Windows, where the select/console cancel readers do
// not apply.
type blockingReader struct {
	release chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.release
	return 0, context.Canceled
}

// Run must not join the input goroutine when the read could not be cancelled:
// that read stays parked in the syscall until its source produces or closes,
// so waiting on it deadlocks Run instead of returning. This deadlocked every
// Run test on windows-latest.
func TestRunReturnsWhenInputReadCannotBeCancelled(t *testing.T) {
	in := &blockingReader{release: make(chan struct{})}
	// Release the parked read only after Run has returned, proving Run did
	// not depend on the read unblocking.
	defer close(in.release)

	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				fx.Quit()
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "quitting"}
			},
		}, WithInput(in), WithOutput(&out))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return: it waited on an input read that cannot be cancelled")
	}
}

// The same guarantee has to hold once the loop is running and the quit comes
// from a fold rather than from Init, which is the path that stops the pipeline
// through the deferred stop rather than the early return.
func TestRunReturnsOnQuitUpdateWithUncancelableInput(t *testing.T) {
	in := &blockingReader{release: make(chan struct{})}
	defer close(in.release)

	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				Go(fx, "quit.async",
					func(context.Context) (int, error) { return 0, nil },
					func(s *testState, _ int, _ error) { fx.Quit() },
				)
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "waiting"}
			},
		}, WithInput(in), WithOutput(&out))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return: it waited on an input read that cannot be cancelled")
	}
}
