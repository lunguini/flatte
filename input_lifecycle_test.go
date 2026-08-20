package flatte

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type trackingCancelReader struct {
	io.Reader
	closeErr     error
	uncancelable bool
	cancelled    bool
	closeCount   int
}

func (r *trackingCancelReader) Cancel() bool {
	r.cancelled = true
	return !r.uncancelable
}

func (r *trackingCancelReader) Close() error {
	r.closeCount++
	return r.closeErr
}

func TestRunClosesJoinedCancelReaderAndPreservesCloseError(t *testing.T) {
	closeErr := errors.New("input close failed")
	reader := &trackingCancelReader{Reader: strings.NewReader(""), closeErr: closeErr}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	state := testState{}
	var output bytes.Buffer
	err := Run(context.Background(), App[testState]{
		State: &state,
		Init:  func(_ *testState, fx Effects[testState]) { fx.Quit() },
		View:  plainView,
	}, WithInput(strings.NewReader("")), WithOutput(&output))

	if !errors.Is(err, closeErr) {
		t.Fatalf("Run error = %v, want input close error %v", err, closeErr)
	}
	if reader.closeCount != 1 {
		t.Fatalf("cancel reader close count = %d, want 1", reader.closeCount)
	}
}

type blockingTrackingCancelReader struct {
	once       sync.Once
	unblock    chan struct{}
	cancelled  bool
	closeCount int
}

func (r *blockingTrackingCancelReader) Read([]byte) (int, error) {
	<-r.unblock
	return 0, io.EOF
}

func (r *blockingTrackingCancelReader) Cancel() bool {
	r.cancelled = true
	r.once.Do(func() { close(r.unblock) })
	return true
}

func (r *blockingTrackingCancelReader) Close() error {
	r.closeCount++
	return nil
}

func TestRunCancelsJoinsAndClosesBlockedCancelReader(t *testing.T) {
	reader := &blockingTrackingCancelReader{unblock: make(chan struct{})}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	state := testState{}
	var output bytes.Buffer
	if err := Run(context.Background(), App[testState]{
		State: &state,
		Init:  func(_ *testState, fx Effects[testState]) { fx.Quit() },
		View:  plainView,
	}, WithInput(strings.NewReader("")), WithOutput(&output)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !reader.cancelled {
		t.Fatal("blocked cancel reader was not cancelled before joining")
	}
	if reader.closeCount != 1 {
		t.Fatalf("blocked cancel reader close count = %d, want 1", reader.closeCount)
	}
}

func TestRunClosesCancelReaderThatReachedEOFWithoutCancellation(t *testing.T) {
	reader := &trackingCancelReader{Reader: strings.NewReader(""), uncancelable: true}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	state := testState{}
	var output bytes.Buffer
	if err := Run(context.Background(), App[testState]{
		State: &state,
		View:  plainView,
	}, WithInput(strings.NewReader("")), WithOutput(&output)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if reader.closeCount != 1 {
		t.Fatalf("EOF cancel reader close count = %d, want 1", reader.closeCount)
	}
}

type eventualCloseReader struct {
	release    chan struct{}
	closeCount atomic.Int32
}

func (r *eventualCloseReader) Read([]byte) (int, error) {
	<-r.release
	return 0, io.EOF
}

func (*eventualCloseReader) Cancel() bool { return false }
func (r *eventualCloseReader) Close() error {
	r.closeCount.Add(1)
	return nil
}

func TestRunEventuallyClosesUncancelableReaderAfterReturn(t *testing.T) {
	reader := &eventualCloseReader{release: make(chan struct{})}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	state := testState{}
	if err := Run(context.Background(), App[testState]{
		State: &state,
		Init:  func(_ *testState, fx Effects[testState]) { fx.Quit() },
		View:  plainView,
	}, WithInput(strings.NewReader("")), WithOutput(&bytes.Buffer{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := reader.closeCount.Load(); got != 0 {
		t.Fatalf("reader closed before blocked read completed: %d", got)
	}
	close(reader.release)
	deadline := time.Now().Add(time.Second)
	for reader.closeCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := reader.closeCount.Load(); got != 1 {
		t.Fatalf("eventual close count = %d, want 1", got)
	}
}

type errorCancelReader struct {
	err        error
	closeCount atomic.Int32
}

func (r *errorCancelReader) Read([]byte) (int, error) { return 0, r.err }
func (*errorCancelReader) Cancel() bool               { return false }
func (r *errorCancelReader) Close() error {
	r.closeCount.Add(1)
	return nil
}

func TestRunClosesCancelReaderAfterTerminalInputError(t *testing.T) {
	wantErr := errors.New("terminal input failed")
	reader := &errorCancelReader{err: wantErr}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	state := testState{}
	err := Run(context.Background(), App[testState]{State: &state, View: plainView},
		WithInput(strings.NewReader("")), WithOutput(&bytes.Buffer{}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}
	if got := reader.closeCount.Load(); got != 1 {
		t.Fatalf("terminal-error close count = %d, want 1", got)
	}
}

type contextCancelReader struct {
	once       sync.Once
	release    chan struct{}
	closeCount atomic.Int32
}

func (r *contextCancelReader) Read([]byte) (int, error) {
	<-r.release
	return 0, context.Canceled
}
func (r *contextCancelReader) Cancel() bool {
	r.once.Do(func() { close(r.release) })
	return true
}
func (r *contextCancelReader) Close() error {
	r.closeCount.Add(1)
	return nil
}

func TestRunContextCancellationPreservesAppliedStateAndCleansInput(t *testing.T) {
	reader := &contextCancelReader{release: make(chan struct{})}
	originalFactory := cancelReaderFactory
	t.Cleanup(func() { cancelReaderFactory = originalFactory })
	cancelReaderFactory = func(io.Reader) (cancelReader, error) { return reader, nil }

	ctx, cancel := context.WithCancel(context.Background())
	fxReady := make(chan Effects[testState], 1)
	applied := make(chan struct{})
	exited := make(chan int, 1)
	state := testState{}
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[testState]{
			State: &state,
			Init:  func(_ *testState, fx Effects[testState]) { fxReady <- fx },
			View:  plainView,
			OnExit: func(s *testState) {
				exited <- s.count
			},
		}, WithInput(strings.NewReader("")), WithOutput(&bytes.Buffer{}))
	}()
	fx := <-fxReady
	if !fx.Send(Named("applied-before-cancel", func(s *testState) {
		s.count++
		close(applied)
	})) {
		t.Fatal("failed to enqueue update")
	}
	<-applied
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run cancellation error = %v, want nil", err)
	}
	if got := <-exited; got != 1 {
		t.Fatalf("OnExit state count = %d, want 1 applied update", got)
	}
	if got := reader.closeCount.Load(); got != 1 {
		t.Fatalf("context-cancel close count = %d, want 1", got)
	}
}
