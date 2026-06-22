package flatte

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunInlineSkipsAltScreenAndEndsBelowTheFrame(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{Content: "inline-frame"}
	}, "q", WithInline())

	if strings.Contains(output, "1049") {
		t.Fatalf("inline run must never touch the alt screen:\n%q", output)
	}
	if strings.Contains(output, "\x1b[H") {
		t.Fatalf("inline run must use relative cursor movement, not absolute home:\n%q", output)
	}
	if !strings.Contains(output, "inline-frame") {
		t.Fatalf("frame content missing:\n%q", output)
	}
	if !strings.HasSuffix(output, "\r\n") {
		t.Fatalf("inline exit must end with a newline so the prompt lands below the frame:\n%q", output)
	}
	if !strings.Contains(output, "\x1b[?25h") {
		t.Fatalf("cursor not restored on exit:\n%q", output)
	}
}

func TestRunInlineIdenticalFramesWriteNothing(t *testing.T) {
	output := runFrameApp(t, plainView, "xxq", WithInline())

	// 'x' changes state but not the frame: one synchronized write total.
	if got := strings.Count(output, "\x1b[?2026h"); got != 1 {
		t.Fatalf("synchronized writes = %d, want 1:\n%q", got, output)
	}
}

func TestRunInlineCursorStillPositions(t *testing.T) {
	output := runFrameApp(t, func(s *testState, ctx RenderContext) Frame {
		return Frame{Content: "field: ab", Cursor: &Cursor{X: 7, Y: 0}}
	}, "q", WithInline())

	show := strings.Index(output, "\x1b[?25h")
	lastSync := strings.LastIndex(output, "\x1b[?2026l")
	if show == -1 || show > lastSync {
		t.Fatalf("cursor not shown within a frame write (show=%d lastSync=%d):\n%q", show, lastSync, output)
	}
}

func TestRunInlineSuspendRepaintsWithFreshRenderer(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	suspended := make(chan struct{})
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
					fx.Suspend()
				case 'q':
					fx.Quit()
				}
			},
			View: func(s *testState, ctx RenderContext) Frame {
				return Frame{Content: "inline-suspendable"}
			},
		}, WithInput(reader), WithOutput(&out), WithInline(),
			withSuspendProcess(func() { close(suspended) }))
	}()

	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-suspended:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for suspendProcess")
	}
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run after resume")
	}

	output := out.String()
	if strings.Contains(output, "1049") {
		t.Fatalf("inline suspend must not touch the alt screen:\n%q", output)
	}
	if got := strings.Count(output, "inline-suspendable"); got < 2 {
		t.Fatalf("frame painted %d time(s), want initial + post-resume repaint:\n%q", got, output)
	}
}
