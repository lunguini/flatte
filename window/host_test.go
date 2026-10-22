package window

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// cellMetrics: 8pt columns, 16pt lines, 4pt steps, 1pt border; text as
// cells.
var cellMetrics = layout.Metrics{Column: 8, Line: 16, Step: 4, Border: 1}

type fakeBackend struct {
	mu          sync.Mutex
	posted      []func()
	stops       int
	invalidates int
}

func (b *fakeBackend) post(f func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.posted = append(b.posted, f)
}

// drain runs posted functions on the "UI thread" (the test goroutine),
// waiting up to a second for at least want of them.
func (b *fakeBackend) drain(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	ran := 0
	for ran < want {
		b.mu.Lock()
		fs := b.posted
		b.posted = nil
		b.mu.Unlock()
		for _, f := range fs {
			f()
			ran++
		}
		if ran < want {
			if time.Now().After(deadline) {
				t.Fatalf("only %d of %d posted functions arrived", ran, want)
			}
			time.Sleep(time.Millisecond)
		}
	}
}

type testState struct {
	events  []flatte.Event
	layouts int
	count   int
	quitOn  rune
	root    func(*testState) layout.Node
	content string
	exited  bool
}

func testApp(s *testState) flatte.App[testState] {
	return flatte.App[testState]{
		State: s,
		Handle: func(s *testState, ev flatte.Event, fx flatte.Effects[testState]) {
			s.events = append(s.events, ev)
			switch ev := ev.(type) {
			case flatte.LayoutEvent:
				s.layouts++
			case flatte.KeyEvent:
				if ev.Key == flatte.KeyCharacter && ev.Rune == s.quitOn {
					fx.Quit()
				}
				if ev.Rune == 'g' {
					flatte.Go(fx, "count", func(context.Context) (int, error) { return 41, nil }, func(s *testState, n int, _ error) { s.count = n + 1 })
				}
			}
		},
		View: func(s *testState, ctx flatte.RenderContext) flatte.Frame {
			if s.root == nil {
				return flatte.Frame{Content: s.content, Title: "plain", Cursor: &flatte.Cursor{X: 1, Y: 0}}
			}
			return flatte.Frame{Root: s.root(s), Title: "tree"}
		},
		OnExit: func(s *testState) { s.exited = true },
	}
}

func newTestHost(t *testing.T, s *testState) (*host[testState], *fakeBackend) {
	t.Helper()
	b := &fakeBackend{}
	h := newHost(context.Background(), testApp(s), cellMetrics, b.post, func() { b.stops++ }, func() { b.invalidates++ })
	t.Cleanup(h.cancel)
	return h, b
}

func TestHostResizeInCells(t *testing.T) {
	s := &testState{content: "x"}
	h, _ := newTestHost(t, s)
	h.start(803, 399.9) // 100.375 columns, 24.99 lines
	if len(s.events) != 1 || s.events[0] != (flatte.ResizeEvent{Width: 100, Height: 24}) {
		t.Fatalf("events = %#v, want one ResizeEvent{100 24}", s.events)
	}
	h.resize(803, 399.9) // unchanged: no event
	if len(s.events) != 1 {
		t.Fatalf("an unchanged size delivered %#v", s.events[1:])
	}
}

func TestHostStringFrameIsCellGrid(t *testing.T) {
	s := &testState{content: "hello\nworld"}
	h, _ := newTestHost(t, s)
	h.start(80, 48)
	scene := h.render()
	if h.title != "plain" || h.cursor == nil || *h.cursor != (flatte.Cursor{X: 1, Y: 0}) {
		t.Fatalf("title %q cursor %+v", h.title, h.cursor)
	}
	if len(scene.Ops) != 1 {
		t.Fatalf("ops = %#v", scene.Ops)
	}
	c, ok := scene.Ops[0].(layout.CellsOp)
	if !ok || c.Cols != 10 || c.Lines != 3 || !strings.HasPrefix(c.Content, "hello") {
		t.Fatalf("op = %#v", scene.Ops[0])
	}
	if s.layouts != 0 {
		t.Fatalf("a string frame delivered %d LayoutEvents", s.layouts)
	}
}

func listTree(s *testState) layout.Node {
	rows := []layout.Node{}
	for i := range 3 {
		rows = append(rows, layout.Label{NodeBase: layout.NodeBase{ID: "row" + string(rune('0'+i)), H: layout.Fixed(1)}, Text: "row"})
	}
	return layout.Col{NodeBase: layout.NodeBase{Pad: 1}, Children: []layout.Node{
		layout.Label{Text: "title", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Col{NodeBase: layout.NodeBase{ID: "list"}, Children: rows},
	}}
}

func TestHostSettlesLayoutInCells(t *testing.T) {
	s := &testState{root: listTree}
	h, _ := newTestHost(t, s)
	h.start(160, 160)
	h.render()
	if s.layouts != 1 {
		t.Fatalf("layouts = %d, want 1", s.layouts)
	}
	ev := s.events[len(s.events)-1].(flatte.LayoutEvent)
	// The list starts below a 16pt title inside a 4pt pad: y = 20pt,
	// which is line 1 (rounded down); it is three 16pt lines tall.
	if got := ev.Rects["list"]; got != (layout.Rect{X: 0, Y: 1, W: 19, H: 3}) {
		t.Fatalf("list = %+v", got)
	}
	if got := ev.Rects["row1"]; got.H != 1 {
		t.Fatalf("row1 = %+v", got)
	}
	h.render()
	if s.layouts != 1 {
		t.Fatalf("an unchanged layout was delivered again (%d)", s.layouts)
	}
}

func TestHostMouseIsExactInsideTarget(t *testing.T) {
	s := &testState{root: listTree}
	h, _ := newTestHost(t, s)
	h.start(160, 160)
	h.render()
	// row1 spans y 36..52pt. A point 15pt into it is still its line 0, even
	// though y=51 is grid line 3 and row1's origin rounds to grid line 2.
	r := h.scene.Rects["row1"]
	h.mouse(30, r.Y+15, flatte.MouseLeft, flatte.MousePress, 0)
	ev := s.events[len(s.events)-1].(flatte.MouseEvent)
	if ev.Target != "row1" || ev.Y-ev.TargetRect.Y != 0 {
		t.Fatalf("mouse = %+v, want target row1 at local line 0", ev)
	}
	if ev.X-ev.TargetRect.X != 3 { // 30pt - 4pt pad = 26pt = column 3
		t.Fatalf("local column = %d, want 3", ev.X-ev.TargetRect.X)
	}
	// No target: plain grid cells.
	h.mouse(159, 159, flatte.MouseWheelDown, flatte.MousePress, flatte.ModShift)
	ev = s.events[len(s.events)-1].(flatte.MouseEvent)
	if ev.Target != "" || ev.X != 19 || ev.Y != 9 || ev.Mod != flatte.ModShift || ev.Button != flatte.MouseWheelDown {
		t.Fatalf("untargeted mouse = %+v", ev)
	}
}

func TestHostAsyncUpdatesArriveOnUIThread(t *testing.T) {
	s := &testState{content: "x"}
	h, b := newTestHost(t, s)
	h.start(80, 16)
	before := b.invalidates
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'g'})
	if s.count != 0 {
		t.Fatal("async result applied before it was posted")
	}
	b.drain(t, 1)
	if s.count != 42 {
		t.Fatalf("count = %d, want 42", s.count)
	}
	if b.invalidates < before+2 {
		t.Fatalf("invalidates = %d, want a redraw after the key and after the update", b.invalidates)
	}
}

func TestHostQuit(t *testing.T) {
	s := &testState{content: "x", quitOn: 'q'}
	h, b := newTestHost(t, s)
	h.start(80, 16)
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'})
	if b.stops != 1 {
		t.Fatalf("stops = %d after fx.Quit", b.stops)
	}
	// Ctrl-C is the app's to handle; the window quits with ⌘Q or close.
	s2 := &testState{content: "x"}
	h2, b2 := newTestHost(t, s2)
	h2.start(80, 16)
	h2.key(flatte.KeyEvent{Key: flatte.KeyCtrlC})
	if b2.stops != 0 || s2.events[len(s2.events)-1] != (flatte.KeyEvent{Key: flatte.KeyCtrlC}) {
		t.Fatalf("Ctrl-C: stops %d, events %#v", b2.stops, s2.events)
	}
	h.close()
	if !s.exited {
		t.Fatal("OnExit not called on close")
	}
}

func TestHostQuitDuringLayout(t *testing.T) {
	s := &testState{root: listTree}
	app := testApp(s)
	handle := app.Handle
	app.Handle = func(s *testState, ev flatte.Event, fx flatte.Effects[testState]) {
		handle(s, ev, fx)
		if _, ok := ev.(flatte.LayoutEvent); ok {
			fx.Quit()
		}
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() { b.stops++ }, func() {})
	t.Cleanup(h.cancel)
	h.start(160, 160)
	h.render()
	if b.stops != 1 {
		t.Fatalf("stops = %d after a quit while settling", b.stops)
	}
}

func TestHostLayoutPassesAreBounded(t *testing.T) {
	// An app whose geometry changes on every LayoutEvent still paints.
	s := &testState{}
	s.root = func(s *testState) layout.Node {
		return layout.Col{Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "grow", H: layout.Fixed(s.layouts + 1)}, Text: "x"}}}
	}
	h, _ := newTestHost(t, s)
	h.start(80, 400)
	h.render()
	if s.layouts != maxLayoutPasses {
		t.Fatalf("layouts = %d, want the %d-pass bound", s.layouts, maxLayoutPasses)
	}
}

func TestWheelAccumulator(t *testing.T) {
	var w wheelAccumulator
	got := []int{w.notches(6, 16), w.notches(6, 16), w.notches(6, 16), w.notches(-40, 16), w.notches(1e9, 0)}
	want := []int{0, 0, 1, -2, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("notches = %v, want %v", got, want)
		}
	}
}

func TestHostWheelBurstLaysOutOnce(t *testing.T) {
	views := 0
	s := &testState{root: func(*testState) layout.Node {
		views++
		return layout.Col{Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "list"}, Text: "rows"}}}
	}}
	h, _ := newTestHost(t, s)
	h.start(160, 64)
	h.render()
	r := h.scene.Rects["list"]
	h.mouse(1, 1, flatte.MouseLeft, flatte.MousePress, 0) // dirties the frame
	h.mouse(1, 1, flatte.MouseLeft, flatte.MouseRelease, 0)
	before, n := views, len(s.events)
	h.wheel(r.X+1, r.Y+1, flatte.MouseWheelDown, 5, flatte.ModShift)
	if views-before != 1 {
		t.Fatalf("a 5-notch burst laid the frame out %d times, want 1", views-before)
	}
	got := s.events[n:]
	if len(got) != 5 {
		t.Fatalf("events %#v, want 5 wheel presses", got)
	}
	for _, ev := range got {
		m, ok := ev.(flatte.MouseEvent)
		if !ok || m.Target != "list" || m.Button != flatte.MouseWheelDown || m.Action != flatte.MousePress || m.Mod != flatte.ModShift {
			t.Fatalf("event %#v", ev)
		}
	}
}
