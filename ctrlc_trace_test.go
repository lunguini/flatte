package flatte

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// The default-quit Ctrl-C must be traced before the loop exits, or the
// recorder/replayer never sees the exit-causing event (audit/replay gap).
func TestDefaultQuitCtrlCIsTraced(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	tracer := &recordingTracer{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State:  &state,
			View:   func(s *testState, ctx RenderContext) Frame { return Frame{Content: "x"} },
			Tracer: tracer,
		}, WithInput(reader), WithOutput(&out))
	}()

	if _, err := writer.Write([]byte{3}); err != nil { // Ctrl-C
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Ctrl-C to quit")
	}

	var sawCtrlC bool
	for _, ev := range tracer.events {
		if key, ok := ev.(KeyEvent); ok && key.Key == KeyCtrlC {
			sawCtrlC = true
		}
	}
	if !sawCtrlC {
		t.Fatalf("tracer never saw the exit-causing Ctrl-C: %#v", tracer.events)
	}
}
