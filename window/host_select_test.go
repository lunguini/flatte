package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func selectState(text *string) *testState {
	return &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{ID: "sel"}, Text: *text, Selectable: true},
			layout.Label{NodeBase: layout.NodeBase{ID: "plain"}, Text: "not selectable"},
			layout.Button{NodeBase: layout.NodeBase{ID: "btn"}, Label: "Go"},
		}}
	}}
}

func TestHostSelectsLabelText(t *testing.T) {
	text := "hello selectable world"
	s := selectState(&text)
	h, b := newTestHost(t, s)
	clip := &fakeClip{}
	h.text, h.clip = &fakeText{}, clip
	h.start(400, 200)
	h.render()
	r := h.scene.Rects["sel"]
	y := r.Y + 1
	// Press at byte 6, drag to byte 16 (fakeText: a point per byte).
	h.mouse(r.X+6, y, flatte.MouseLeft, flatte.MousePress, 0)
	h.mouse(r.X+16, y, flatte.MouseLeft, flatte.MouseMotion, 0)
	if got := h.selectedText(); got != "selectable" {
		t.Fatalf("selected %q, want selectable", got)
	}
	if b.invalidates == 0 {
		t.Error("selecting did not redraw")
	}
	// The app still saw the clicks.
	if m := lastMouse(t, s); m.Target != "sel" || m.Action != flatte.MouseMotion {
		t.Errorf("app saw %+v", m)
	}
	// Dragging past the label clamps to it; dragging back the other way
	// selects backwards.
	h.mouse(r.X+500, y+50, flatte.MouseLeft, flatte.MouseMotion, 0)
	if got := h.selectedText(); got != "selectable world" {
		t.Errorf("past the end: %q", got)
	}
	h.mouse(r.X+2, y, flatte.MouseLeft, flatte.MouseMotion, 0)
	h.mouse(r.X+2, y, flatte.MouseLeft, flatte.MouseRelease, 0)
	if got := h.selectedText(); got != "llo " {
		t.Errorf("backwards: %q", got)
	}
	// Motion after release does not extend it.
	h.mouse(r.X+10, y, flatte.MouseNone, flatte.MouseMotion, 0)
	if got := h.selectedText(); got != "llo " {
		t.Errorf("after release: %q", got)
	}
	// ⌘C copies it and is not the app's.
	n := len(s.events)
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'c', Mod: flatte.ModSuper})
	if clip.text != "llo " {
		t.Errorf("clipboard %q", clip.text)
	}
	if len(s.events) != n {
		t.Errorf("⌘C with a selection reached the app: %+v", s.events[n:])
	}
	// The painter is told what to highlight.
	if sel := h.presentation().sel; !sel.ok || sel.text != text {
		t.Errorf("presentation %+v", sel)
	}
	// A click on plain text clears it; ⌘C is then the app's.
	p := h.scene.Rects["plain"]
	h.mouse(p.X+1, p.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	h.mouse(p.X+1, p.Y+1, flatte.MouseLeft, flatte.MouseRelease, 0)
	if got := h.selectedText(); got != "" {
		t.Errorf("after a click elsewhere: %q", got)
	}
	n = len(s.events)
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'c', Mod: flatte.ModSuper})
	if len(s.events) != n+1 {
		t.Error("⌘C without a selection did not reach the app")
	}
}

func TestHostSelectionEndsWithItsLabel(t *testing.T) {
	text := "hello world"
	s := selectState(&text)
	h, _ := newTestHost(t, s)
	h.text, h.clip = &fakeText{}, &fakeClip{}
	h.start(400, 200)
	h.render()
	r := h.scene.Rects["sel"]
	h.mouse(r.X, r.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	h.mouse(r.X+5, r.Y+1, flatte.MouseLeft, flatte.MouseMotion, 0)
	if h.selectedText() != "hello" {
		t.Fatalf("selected %q", h.selectedText())
	}
	// The label's text changes: the selection goes, mid-drag too.
	text = "goodbye"
	h.render()
	if got := h.selectedText(); got != "" || h.presentation().sel.ok || h.selecting {
		t.Errorf("stale selection %q %+v", got, h.presentation().sel)
	}
	h.mouse(r.X+3, r.Y+1, flatte.MouseLeft, flatte.MouseMotion, 0)
	if got := h.selectedText(); got != "" {
		t.Errorf("drag revived it: %q", got)
	}
}

func TestHostSelectionSkipsControlsAndLayers(t *testing.T) {
	s := &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{ID: "under", H: layout.Grow(1)}, Text: "covered text", Selectable: true},
			layout.Label{NodeBase: layout.NodeBase{ID: "cover", Overlay: true, W: layout.Fixed(5), H: layout.Fixed(1)}, Text: "menu"},
		}}
	}}
	h, _ := newTestHost(t, s)
	h.text = &fakeText{}
	h.start(400, 200)
	h.render()
	c := h.scene.Rects["cover"]
	if _, _, ok := h.selectableAt(c.X+1, c.Y+1); ok {
		t.Error("selectable through an overlay")
	}
	u := h.scene.Rects["under"]
	if _, _, ok := h.selectableAt(u.X+u.W-1, u.Y+1); !ok {
		t.Error("not selectable beside the overlay")
	}
	if shape, _ := h.hover(u.X+u.W-1, u.Y+1); shape != pointerText {
		t.Errorf("pointer over selectable text %v", shape)
	}
	if shape, _ := h.hover(-1, -1); shape != pointerArrow {
		t.Errorf("pointer outside %v", shape)
	}
	text := "x"
	s2 := selectState(&text)
	h2, _ := newTestHost(t, s2)
	h2.text = &fakeText{}
	h2.start(400, 200)
	h2.render()
	btn := h2.scene.Rects["btn"]
	h2.mouse(btn.X+1, btn.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	if h2.presentation().sel.ok {
		t.Error("a button press started a selection")
	}
}
