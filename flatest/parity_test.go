package flatest

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lunguini/flatte"
)

// parityApp is a synchronous counter: '+' increments, 'q' quits. No async
// or timers, so real Run and the Driver must reach identical final state
// for the same key sequence — a guard against Driver/Run sequencing drift.
func parityApp(state *counter) flatte.App[counter] {
	return flatte.App[counter]{
		State: state,
		Handle: func(s *counter, ev flatte.Event, fx flatte.Effects[counter]) {
			if k, ok := ev.(flatte.KeyEvent); ok && k.Key == flatte.KeyCharacter {
				switch k.Rune {
				case '+':
					s.n++
				case 'q':
					fx.Quit()
				}
			}
		},
		View: func(s *counter, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "n=" + strconv.Itoa(s.n)}
		},
	}
}

func TestDriverRunParityOnSyncApp(t *testing.T) {
	const script = "++x+" // 'x' is ignored; three increments

	// --- real Run, driven over a pipe ---
	runState := &counter{}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flatte.Run(context.Background(), parityApp(runState),
			flatte.WithInput(reader), flatte.WithOutput(&out))
	}()
	if _, err := writer.Write([]byte(script + "q")); err != nil {
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

	// --- Driver, same script (Run's off-TTY width is the 72 fallback) ---
	d := Start(parityApp(&counter{}), 72)
	for _, r := range script {
		d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r})
	}

	if runState.n != d.State().n {
		t.Fatalf("final state diverged: Run n=%d, Driver n=%d", runState.n, d.State().n)
	}
	runFrame := parityApp(runState).View(runState, flatte.RenderContext{Width: 72})
	if runFrame.Content != d.Frame().Content {
		t.Fatalf("final frame diverged: Run %q, Driver %q", runFrame.Content, d.Frame().Content)
	}
}

func burstParityApp(state *counter, settled chan struct{}) flatte.App[counter] {
	return flatte.App[counter]{
		State: state,
		Init: func(_ *counter, fx flatte.Effects[counter]) {
			for range 3 {
				fx.Send(flatte.Named("inc", func(s *counter) {
					s.n++
					if s.n == 3 {
						close(settled)
					}
				}))
			}
		},
		Handle: func(_ *counter, ev flatte.Event, fx flatte.Effects[counter]) {
			if key, ok := ev.(flatte.KeyEvent); ok && key.Key == flatte.KeyCharacter && key.Rune == 'q' {
				fx.Quit()
			}
		},
		View: func(s *counter, _ flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "n=" + strconv.Itoa(s.n)}
		},
	}
}

// Run coalesces only a bounded snapshot of queued updates per paint, while the
// deterministic Driver drains synchronously to completion. Their observation
// cadence deliberately differs, but a finite burst must still converge to the
// same state and frame.
func TestDriverRunParityAfterQueuedUpdateBurst(t *testing.T) {
	runState := &counter{}
	runSettled := make(chan struct{})
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() {
		done <- flatte.Run(context.Background(), burstParityApp(runState, runSettled),
			flatte.WithInput(reader), flatte.WithOutput(&bytes.Buffer{}))
	}()

	select {
	case <-runSettled:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not apply queued update burst")
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

	driverState := &counter{}
	driverSettled := make(chan struct{})
	d := Start(burstParityApp(driverState, driverSettled), 72)
	select {
	case <-driverSettled:
	default:
		t.Fatal("Driver did not synchronously drain queued update burst")
	}

	if runState.n != d.State().n || d.State().n != 3 {
		t.Fatalf("final state diverged: Run n=%d, Driver n=%d", runState.n, d.State().n)
	}
	runFrame := burstParityApp(runState, make(chan struct{})).View(runState, flatte.RenderContext{Width: 72})
	if runFrame != d.Frame() {
		t.Fatalf("final frame diverged: Run %#v, Driver %#v", runFrame, d.Frame())
	}
}
