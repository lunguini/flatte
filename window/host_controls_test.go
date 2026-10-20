package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func controlsState() *testState {
	return &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "ok", Tooltip: "Confirm"}, Label: "OK"},
			layout.Button{NodeBase: layout.NodeBase{ID: "off"}, Label: "Off", Disabled: true},
			layout.Input{NodeBase: layout.NodeBase{ID: "name"}, Value: "x"},
			layout.Label{NodeBase: layout.NodeBase{ID: "text"}, Text: "plain"},
		}}
	}}
}

func TestHostHoverPointerAndTooltip(t *testing.T) {
	s := controlsState()
	h, b := newTestHost(t, s)
	h.start(200, 100)
	h.render()
	at := func(id string) (float64, float64) { r := h.scene.Rects[id]; return r.X + 1, r.Y + 1 }

	before := b.invalidates
	shape, tip := h.hover(at("ok"))
	if shape != pointerHand || tip != "Confirm" || h.hovered != "ok" || b.invalidates != before+1 {
		t.Fatalf("over the button: shape %v tip %q hovered %q redraws %d", shape, tip, h.hovered, b.invalidates-before)
	}
	h.hover(at("ok")) // unchanged: no redraw
	if b.invalidates != before+1 {
		t.Fatal("hovering the same control redrew")
	}
	if shape, _ := h.hover(at("off")); shape != pointerArrow || h.hovered != "" {
		t.Fatalf("over a disabled button: shape %v hovered %q", shape, h.hovered)
	}
	if shape, _ := h.hover(at("name")); shape != pointerText {
		t.Fatalf("over an input: shape %v, want text", shape)
	}
	if shape, tip := h.hover(at("text")); shape != pointerArrow || tip != "" {
		t.Fatalf("over a label: shape %v tip %q", shape, tip)
	}
	h.hover(at("ok"))
	h.hover(-1, -1) // left the view
	if h.hovered != "" {
		t.Fatalf("left the view, hovered %q", h.hovered)
	}
	// Hover is presentation: the app saw no events for it.
	for _, ev := range s.events {
		if _, ok := ev.(flatte.MouseEvent); ok {
			t.Fatalf("hover delivered %+v", ev)
		}
	}
}

func TestHostPressedControl(t *testing.T) {
	s := controlsState()
	h, _ := newTestHost(t, s)
	h.start(200, 100)
	h.render()
	ok, off := h.scene.Rects["ok"], h.scene.Rects["off"]
	h.mouse(ok.X+1, ok.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	if _, pressed := h.controlState(); pressed != "ok" {
		t.Fatalf("pressed %q, want ok", pressed)
	}
	if m := lastMouse(t, s); m.Target != "ok" {
		t.Fatalf("the click reached the app as %+v", m)
	}
	h.mouse(ok.X+1, ok.Y+1, flatte.MouseLeft, flatte.MouseRelease, 0)
	if _, pressed := h.controlState(); pressed != "" {
		t.Fatalf("released, still pressed %q", pressed)
	}
	h.mouse(off.X+1, off.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	if _, pressed := h.controlState(); pressed != "" {
		t.Fatalf("a disabled button shows pressed")
	}
}
