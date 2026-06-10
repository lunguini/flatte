package flatcore

import (
	"bufio"
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

	Async(ctx, updates, "counter.load",
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

	Async(ctx, updates, "counter.stale",
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

func TestDrawNormalizesLineEndingsForRawTerminalMode(t *testing.T) {
	var out bytes.Buffer

	Draw(&out, "top\nmiddle\nbottom")

	got := out.String()
	if strings.Contains(got, "top\nmiddle") {
		t.Fatalf("draw output contains bare line feeds: %q", got)
	}
	if !strings.Contains(got, "top\r\nmiddle\r\nbottom") {
		t.Fatalf("draw output missing CRLF-normalized frame: %q", got)
	}
}

func TestDiffRendererWritesFullFrameOnFirstDraw(t *testing.T) {
	var out bytes.Buffer
	renderer := NewDiffRenderer()

	renderer.Draw(&out, "top\nbottom", RenderContext{Width: 24})

	got := out.String()
	if !strings.HasPrefix(got, "\x1b[H\x1b[2J") {
		t.Fatalf("first draw = %q, want full redraw prefix", got)
	}
	if !strings.Contains(got, "top\r\nbottom") {
		t.Fatalf("first draw = %q, want CRLF-normalized frame", got)
	}
}

func TestDiffRendererSkipsIdenticalFrame(t *testing.T) {
	var out bytes.Buffer
	renderer := NewDiffRenderer()

	renderer.Draw(&out, "same\nframe", RenderContext{Width: 24})
	out.Reset()
	renderer.Draw(&out, "same\nframe", RenderContext{Width: 24})

	if out.Len() != 0 {
		t.Fatalf("identical frame wrote %q, want no output", out.String())
	}
}

func TestDiffRendererRewritesOnlyChangedRows(t *testing.T) {
	var out bytes.Buffer
	renderer := NewDiffRenderer()

	renderer.Draw(&out, "title\nloading -\nfooter", RenderContext{Width: 24})
	out.Reset()
	renderer.Draw(&out, "title\nloading \\\nfooter", RenderContext{Width: 24})

	got := out.String()
	if got != "\x1b[2;1H\x1b[2Kloading \\" {
		t.Fatalf("changed row output = %q, want only row 2 rewrite", got)
	}
}

func TestDiffRendererClearsRowsRemovedFromShorterFrame(t *testing.T) {
	var out bytes.Buffer
	renderer := NewDiffRenderer()

	renderer.Draw(&out, "top\nmiddle\nbottom", RenderContext{Width: 24})
	out.Reset()
	renderer.Draw(&out, "top", RenderContext{Width: 24})

	got := out.String()
	want := "\x1b[2;1H\x1b[2K\x1b[3;1H\x1b[2K"
	if got != want {
		t.Fatalf("shorter frame output = %q, want removed rows cleared", got)
	}
}

func TestDiffRendererFullRedrawsWhenWidthChanges(t *testing.T) {
	var out bytes.Buffer
	renderer := NewDiffRenderer()

	renderer.Draw(&out, "same\nframe", RenderContext{Width: 24})
	out.Reset()
	renderer.Draw(&out, "same\nframe", RenderContext{Width: 30})

	got := out.String()
	if !strings.HasPrefix(got, "\x1b[H\x1b[2J") {
		t.Fatalf("width change output = %q, want full redraw", got)
	}
}

func TestParseInputEvents(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Event
	}{
		{name: "j is a character", in: "j", want: KeyEvent{Key: KeyCharacter, Rune: 'j'}},
		{name: "k is a character", in: "k", want: KeyEvent{Key: KeyCharacter, Rune: 'k'}},
		{name: "K is a character", in: "K", want: KeyEvent{Key: KeyCharacter, Rune: 'K'}},
		{name: "enter", in: "\n", want: KeyEvent{Key: KeyEnter}},
		{name: "q character", in: "q", want: KeyEvent{Key: KeyCharacter, Rune: 'q'}},
		{name: "character", in: "x", want: KeyEvent{Key: KeyCharacter, Rune: 'x'}},
		{name: "backspace", in: "\x7f", want: KeyEvent{Key: KeyBackspace}},
		{name: "tab", in: "\t", want: KeyEvent{Key: KeyTab}},
		{name: "escape", in: "\x1b", want: KeyEvent{Key: KeyEscape}},
		{name: "arrow up", in: "\x1b[A", want: KeyEvent{Key: KeyUp}},
		{name: "arrow down", in: "\x1b[B", want: KeyEvent{Key: KeyDown}},
		{name: "arrow left", in: "\x1b[D", want: KeyEvent{Key: KeyLeft}},
		{name: "arrow right", in: "\x1b[C", want: KeyEvent{Key: KeyRight}},
		{name: "delete", in: "\x1b[3~", want: KeyEvent{Key: KeyDelete}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadEvent(strings.NewReader(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ReadEvent() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestEscapeDoesNotWaitForOrConsumeNextCharacter(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\x1bq"))

	first, err := readEvent(reader)
	if err != nil {
		t.Fatal(err)
	}
	second, err := readEvent(reader)
	if err != nil {
		t.Fatal(err)
	}

	if first != Event(KeyEvent{Key: KeyEscape}) {
		t.Fatalf("first event = %#v, want escape", first)
	}
	if second != Event(KeyEvent{Key: KeyCharacter, Rune: 'q'}) {
		t.Fatalf("second event = %#v, want q character", second)
	}
}

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
				Async(fx.Context, fx.Updates, "counter.load",
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
			View: func(s *testState, ctx RenderContext) string {
				return "count"
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
			View: func(s *testState, ctx RenderContext) string {
				return "unchanged"
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

	if got := strings.Count(out.String(), "\x1b[H\x1b[2J"); got != 1 {
		t.Fatalf("full redraw count = %d, want only initial redraw; output %q", got, out.String())
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
			View: func(s *testState, ctx RenderContext) string {
				return "init quit"
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
			View: func(s *testState, ctx RenderContext) string { return "x" },
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
			View: func(s *testState, ctx RenderContext) string {
				return fmt.Sprintf("count:%d", s.count)
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

	output := out.String()
	for _, intermediate := range []string{"count:1", "count:2"} {
		if strings.Contains(output, intermediate) {
			t.Fatalf("output contains intermediate frame %q; updates were not coalesced:\n%q", intermediate, output)
		}
	}
	if !strings.Contains(output, "count:0") {
		t.Fatalf("output missing initial pre-loop frame count:0:\n%q", output)
	}
	if !strings.Contains(output, "count:3") {
		t.Fatalf("output missing final frame count:3:\n%q", output)
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
			View: func(s *testState, ctx RenderContext) string {
				return fmt.Sprintf("count:%d", s.count)
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
			View: func(s *testState, ctx RenderContext) string { return "unchanged" },
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
			View: func(s *testState, ctx RenderContext) string { return "x" },
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
