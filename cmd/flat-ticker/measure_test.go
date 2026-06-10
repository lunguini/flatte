package main

// Output-volume measurement for the no-frame-rate-cap experiment: how many
// bytes does the real Run loop emit at high tick frequencies?
//
// This is NOT part of the normal test suite — it sleeps for seconds and its
// numbers are timing-dependent, so it is gated behind an env var and makes no
// timing assertions. Run it manually:
//
//	GOCACHE=$PWD/.cache/go-build FLAT_TICKER_MEASURE=1 \
//	  go test -v -run TestMeasureOutputVolume ./cmd/flat-ticker/
//
// Accounting: each draw is exactly one buffered Write (run.go's draw closure
// uses a single Fprintf). Run additionally emits one 14-byte setup write
// ("\x1b[?1049h\x1b[?25l") and one 14-byte teardown write
// ("\x1b[?25h\x1b[?1049l"); both are subtracted below. The first draw is a
// full-screen paint; every later draw is a single-row rewrite of the changed
// ticks row wrapped in synchronized-output markers.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
)

// countingWriter counts Write calls and bytes. All writes happen on the Run
// loop goroutine; the test reads the totals only after Run has returned (the
// done channel provides the happens-before edge), so no locking is needed.
type countingWriter struct {
	writes     int
	bytes      int
	firstSizes []int // first few write sizes, to characterize payloads
	lastSize   int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	w.bytes += len(p)
	if len(w.firstSizes) < 4 {
		w.firstSizes = append(w.firstSizes, len(p))
	}
	w.lastSize = len(p)
	return len(p), nil
}

func TestMeasureOutputVolume(t *testing.T) {
	if os.Getenv("FLAT_TICKER_MEASURE") == "" {
		t.Skip("measurement run only; set FLAT_TICKER_MEASURE=1 to enable")
	}

	const window = 2 * time.Second
	const controlWrites = 2  // alt-screen/cursor setup + teardown
	const controlBytes = 28 // 14 bytes each

	intervals := []time.Duration{
		1 * time.Millisecond,
		5 * time.Millisecond,
		16 * time.Millisecond,
		100 * time.Millisecond,
	}

	t.Logf("interval | window | ticks | draws | total bytes | draws/s | bytes/s | ticks/draw")
	for _, interval := range intervals {
		t.Run(interval.String(), func(t *testing.T) {
			t.Setenv("FLAT_TICKER_INTERVAL", interval.String())

			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			defer reader.Close()
			defer writer.Close()

			state := &State{}
			out := &countingWriter{}
			done := make(chan error, 1)
			go func() {
				done <- flatcore.Run(context.Background(), flatcore.App[State]{
					State:  state,
					Init:   Init,
					Handle: Handle,
					View:   View,
				}, flatcore.WithInput(reader), flatcore.WithOutput(out))
			}()

			time.Sleep(window)
			if _, err := writer.Write([]byte("q")); err != nil {
				t.Fatalf("write quit: %v", err)
			}
			if err := <-done; err != nil {
				t.Fatalf("Run: %v", err)
			}

			// Run has returned: the loop goroutine is gone, state and the
			// writer are safe to read directly.
			draws := out.writes - controlWrites
			drawBytes := out.bytes - controlBytes
			secs := window.Seconds()
			ticksPerDraw := 0.0
			if draws > 0 {
				ticksPerDraw = float64(state.ticks) / float64(draws)
			}
			t.Logf("%8s | %6s | %5d | %5d | %11d | %7.1f | %7.0f | %10.2f",
				interval, window, state.ticks, draws, drawBytes,
				float64(draws)/secs, float64(drawBytes)/secs, ticksPerDraw)
			t.Logf("  write sizes: first writes %v (setup, full first draw, then row rewrites), last (teardown) %d",
				out.firstSizes, out.lastSize)
		})
	}
}
