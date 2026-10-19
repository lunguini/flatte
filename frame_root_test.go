package flatte

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func rootTree() layout.Node {
	return layout.Col{Children: []layout.Node{
		layout.Text{NodeBase: layout.NodeBase{ID: "header", H: layout.Fixed(1)}, String: "header"},
		layout.Text{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, String: "body"},
	}}
}

func TestComposeFrameResolvesRoot(t *testing.T) {
	cursor := &Cursor{X: 1, Y: 2}
	frame, comp := ComposeFrame(Frame{Root: rootTree(), Content: "ignored", Title: "t", Cursor: cursor}, 10, 4)
	want, _ := layout.SolveAndCompose(rootTree(), 10, 4)
	if frame.Content != want {
		t.Fatalf("Content = %q, want the composed tree %q", frame.Content, want)
	}
	if frame.Title != "t" || frame.Cursor != cursor || frame.Root == nil {
		t.Fatalf("metadata not preserved: %+v", frame)
	}
	if got, _ := comp.HitTest(0, 0); got != "header" {
		t.Fatalf("HitTest(0,0) = %q, want header", got)
	}
	if got, _ := comp.HitTest(0, 3); got != "body" {
		t.Fatalf("HitTest(0,3) = %q, want body", got)
	}
}

func TestComposeFrameWithoutRootIsUnchanged(t *testing.T) {
	in := Frame{Content: "plain", Title: "t"}
	frame, comp := ComposeFrame(in, 10, 4)
	if !framesEqual(frame, in) || frame.Root != nil {
		t.Fatalf("frame changed: %+v", frame)
	}
	if got, _ := comp.HitTest(0, 0); got != "" {
		t.Fatalf("HitTest without Root = %q, want \"\"", got)
	}
}

func TestComposeFrameDegenerateSize(t *testing.T) {
	frame, comp := ComposeFrame(Frame{Root: rootTree(), Content: "stale"}, 0, 4)
	if id, _ := comp.HitTest(0, 0); frame.Content != "" || id != "" {
		t.Fatalf("zero-width compose = %q", frame.Content)
	}
}

func TestRootHeight(t *testing.T) {
	fixed := layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(3)}, String: "x"}
	content := layout.Text{String: "a\nb"}
	tall := layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(99)}, String: "x"}
	grow := layout.Text{NodeBase: layout.NodeBase{H: layout.Grow(1)}, String: "x"}
	for _, tc := range []struct {
		name   string
		root   layout.Node
		inline bool
		want   int
	}{
		{"alt screen fills the terminal", fixed, false, 24},
		{"inline fixed", fixed, true, 3},
		{"inline content", content, true, 2},
		{"inline clamps to terminal", tall, true, 24},
		{"inline grow fills the terminal", grow, true, 24},
	} {
		if got := rootHeight(tc.root, 24, tc.inline); got != tc.want {
			t.Errorf("%s: rootHeight = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestRunFillsMouseTargetFromRoot drives Run with real SGR mouse bytes and
// checks Handle sees the ID of the node under the pointer, solved by the
// runtime from Frame.Root — no app-side Solve.
func TestRunFillsMouseTargetFromRoot(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var targets []string
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[testState]{
			State: &testState{},
			Handle: func(_ *testState, ev Event, fx Effects[testState]) {
				switch ev := ev.(type) {
				case MouseEvent:
					if ev.Action == MousePress {
						targets = append(targets, ev.Target)
					}
				case KeyEvent:
					if ev.Rune == 'q' {
						fx.Quit()
					}
				}
			},
			View: func(*testState, RenderContext) Frame { return Frame{Root: rootTree()} },
		}, WithInput(reader), WithOutput(&out), WithMouse(MouseModeCellMotion))
	}()

	// SGR coordinates are 1-based: (6,1) is cell (5,0), (6,6) is cell (5,5),
	// and (99,1) lies outside the 72-column fallback frame.
	input := "\x1b[<0;6;1M\x1b[<0;6;1m" + "\x1b[<0;6;6M\x1b[<0;6;6m" + "\x1b[<0;99;1M\x1b[<0;99;1m" + "q"
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(targets, ","), "header,body,"; got != want {
		t.Fatalf("targets = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "header") {
		t.Fatalf("composed root not drawn:\n%q", out.String())
	}
}
