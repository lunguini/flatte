package flatte

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type outputWriteResult struct {
	n   int
	err error
}

type sequencedOutputWriter struct {
	results []outputWriteResult
	writes  int
}

func (w *sequencedOutputWriter) Write(p []byte) (int, error) {
	result := outputWriteResult{n: len(p)}
	if w.writes < len(w.results) {
		result = w.results[w.writes]
		if result.n < 0 {
			result.n = len(p)
		}
	}
	w.writes++
	return result.n, result.err
}

func outputTestApp(onExit func(*testState)) App[testState] {
	return App[testState]{
		State:  &testState{},
		OnExit: onExit,
		Handle: func(s *testState, event Event, fx Effects[testState]) {
			if key, ok := event.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
				fx.Quit()
			}
		},
		View: func(*testState, RenderContext) Frame { return Frame{Content: "output"} },
	}
}

func TestOutputSinkDetectsShortWrite(t *testing.T) {
	w := &sequencedOutputWriter{results: []outputWriteResult{{n: 1}}}
	sink := &outputSink{w: w}

	if err := sink.write([]byte("abc")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("write error = %v, want io.ErrShortWrite", err)
	}
	if !errors.Is(sink.err, io.ErrShortWrite) {
		t.Fatalf("latched error = %v, want io.ErrShortWrite", sink.err)
	}
}

func TestRunReturnsInitialOutputFailureAndStillCallsOnExit(t *testing.T) {
	errInitial := errors.New("initial output failure")
	w := &sequencedOutputWriter{results: []outputWriteResult{{n: 0, err: errInitial}}}
	exited := 0
	app := outputTestApp(func(*testState) { exited++ })

	err := Run(context.Background(), app, WithInput(strings.NewReader("q")), WithOutput(w))

	if !errors.Is(err, errInitial) {
		t.Fatalf("Run error = %v, want %v", err, errInitial)
	}
	if exited != 1 {
		t.Fatalf("OnExit calls = %d, want 1", exited)
	}
}

func TestRunReturnsCleanupOnlyOutputFailure(t *testing.T) {
	errCleanup := errors.New("cleanup output failure")
	w := &sequencedOutputWriter{results: []outputWriteResult{
		{n: -1},
		{n: 0, err: errCleanup},
	}}
	exited := 0
	app := outputTestApp(func(*testState) { exited++ })

	err := Run(context.Background(), app, WithInput(strings.NewReader("q")), WithOutput(w))

	if !errors.Is(err, errCleanup) {
		t.Fatalf("Run error = %v, want %v", err, errCleanup)
	}
	if exited != 1 {
		t.Fatalf("OnExit calls = %d, want 1", exited)
	}
}

func TestRunPreservesFirstOutputFailureAcrossCleanup(t *testing.T) {
	errInitial := errors.New("initial output failure")
	errCleanup := errors.New("cleanup output failure")
	w := &sequencedOutputWriter{results: []outputWriteResult{
		{n: 0, err: errInitial},
		{n: 0, err: errCleanup},
	}}
	app := outputTestApp(nil)

	err := Run(context.Background(), app, WithInput(strings.NewReader("q")), WithOutput(w))

	if !errors.Is(err, errInitial) {
		t.Fatalf("Run error = %v, want first output error %v", err, errInitial)
	}
	if errors.Is(err, errCleanup) {
		t.Fatalf("Run error = %v, unexpectedly replaced by cleanup error", err)
	}
}

type failingInputReader struct{ err error }

func (r failingInputReader) Read([]byte) (int, error) { return 0, r.err }

func TestRunPreservesEarlierNonOutputError(t *testing.T) {
	errInput := errors.New("input failure")
	errCleanup := errors.New("cleanup output failure")
	w := &sequencedOutputWriter{results: []outputWriteResult{
		{n: -1},
		{n: 0, err: errCleanup},
	}}
	app := outputTestApp(nil)

	err := Run(context.Background(), app, WithInput(failingInputReader{err: errInput}), WithOutput(w))

	if !errors.Is(err, errInput) {
		t.Fatalf("Run error = %v, want earlier input error %v", err, errInput)
	}
}

func TestOutputSinkCanWriteFromABytesBuffer(t *testing.T) {
	var out bytes.Buffer
	sink := &outputSink{w: &out}
	if err := sink.write([]byte("ok")); err != nil {
		t.Fatalf("write error = %v", err)
	}
	if got := out.String(); got != "ok" {
		t.Fatalf("output = %q, want %q", got, "ok")
	}
}
