package flatte

import (
	"bytes"
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestQuitRequestsTurnCtrlCIntoARequest: with WithQuitRequests, the default
// Ctrl+C quit reaches the app as a QuitRequest and quits only when the app
// says so.
func TestQuitRequestsTurnCtrlCIntoARequest(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var requests atomic.Int32
	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if _, ok := ev.(QuitRequest); ok {
					// The first request is declined; the second is accepted.
					if requests.Add(1) == 2 {
						fx.Quit()
					}
				}
			},
			View: func(s *testState, ctx RenderContext) Frame { return Frame{Content: "x"} },
		}, WithInput(reader), WithOutput(&out), WithQuitRequests())
	}()

	if _, err := writer.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		t.Fatal("Ctrl-C quit although the app declined the request")
	case <-time.After(200 * time.Millisecond):
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
	if _, err := writer.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted quit request did not end Run")
	}
}

func TestSessionReportsQuitRequests(t *testing.T) {
	app := App[testState]{State: &testState{}, View: func(*testState, RenderContext) Frame { return Frame{} }}
	if newSession(context.Background(), app).QuitRequests() {
		t.Fatal("a session without the option reports quit requests")
	}
	config := applyRunOptions(nativeRunConfig(), WithQuitRequests())
	if !config.config.quitRequests {
		t.Fatal("option not applied")
	}
}
