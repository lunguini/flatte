package flatcore

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// runPrintApp runs an inline-or-alt-screen app whose 'x' key triggers onX and
// whose 'q' key quits, over a pipe, and returns everything written to output.
func runPrintApp(t *testing.T, inline bool, onX func(Effects[testState])) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	var out bytes.Buffer
	opts := []Option{WithInput(reader), WithOutput(&out)}
	if inline {
		opts = append(opts, WithInline())
	}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				if key, ok := ev.(KeyEvent); ok && key.Key == KeyCharacter {
					switch key.Rune {
					case 'x':
						onX(fx)
					case 'q':
						fx.Quit()
					}
				}
			},
			View: plainView,
		}, opts...)
	}()

	if _, err := writer.Write([]byte("xq")); err != nil {
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
	return out.String()
}

func TestRunPrintEmitsLineInInlineMode(t *testing.T) {
	out := runPrintApp(t, true, func(fx Effects[testState]) {
		fx.Print("hello scrollback")
	})
	if !strings.Contains(out, "hello scrollback") {
		t.Fatalf("fx.Print content not emitted above the inline frame:\n%q", out)
	}
	// The position-independent path erases the old frame from the screen
	// (EraseScreenBelow) and writes the unmanaged line terminated with
	// EraseLineRight + CRLF, then repaints the frame fresh below it. The earlier
	// scroll-and-insert approaches (uv PrependString / BT v2 insertAbove) used
	// InsertLine and assumed the frame sat at the screen bottom, which walked the
	// frame down the screen on each print; assert we no longer do that.
	if !strings.Contains(out, "\x1b[J") { // ansi.EraseScreenBelow
		t.Fatalf("fx.Print did not erase the frame before emitting scrollback:\n%q", out)
	}
	if !strings.Contains(out, "hello scrollback\x1b[K\r\n") {
		t.Fatalf("inserted line not terminated with EraseLineRight+CRLF:\n%q", out)
	}
	afterLine := out[strings.Index(out, "hello scrollback\x1b[K\r\n")+len("hello scrollback\x1b[K\r\n"):]
	if beforeFrame, _, ok := strings.Cut(afterLine, "x"); ok && strings.Contains(beforeFrame, "\x1b[J") {
		t.Fatalf("fx.Print erased after emitting the scrollback line, so the line is not visible before repaint:\n%q", out)
	}
	if strings.Contains(out, "\x1b[L") || strings.Contains(out, "\x1b[1L") {
		t.Fatalf("fx.Print must not use InsertLine (the scroll-at-bottom approach):\n%q", out)
	}
}

func TestRunPrintfFormats(t *testing.T) {
	out := runPrintApp(t, true, func(fx Effects[testState]) {
		fx.Printf("count=%d", 42)
	})
	if !strings.Contains(out, "count=42") {
		t.Fatalf("fx.Printf content not emitted:\n%q", out)
	}
}

func TestRunPrintIsNoOpInAltScreen(t *testing.T) {
	out := runPrintApp(t, false, func(fx Effects[testState]) {
		fx.Print("hello scrollback")
	})
	if strings.Contains(out, "hello scrollback") {
		t.Fatalf("fx.Print must be a no-op in alt-screen mode (the lines would be overwritten):\n%q", out)
	}
}
