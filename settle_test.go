package flatte

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// sized renders a Root whose "list" pane is rows tall and whose "body"
// fills the rest; growing rows moves "body", so adopting a size changes the
// solved rects again — the chain LayoutEvent exists to settle.
func sized(rows int) Frame {
	return Frame{Root: layout.Col{Children: []layout.Node{
		layout.Text{NodeBase: layout.NodeBase{ID: "list", H: layout.Fixed(rows)}, String: fmt.Sprintf("rows=%d", rows)},
		layout.Text{NodeBase: layout.NodeBase{ID: "body", H: layout.Grow(1)}, String: fmt.Sprintf("body rows=%d", rows)},
	}}}
}

type settleProbe struct {
	views, events int
	rows          int
}

func (p *settleProbe) view() Frame { p.views++; return sized(p.rows) }

func TestSettleFrameWithoutRoot(t *testing.T) {
	views, events := 0, 0
	last := map[string]layout.Rect{"keep": {W: 1}}
	frame, comp, gotLast := SettleFrame(func() Frame { views++; return Frame{Content: "plain"} },
		func(LayoutEvent) { events++ }, last, 10, 4)
	if frame.Content != "plain" || views != 1 || events != 0 {
		t.Fatalf("frame=%q views=%d events=%d", frame.Content, views, events)
	}
	if id, _ := comp.HitTest(0, 0); id != "" || len(gotLast) != 1 {
		t.Fatalf("composition/last changed: hit=%q last=%v", id, gotLast)
	}
}

func TestSettleFrameDeliversBeforePaintThenStabilizes(t *testing.T) {
	p := &settleProbe{rows: 1}
	var got []LayoutEvent
	frame, comp, last := SettleFrame(p.view, func(ev LayoutEvent) { got = append(got, ev) }, nil, 10, 6)
	if p.views != 2 || len(got) != 1 {
		t.Fatalf("views=%d events=%d, want 2 and 1", p.views, len(got))
	}
	if r := got[0].Rects["list"]; r != (layout.Rect{W: 10, H: 1}) {
		t.Fatalf("delivered list rect = %+v", r)
	}
	if !strings.Contains(frame.Content, "rows=1") || len(last) != 2 || len(comp.Rects) != 2 {
		t.Fatalf("frame=%q last=%v", frame.Content, last)
	}

	p.views = 0
	_, _, _ = SettleFrame(p.view, func(ev LayoutEvent) { got = append(got, ev) }, last, 10, 6)
	if p.views != 1 || len(got) != 1 {
		t.Fatalf("stable render: views=%d events=%d, want 1 and still 1", p.views, len(got))
	}
}

// TestSettleFrameFollowsAdoptionChain mirrors a list adopting its height from
// its pane: the first event resizes the list, which moves body, so a second
// event is needed; then the geometry is stable and the frame is painted.
func TestSettleFrameFollowsAdoptionChain(t *testing.T) {
	p := &settleProbe{rows: 1}
	handle := func(ev LayoutEvent) {
		p.events++
		p.rows = min(ev.Rects["body"].H, 2) // adopt: list takes up to 2 rows of body
	}
	frame, _, last := SettleFrame(p.view, handle, nil, 10, 6)
	if p.events != 2 || p.views != 3 {
		t.Fatalf("events=%d views=%d, want 2 and 3", p.events, p.views)
	}
	if !strings.Contains(frame.Content, "rows=2") || last["list"].H != 2 {
		t.Fatalf("frame=%q last list=%+v", frame.Content, last["list"])
	}
}

func TestSettleFrameBoundsNonConvergingLayout(t *testing.T) {
	p := &settleProbe{rows: 1}
	handle := func(LayoutEvent) { p.events++; p.rows++ } // never converges
	frame, comp, last := SettleFrame(p.view, handle, nil, 10, 10)
	if p.events != maxLayoutPasses || p.views != maxLayoutPasses+1 {
		t.Fatalf("events=%d views=%d, want %d and %d", p.events, p.views, maxLayoutPasses, maxLayoutPasses+1)
	}
	// The painted frame is the last render; the rects it differs by are
	// re-delivered on the next render instead of spinning now.
	if !strings.Contains(frame.Content, "rows=3") || last["list"].H != 2 || comp.Rects["list"].H != 3 {
		t.Fatalf("frame=%q last=%+v comp=%+v", frame.Content, last["list"], comp.Rects["list"])
	}
	p.events = 0
	_, _, _ = SettleFrame(p.view, handle, last, 10, 10)
	if p.events == 0 {
		t.Fatal("unsettled geometry was not re-delivered on the next render")
	}
}

func TestSettleFrameDeliversACopy(t *testing.T) {
	p := &settleProbe{rows: 1}
	_, _, last := SettleFrame(p.view, func(ev LayoutEvent) { clear(ev.Rects) }, nil, 10, 6)
	if len(last) != 2 {
		t.Fatalf("handler mutation reached the host's copy: %v", last)
	}
	events := 0
	_, _, _ = SettleFrame(p.view, func(LayoutEvent) { events++ }, last, 10, 6)
	if events != 0 {
		t.Fatalf("unchanged layout re-delivered %d times", events)
	}
}

func TestSettleFrameNilHandle(t *testing.T) {
	p := &settleProbe{rows: 1}
	frame, _, last := SettleFrame(p.view, nil, nil, 10, 6)
	if p.views != 1 || !strings.Contains(frame.Content, "rows=1") || last != nil {
		t.Fatalf("views=%d frame=%q last=%v", p.views, frame.Content, last)
	}
}

// lockedBuffer is a goroutine-safe output sink so a test can watch Run's
// output while Run is still writing it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type layoutRunState struct {
	rows    int
	targets []string
	rects   []layout.Rect
}

func runLayoutApp(t *testing.T, onLayout func(*layoutRunState, LayoutEvent, Effects[layoutRunState]), opts ...Option) (*lockedBuffer, *os.File, chan error, *layoutRunState) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	state := &layoutRunState{}
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[layoutRunState]{
			State: state,
			Handle: func(s *layoutRunState, ev Event, fx Effects[layoutRunState]) {
				switch ev := ev.(type) {
				case LayoutEvent:
					onLayout(s, ev, fx)
				case MouseEvent:
					if ev.Action == MousePress {
						s.targets = append(s.targets, ev.Target)
						s.rects = append(s.rects, ev.TargetRect)
					}
				case KeyEvent:
					if ev.Rune == 'q' {
						fx.Quit()
					}
				}
			},
			View: func(s *layoutRunState, _ RenderContext) Frame { return sized(s.rows) },
		}, append([]Option{WithInput(reader), WithOutput(out)}, opts...)...)
	}()
	return out, writer, done, state
}

func waitFor(t *testing.T, out *lockedBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q in output:\n%q", want, out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestRunAdoptsLayoutBeforeFirstPaint proves geometry-derived state is in
// place before anything is shown: the unadopted rows=0 frame never paints.
func TestRunAdoptsLayoutBeforeFirstPaint(t *testing.T) {
	out, writer, done, _ := runLayoutApp(t, func(s *layoutRunState, ev LayoutEvent, _ Effects[layoutRunState]) {
		s.rows = min(ev.Rects["body"].H, 3)
	})
	waitFor(t, out, "rows=3")
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "rows=0") {
		t.Fatalf("a frame painted before its layout was adopted:\n%q", out.String())
	}
}

func TestRunQuitFromLayoutEvent(t *testing.T) {
	out, _, done, _ := runLayoutApp(t, func(_ *layoutRunState, _ LayoutEvent, fx Effects[layoutRunState]) {
		fx.Quit()
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not quit from a LayoutEvent")
	}
	if strings.Contains(out.String(), "rows=") {
		t.Fatalf("frame painted after quit was requested:\n%q", out.String())
	}
}

// TestRunFlushesActionsQueuedByLayoutEvent: an effect queued while settling
// layout runs without waiting for an unrelated later event.
func TestRunFlushesActionsQueuedByLayoutEvent(t *testing.T) {
	out, writer, done, _ := runLayoutApp(t, func(s *layoutRunState, _ LayoutEvent, fx Effects[layoutRunState]) {
		if s.rows == 0 {
			s.rows = 1
			fx.SetClipboard("laid out")
		}
	})
	waitFor(t, out, ansi.SetSystemClipboard("laid out"))
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunFillsMouseTargetRect(t *testing.T) {
	out, writer, done, state := runLayoutApp(t, func(s *layoutRunState, ev LayoutEvent, _ Effects[layoutRunState]) {
		s.rows = 2
	}, WithMouse(MouseModeCellMotion))
	waitFor(t, out, "rows=2")
	// Cell (3,1) is inside the 2-row list; (3,5) is in body below it.
	if _, err := writer.Write([]byte("\x1b[<0;4;2M\x1b[<0;4;2m\x1b[<0;4;6M\x1b[<0;4;6mq")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(state.targets, ","); got != "list,body" {
		t.Fatalf("targets = %q", got)
	}
	if state.rects[0] != (layout.Rect{W: 72, H: 2}) || state.rects[1] != (layout.Rect{Y: 2, W: 72, H: 22}) {
		t.Fatalf("target rects = %+v", state.rects)
	}
}

func TestRunFillsMouseCaretAndSuper(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	type state struct {
		carets []int
		keys   []KeyEvent
	}
	s := &state{}
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[state]{
			State: s,
			Handle: func(s *state, ev Event, fx Effects[state]) {
				switch ev := ev.(type) {
				case MouseEvent:
					if ev.Action == MousePress {
						c := -1
						if ev.HasCaret {
							c = ev.Caret
						}
						s.carets = append(s.carets, c)
					}
				case KeyEvent:
					if ev.Rune == 'q' {
						fx.Quit()
						return
					}
					s.keys = append(s.keys, ev)
				}
			},
			View: func(*state, RenderContext) Frame {
				return Frame{Root: layout.Col{Children: []layout.Node{
					layout.Input{NodeBase: layout.NodeBase{ID: "in"}, Value: "hello"},
					layout.Label{NodeBase: layout.NodeBase{ID: "below"}, Text: "ready"},
				}}}
			},
		}, WithInput(reader), WithOutput(out), WithMouse(MouseModeCellMotion))
	}()
	waitFor(t, out, "ready")
	// Clicks at (3,0) in the input and (3,1) on the label; then ⌘C as a
	// kitty-protocol key (modifiers 1+8: super), and ⌘← and ⇧⌘← as Warp
	// sends them in the xterm encoding (modifier bit 8: meta).
	if _, err := writer.Write([]byte("\x1b[<0;4;1M\x1b[<0;4;2M\x1b[99;9u\x1b[1;9D\x1b[1;10Dq")); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(s.carets) != 2 || s.carets[0] != 3 || s.carets[1] != -1 {
		t.Fatalf("carets = %v, want [3 -1]", s.carets)
	}
	if len(s.keys) != 3 || s.keys[0].Rune != 'c' || !s.keys[0].Mod.Contains(ModSuper) {
		t.Fatalf("keys = %+v, want super+c first", s.keys)
	}
	if k := s.keys[1]; k.Key != KeyLeft || k.Mod != ModSuper {
		t.Fatalf("⌘← (CSI 1;9D) = %+v, want super+left", k)
	}
	if k := s.keys[2]; k.Key != KeyLeft || k.Mod != ModSuper|ModShift {
		t.Fatalf("⇧⌘← (CSI 1;10D) = %+v, want super+shift+left", k)
	}
}
