package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// fakeText lays text out one point per byte on 16pt rows of 10 bytes, so
// carets are arithmetic; it records the goal x it is asked to keep.
type fakeText struct{ goals []float64 }

func (f *fakeText) caretAt(op layout.InputOp, x, y float64) int {
	row := max(int((y-op.Rect.Y)/16), 0)
	col := min(max(int(x-op.Rect.X), 0), 10)
	return min(row*10+col, len(op.Value))
}

func (f *fakeText) caretX(_ layout.InputOp, caret int) float64 { return float64(caret % 10) }

func (f *fakeText) caretStep(op layout.InputOp, caret, dir int, x float64) int {
	f.goals = append(f.goals, x)
	row := min(max(caret/10+dir, 0), len(op.Value)/10)
	return min(row*10+int(x), len(op.Value))
}

// textOffset lays a label out one point per byte on one line.
func (f *fakeText) textOffset(op layout.TextOp, x, _ float64) int {
	return min(max(int(x-op.Rect.X), 0), len(op.Text))
}

type fakeClip struct{ text string }

func (c *fakeClip) get() string     { return c.text }
func (c *fakeClip) set(text string) { c.text = text }

func areaState(caret *int) *testState {
	return &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{ID: "top"}, Text: "top"},
			layout.Input{NodeBase: layout.NodeBase{ID: "ed", H: layout.Fixed(3)}, Value: "0123456789abcdefghijKLMNO", Caret: *caret, Focused: true, Multiline: true},
			layout.Label{NodeBase: layout.NodeBase{ID: "below"}, Text: "below"},
		}}
	}}
}

func lastMouse(t *testing.T, s *testState) flatte.MouseEvent {
	t.Helper()
	for i := len(s.events) - 1; i >= 0; i-- {
		if m, ok := s.events[i].(flatte.MouseEvent); ok {
			return m
		}
	}
	t.Fatal("no mouse event")
	return flatte.MouseEvent{}
}

func lastKey(t *testing.T, s *testState) flatte.KeyEvent {
	t.Helper()
	for i := len(s.events) - 1; i >= 0; i-- {
		if k, ok := s.events[i].(flatte.KeyEvent); ok {
			return k
		}
	}
	t.Fatal("no key event")
	return flatte.KeyEvent{}
}

func TestHostMouseCaretAndDragCapture(t *testing.T) {
	caret := 0
	s := areaState(&caret)
	h, _ := newTestHost(t, s)
	h.text = &fakeText{}
	h.start(200, 100)
	h.render()
	ed := h.scene.Rects["ed"]
	h.mouse(ed.X+3, ed.Y+17, flatte.MouseLeft, flatte.MousePress, 0)
	if m := lastMouse(t, s); m.Target != "ed" || !m.HasCaret || m.Caret != 13 {
		t.Fatalf("press = %+v, want caret 13 on ed", m)
	}
	// Dragged below the field: still the field, caret clamped by the layout.
	h.mouse(ed.X+5, ed.Y+ed.H+30, flatte.MouseLeft, flatte.MouseMotion, 0)
	if m := lastMouse(t, s); m.Target != "ed" || !m.HasCaret || m.Caret != 25 {
		t.Fatalf("drag = %+v, want a caret on ed", m)
	}
	h.mouse(ed.X+5, ed.Y+ed.H+30, flatte.MouseLeft, flatte.MouseRelease, 0)
	if m := lastMouse(t, s); m.Target != "ed" || !m.HasCaret {
		t.Fatalf("release = %+v, want it on ed", m)
	}
	// After release, the pointer below targets what is there.
	h.mouse(ed.X+5, ed.Y+ed.H+2, flatte.MouseLeft, flatte.MouseMotion, 0)
	if m := lastMouse(t, s); m.Target != "below" || m.HasCaret {
		t.Fatalf("after release = %+v, want the label, no caret", m)
	}
	// A press elsewhere does not capture.
	h.mouse(1, 1, flatte.MouseLeft, flatte.MousePress, 0)
	h.mouse(ed.X+3, ed.Y+1, flatte.MouseLeft, flatte.MouseMotion, 0)
	if m := lastMouse(t, s); m.Target != "ed" || m.Caret != 3 {
		t.Fatalf("motion over ed after a press elsewhere = %+v", m)
	}
}

func TestHostVerticalKeysCarryCarets(t *testing.T) {
	caret := 13
	s := areaState(&caret)
	h, _ := newTestHost(t, s)
	text := &fakeText{}
	h.text = text
	h.start(200, 100)
	down := flatte.KeyEvent{Key: flatte.KeyDown}
	h.key(down)
	if k := lastKey(t, s); !k.HasCaret || k.Caret != 23 {
		t.Fatalf("down = %+v, want caret 23", k)
	}
	// The app moves to it; the next move keeps the goal x of the first.
	caret = 23
	h.key(flatte.KeyEvent{Key: flatte.KeyUp, Mod: flatte.ModShift})
	if k := lastKey(t, s); !k.HasCaret || k.Caret != 13 {
		t.Fatalf("shift-up = %+v, want caret 13", k)
	}
	// The app moved the caret itself: a new goal from where it is.
	caret = 7
	h.key(down)
	if len(text.goals) != 3 || text.goals[0] != 3 || text.goals[1] != 3 || text.goals[2] != 7 {
		t.Fatalf("goals = %v, want [3 3 7]", text.goals)
	}
	// Other keys and modified arrows carry nothing.
	h.key(flatte.KeyEvent{Key: flatte.KeyLeft})
	h.key(flatte.KeyEvent{Key: flatte.KeyDown, Mod: flatte.ModAlt})
	if k := lastKey(t, s); k.HasCaret {
		t.Fatalf("alt-down carries a caret: %+v", k)
	}
	// No focused multi-line input: nothing.
	plain := &testState{root: func(*testState) layout.Node { return layout.Label{Text: "x"} }}
	h2, _ := newTestHost(t, plain)
	h2.text = text
	h2.start(200, 100)
	h2.key(down)
	if k := lastKey(t, plain); k.HasCaret {
		t.Fatalf("down without a text area carries a caret: %+v", k)
	}
}

func TestHostClipboardAndPaste(t *testing.T) {
	var fx flatte.Effects[testState]
	s := &testState{content: "x"}
	h, b := newTestHost(t, s)
	clip := &fakeClip{text: "from the system"}
	h.clip = clip
	h.start(100, 50)
	fx = h.fx
	fx.SetClipboard("copied")
	if clip.text != "copied" {
		t.Fatalf("clipboard %q, want copied", clip.text)
	}
	fx.ReadClipboard()
	b.drain(t, 1)
	if ev, ok := s.events[len(s.events)-1].(flatte.ClipboardEvent); !ok || ev.Text != "copied" {
		t.Fatalf("last event %#v, want the clipboard's text", s.events[len(s.events)-1])
	}
	h.paste()
	if ev, ok := s.events[len(s.events)-1].(flatte.PasteEvent); !ok || ev.Text != "copied" {
		t.Fatalf("last event %#v, want a paste", s.events[len(s.events)-1])
	}
}
