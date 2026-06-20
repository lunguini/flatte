package flat

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"
	"testing"
)

// runFrameApp drives Run over a pipe with the given View, feeding it the
// input bytes and returning the full terminal output. The Handle counts
// 'x' presses into state.count and quits on 'q'.
func runFrameApp(t *testing.T, view func(*testState, RenderContext) Frame, input string, opts ...Option) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := testState{}
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &state,
			Handle: func(s *testState, ev Event, fx Effects[testState]) {
				key, ok := ev.(KeyEvent)
				if !ok || key.Key != KeyCharacter {
					return
				}
				switch key.Rune {
				case 'x':
					s.count++
				case 'q':
					fx.Quit()
				}
			},
			View: view,
		}, append([]Option{WithInput(reader), WithOutput(&out)}, opts...)...)
	}()

	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestRunShowsAndMovesHardwareCursor(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{
			Content: "field: abc",
			Cursor:  &Cursor{X: 7 + s.count, Y: 0},
		}
	}, "xq")

	if !strings.Contains(output, "\x1b[?25h") {
		t.Fatalf("cursor never shown:\n%q", output)
	}
	hide := strings.Index(output, "\x1b[?25l")
	show := strings.Index(output, "\x1b[?25h")
	if hide == -1 || show < hide {
		t.Fatalf("expected startup hide before show, hide=%d show=%d:\n%q", hide, show, output)
	}
	// Initial frame + cursor-only move ('x' changes no content) = two
	// synchronized writes: a cursor move must not short-circuit.
	if got := strings.Count(output, "\x1b[?2026h"); got < 2 {
		t.Fatalf("synchronized writes = %d, want >= 2:\n%q", got, output)
	}
}

func TestRunKeepsCursorHiddenWithoutFrameCursor(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{Content: "x"}
	}, "q")

	// The only show is the exit restore, after the last synchronized block.
	if got := strings.Count(output, "\x1b[?25h"); got != 1 {
		t.Fatalf("cursor shown %d time(s), want exit restore only:\n%q", got, output)
	}
	if strings.Index(output, "\x1b[?25h") < strings.LastIndex(output, "\x1b[?2026l") {
		t.Fatalf("cursor shown before the last frame write:\n%q", output)
	}
}

func TestRunEmitsCursorStyleAndResetsOnExit(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{
			Content: "field",
			Cursor: &Cursor{
				X: 1,
				Y: 0,
				Style: &CursorStyle{
					Shape: CursorShapeBar,
					Blink: false,
					Color: color.RGBA{R: 255, G: 128, B: 0, A: 255},
				},
			},
		}
	}, "q")

	if !strings.Contains(output, "\x1b[6 q") {
		t.Fatalf("bar cursor shape not emitted:\n%q", output)
	}
	if !strings.Contains(output, "\x1b]12;#ff8000\x07") {
		t.Fatalf("cursor color not emitted:\n%q", output)
	}
	if !strings.Contains(output, "\x1b[0 q") {
		t.Fatalf("cursor shape not reset on exit:\n%q", output)
	}
	if !strings.Contains(output, "\x1b]112\x07") {
		t.Fatalf("cursor color not reset on exit:\n%q", output)
	}
}

func TestRunEmitsWindowTitleOnChangeAndResetsOnExit(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{
			Content: fmt.Sprintf("count:%d", s.count),
			Title:   "flat demo",
		}
	}, "xq")

	// 'x' forces a second draw with the same title: emitted once, on change.
	if got := strings.Count(output, "\x1b]2;flat demo\x07"); got != 1 {
		t.Fatalf("title emitted %d time(s), want once:\n%q", got, output)
	}
	if !strings.Contains(output, "\x1b]2;\x07") {
		t.Fatalf("title not reset on exit:\n%q", output)
	}
}
