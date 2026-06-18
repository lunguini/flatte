package flat

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type testState struct {
	count  int
	events []Key
}

type recordingTracer struct {
	events  []Event
	updates []string
}

func (t *recordingTracer) Event(ev Event) {
	t.events = append(t.events, ev)
}

// keyEvents filters the KeyEvents out of a traced event stream, so tests can
// assert on key input without depending on the initial ResizeEvent the loop
// delivers at startup.
func keyEvents(events []Event) []KeyEvent {
	var keys []KeyEvent
	for _, ev := range events {
		if key, ok := ev.(KeyEvent); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

func (t *recordingTracer) Update(name string) {
	t.updates = append(t.updates, name)
}

func TestNamedUpdateAppliesAndCarriesName(t *testing.T) {
	update := Named("counter.inc", func(s *testState) {
		s.count++
	})
	state := testState{}

	if update.Name() != "counter.inc" {
		t.Fatalf("Name() = %q, want %q", update.Name(), "counter.inc")
	}
	update.Apply(&state)

	if state.count != 1 {
		t.Fatalf("count = %d, want 1", state.count)
	}
}

func TestAsyncSendsNamedFoldedUpdate(t *testing.T) {
	ctx := context.Background()
	updates := make(chan StateUpdate[testState], 1)

	Async(ctx, updates, nil, "counter.load",
		func(context.Context) (int, error) {
			return 7, nil
		},
		func(s *testState, value int, err error) {
			s.count = value
		},
	)

	update := receiveCoreUpdate(t, updates)
	state := testState{}
	update.Apply(&state)

	if update.Name() != "counter.load" {
		t.Fatalf("Name() = %q, want %q", update.Name(), "counter.load")
	}
	if state.count != 7 {
		t.Fatalf("count = %d, want 7", state.count)
	}
}

func TestAsyncSuppressesCancelledUpdate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	updates := make(chan StateUpdate[testState], 1)

	Async(ctx, updates, nil, "counter.stale",
		func(context.Context) (int, error) {
			return 7, nil
		},
		func(s *testState, value int, err error) {
			s.count = value
		},
	)

	select {
	case update := <-updates:
		t.Fatalf("received cancelled update: %s", update.Name())
	case <-time.After(25 * time.Millisecond):
	}
}

func TestApplyUpdateTracesBeforeApply(t *testing.T) {
	var calls []string
	state := testState{}
	tracer := UpdateTracer(func(name string) {
		calls = append(calls, "trace:"+name)
	})
	update := Named("counter.inc", func(s *testState) {
		calls = append(calls, "apply")
		s.count++
	})

	ApplyUpdate(&state, tracer, update)

	if got := strings.Join(calls, ","); got != "trace:counter.inc,apply" {
		t.Fatalf("calls = %q, want trace before apply", got)
	}
}

// Frame diffing is owned by ultraviolet's TerminalRenderer since the Phase 3
// cutover; the Run-level guarantees it must uphold (one synchronized write
// per changed frame, zero bytes for identical frames) are asserted by the
// TestRunWrapsFramesInSynchronizedOutput / TestRunSkipsSyncMarkersFor-
// UnchangedFrames / TestRunCoalescesQueuedUpdatesIntoOneDraw tests below.

// Byte-level input decoding is owned by ultraviolet's TerminalReader since
// the Phase 3 cutover; the mapping onto the closed event set is specified by
// the table in translate_test.go, and end-to-end byte decoding is exercised
// by every Run-based test that writes input through a pipe.

func TestRunProcessesInputAndAsyncUpdates(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	tracer := &recordingTracer{}
	done := make(chan error, 1)
	var out bytes.Buffer
	ctx := context.Background()

	go func() {
		done <- Run(ctx, App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				Async(fx.Context, fx.Updates, nil, "counter.load",
					func(context.Context) (int, error) {
						return 3, nil
					},
					func(s *testState, value int, err error) {
						s.count = value
					},
				)
			},
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok {
					return
				}
				s.events = append(s.events, key.Key)
				if key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "count"}
			},
			Tracer: tracer,
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := writer.Write([]byte("jq")); err != nil {
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

	if state.count != 3 {
		t.Fatalf("count = %d, want async update applied", state.count)
	}
	if len(state.events) != 2 || state.events[0] != KeyCharacter || state.events[1] != KeyCharacter {
		t.Fatalf("events = %#v, want two character events", state.events)
	}
	if got := strings.Join(tracer.updates, ","); got != "counter.load" {
		t.Fatalf("updates = %q, want counter.load", got)
	}
	// The loop traces the initial ResizeEvent before any key input.
	if len(tracer.events) == 0 {
		t.Fatal("tracer recorded no events")
	}
	if first, ok := tracer.events[0].(ResizeEvent); !ok || first.Width != 72 || first.Height != 24 {
		t.Fatalf("first traced event = %#v, want initial ResizeEvent 72x24", tracer.events[0])
	}
	keys := keyEvents(tracer.events)
	if len(keys) != 2 || keys[0].Key != KeyCharacter || keys[1].Key != KeyCharacter {
		t.Fatalf("traced key events = %#v, want two character events", keys)
	}
}

func TestRunSkipsTerminalWritesForUnchangedFrames(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "unchanged"}
			},
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := writer.Write([]byte("jq")); err != nil {
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

	// Each draw is one synchronized-output block; the 'j' keystroke produced
	// an identical frame, which must not write anything at all.
	if got := strings.Count(out.String(), "\x1b[?2026h"); got != 1 {
		t.Fatalf("draw count = %d, want only the initial draw; output %q", got, out.String())
	}
}

func TestRunExitsWhenInitRequestsQuit(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				fx.Quit()
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "init quit"}
			},
		}, WithInput(reader), WithOutput(&out))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after Init requested quit")
	}
}

func TestEffectsQuitRequestsExit(t *testing.T) {
	var called bool
	fx := NewEffects[testState](context.Background(), nil, func() { called = true })

	fx.Quit()

	if !called {
		t.Fatal("Quit() did not invoke the quit callback")
	}
}

func TestEffectsQuitOnZeroValueIsNoop(t *testing.T) {
	var fx Effects[testState]
	fx.Quit() // must not panic
}

func TestRunDeliversCtrlCToAppWithoutDefaultQuit(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok {
					return
				}
				s.events = append(s.events, key.Key)
				if key.Key == KeyCtrlC {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame { return Frame{Content: "x"} },
		}, WithInput(reader), WithOutput(&out), WithoutDefaultQuit())
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := writer.Write([]byte{3}); err != nil {
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

	if len(state.events) != 1 || state.events[0] != KeyCtrlC {
		t.Fatalf("events = %#v, want app-delivered KeyCtrlC", state.events)
	}
}

func TestRunCoalescesQueuedUpdatesIntoOneDraw(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				// Three updates queued before the loop starts; they must
				// be applied in one batch and drawn once.
				for range 3 {
					fx.Updates <- Named("inc", func(s *testState) { s.count++ })
				}
			},
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: fmt.Sprintf("count:%d", s.count)}
			},
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(100 * time.Millisecond)
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

	if state.count != 3 {
		t.Fatalf("count = %d, want all three queued updates applied", state.count)
	}
	// The renderer writes cell-level deltas, so intermediate frame literals
	// can't be asserted on; the coalescing guarantee is the draw count: the
	// initial count:0 paint plus exactly ONE redraw for the whole batch.
	output := out.String()
	if got := strings.Count(output, "\x1b[?2026h"); got != 2 {
		t.Fatalf("draw count = %d, want initial + one coalesced redraw:\n%q", got, output)
	}
	if !strings.Contains(output, "count:0") {
		t.Fatalf("output missing initial pre-loop frame count:0:\n%q", output)
	}
}

func TestRunWrapsFramesInSynchronizedOutput(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok {
					return
				}
				if key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
				s.count++
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: fmt.Sprintf("count:%d", s.count)}
			},
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := writer.Write([]byte("xq")); err != nil {
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

	output := out.String()
	begin := strings.Count(output, "\x1b[?2026h")
	end := strings.Count(output, "\x1b[?2026l")
	if begin == 0 || begin != end {
		t.Fatalf("synchronized output markers begin=%d end=%d in %q", begin, end, output)
	}
}

func TestRunEntersAltScreenThroughRenderer(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame { return Frame{Content: "altscreen"} },
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
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

	// Alt-screen entry must be queued through the renderer — that call is
	// what also flips its fullscreen/absolute-cursor flags, and skipping it
	// desyncs the screen by one row on real terminals. Renderer-queued entry
	// is observable as the 1049h escape living INSIDE the first synchronized
	// block instead of preceding it.
	output := out.String()
	sync := strings.Index(output, "\x1b[?2026h")
	entry := strings.Index(output, "\x1b[?1049h")
	if sync != 0 {
		t.Fatalf("output must start with the first synchronized block, got %q", output)
	}
	if entry == -1 || entry < sync {
		t.Fatalf("alt-screen entry not queued through the renderer: entry=%d sync=%d in %q", entry, sync, output)
	}
	if !strings.HasSuffix(output, "\x1b[?1049l") {
		t.Fatalf("output must end with renderer-queued alt-screen exit, got %q", output)
	}
}

func TestRunSkipsSyncMarkersForUnchangedFrames(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame { return Frame{Content: "unchanged"} },
		}, WithInput(reader), WithOutput(&out))
	}()

	time.Sleep(50 * time.Millisecond)
	if _, err := writer.Write([]byte("xq")); err != nil {
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

	// Initial frame writes once; the unchanged frame after 'x' must
	// produce no bytes at all — not even empty sync markers.
	if got := strings.Count(out.String(), "\x1b[?2026h"); got != 1 {
		t.Fatalf("sync marker count = %d, want exactly 1 (initial frame only); output %q", got, out.String())
	}
}

func TestRunExitsWhenFoldRequestsQuit(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	done := make(chan error, 1)
	var out bytes.Buffer

	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Init: func(s *testState, fx Effects[testState]) {
				Go(fx, "quit.async",
					func(context.Context) (int, error) { return 0, nil },
					func(s *testState, _ int, _ error) { fx.Quit() },
				)
			},
			View: func(s *testState, ctx RenderContext) Frame { return Frame{Content: "x"} },
		}, WithInput(reader), WithOutput(&out))
	}()

	// No input is ever written: the only exit path is the fold's Quit.
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after a fold requested quit")
	}
}

func receiveCoreUpdate[S any](t *testing.T, updates <-chan StateUpdate[S]) StateUpdate[S] {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for update")
		return nil
	}
}
