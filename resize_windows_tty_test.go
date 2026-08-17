//go:build windows

package flatte

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/term"
)

type windowsResizeTTYState struct {
	initialWidth  int
	initialHeight int
	sawResize     bool
	geometryMatch bool
	frameEpoch    int
}

type windowsResizeCapture struct {
	file *os.File
	bytes.Buffer
}

func (w *windowsResizeCapture) Fd() uintptr { return w.file.Fd() }

func (w *windowsResizeCapture) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	return w.Buffer.Write(p)
}

// TestWindowsConsoleResizeTTY is an explicit real-console gate. Ordinary CI
// uses pipes and must skip it: a WINDOW_BUFFER_SIZE_EVENT can only be produced
// by resizing an attached Windows console. Build the test binary and run it
// directly as documented in README.md, then resize the window once.
func TestWindowsConsoleResizeTTY(t *testing.T) {
	if os.Getenv("FLATTE_WINDOWS_RESIZE_TTY") != "1" {
		t.Skip("set FLATTE_WINDOWS_RESIZE_TTY=1 in a real Windows console")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		t.Fatal("stdin and stdout must both be attached to a real Windows console")
	}

	state := &windowsResizeTTYState{}
	output := &windowsResizeCapture{file: os.Stdout}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	fmt.Fprintln(os.Stdout, "Resize this console window once within 45 seconds.")
	err := Run(ctx, App[windowsResizeTTYState]{
		State: state,
		Handle: func(s *windowsResizeTTYState, ev Event, fx Effects[windowsResizeTTYState]) {
			if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter && key.Rune == 'q' {
				fx.Quit()
				return
			}
			resize, ok := ev.(ResizeEvent)
			if !ok {
				return
			}
			if s.initialWidth == 0 && s.initialHeight == 0 {
				s.initialWidth, s.initialHeight = resize.Width, resize.Height
				return
			}
			if resize.Width != s.initialWidth || resize.Height != s.initialHeight {
				s.sawResize = true
				s.frameEpoch++
				width, height, err := term.GetSize(int(os.Stdout.Fd()))
				s.geometryMatch = err == nil && width == resize.Width && height == resize.Height
				// Do not quit on this event: Run draws only after Handle
				// returns, so the resize itself must reach that draw first.
			}
		},
		View: func(s *windowsResizeTTYState, _ RenderContext) Frame {
			return Frame{Content: fmt.Sprintf("Initial size: %dx%d\nResize epoch: %d\nResize the Windows console, then press q.", s.initialWidth, s.initialHeight, s.frameEpoch)}
		},
	}, WithOutput(output))
	if err != nil {
		t.Fatal(err)
	}
	if !state.sawResize {
		t.Fatal("no live Windows console ResizeEvent observed before timeout")
	}
	if !state.geometryMatch {
		t.Fatal("ResizeEvent dimensions did not match the visible Windows console")
	}
	if !bytes.Contains(output.Bytes(), []byte("Resize epoch: 1")) {
		t.Fatal("resize did not produce a repaint containing the updated frame epoch")
	}
}
