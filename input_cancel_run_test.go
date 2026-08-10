package flatte

import (
	"bytes"
	"context"
	"testing"
	"time"
)

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
