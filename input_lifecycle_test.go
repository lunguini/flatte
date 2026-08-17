package flatte

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
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
