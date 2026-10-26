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
	if shape != pointerHand || tip != "Confirm" || h.pres.hovered != "ok" || b.invalidates != before+1 {
		t.Fatalf("over the button: shape %v tip %q hovered %q redraws %d", shape, tip, h.pres.hovered, b.invalidates-before)
	}
	h.hover(at("ok")) // unchanged: no redraw
	if b.invalidates != before+1 {
		t.Fatal("hovering the same control redrew")
	}
	if shape, _ := h.hover(at("off")); shape != pointerArrow || h.pres.hovered != "" {
		t.Fatalf("over a disabled button: shape %v hovered %q", shape, h.pres.hovered)
	}
	if shape, _ := h.hover(at("name")); shape != pointerText {
		t.Fatalf("over an input: shape %v, want text", shape)
	}
	if shape, tip := h.hover(at("text")); shape != pointerArrow || tip != "" {
		t.Fatalf("over a label: shape %v tip %q", shape, tip)
	}
	h.hover(at("ok"))
	h.hover(-1, -1) // left the view
	if h.pres.hovered != "" {
		t.Fatalf("left the view, hovered %q", h.pres.hovered)
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
	if pressed := h.presentation().pressed; pressed != "ok" {
		t.Fatalf("pressed %q, want ok", pressed)
	}
	if m := lastMouse(t, s); m.Target != "ok" {
		t.Fatalf("the click reached the app as %+v", m)
	}
	h.mouse(ok.X+1, ok.Y+1, flatte.MouseLeft, flatte.MouseRelease, 0)
	if pressed := h.presentation().pressed; pressed != "" {
		t.Fatalf("released, still pressed %q", pressed)
	}
	h.mouse(off.X+1, off.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	if pressed := h.presentation().pressed; pressed != "" {
		t.Fatalf("a disabled button shows pressed")
	}
}

func TestHostHoverIgnoresClippedAndCoveredRows(t *testing.T) {
	s := &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: "head"},
			layout.Scroll{NodeBase: layout.NodeBase{H: layout.Fixed(2)}, OffsetY: 1, Child: layout.Col{Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "hidden", H: layout.Fixed(1)}, Text: "hidden"},
				layout.Label{NodeBase: layout.NodeBase{ID: "row", H: layout.Fixed(1)}, Text: "row"},
				layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: "bottom"},
			}}},
			layout.Label{NodeBase: layout.NodeBase{ID: "modal", Overlay: true, W: layout.Fixed(4), H: layout.Fixed(1)}, Text: "modal"},
		}}
	}}
	h, _ := newTestHost(t, s)
	h.start(200, 60)
	h.render()
	r := h.scene.Rects["hidden"]
	h.hover(r.X+1, r.Y+1)
	if h.over["hidden"] {
		t.Fatal("clipped row hovered")
	}
	r = h.scene.Rects["modal"]
	h.hover(r.X+1, r.Y+1)
	if h.over["row"] || !h.over["modal"] {
		t.Fatalf("covered row hovered: %v", h.over)
	}
}

func TestHostInteractiveBoxHoverIsDefaultPresentation(t *testing.T) {
	state := &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Row{NodeBase: layout.NodeBase{ID: "item", Box: layout.Box{Interactive: true, Round: true}}, Children: []layout.Node{layout.Label{Text: "Choose"}}},
			layout.Row{NodeBase: layout.NodeBase{ID: "plain"}, Children: []layout.Node{layout.Label{Text: "Plain"}}},
		}}
	}}
	host, _ := newTestHost(t, state)
	host.start(200, 100)
	host.render()
	rect := host.scene.Rects["item"]
	shape, _ := host.hover(rect.X+1, rect.Y+1)
	if shape != pointerHand || !host.over["item"] {
		t.Fatalf("interactive row shape=%v over=%v", shape, host.over)
	}
	if len(state.events) != 0 {
		for _, event := range state.events {
			if _, ok := event.(flatte.MouseEvent); ok {
				t.Fatal("hover wrote app state")
			}
		}
	}
	rect = host.scene.Rects["plain"]
	if shape, _ := host.hover(rect.X+1, rect.Y+1); shape != pointerArrow {
		t.Fatal("ordinary row became interactive")
	}
}

func TestInteractiveItemHoverRespectsScrollAndOverlay(t *testing.T) {
	state := &testState{root: func(*testState) layout.Node {
		item := func(id string) layout.Node {
			return layout.Row{NodeBase: layout.NodeBase{ID: id, H: layout.Fixed(1), Box: layout.Box{Interactive: true}}, Children: []layout.Node{layout.Label{Text: id}}}
		}
		return layout.Col{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: "header"},
			layout.Scroll{NodeBase: layout.NodeBase{H: layout.Fixed(2)}, OffsetY: 1, Child: layout.Col{Children: []layout.Node{item("hidden-item"), item("visible-item"), item("last-item")}}},
			layout.Label{NodeBase: layout.NodeBase{ID: "cover", Overlay: true, W: layout.Fixed(4), H: layout.Fixed(1)}, Text: "cover"},
		}}
	}}
	host, _ := newTestHost(t, state)
	host.start(200, 60)
	host.render()
	hidden := host.scene.Rects["hidden-item"]
	if shape, _ := host.hover(hidden.X+1, hidden.Y+1); shape != pointerArrow || host.over["hidden-item"] {
		t.Fatal("clipped item gained hover")
	}
	visible := host.scene.Rects["visible-item"]
	if shape, _ := host.hover(visible.X+1, visible.Y+1); shape != pointerHand {
		t.Fatal("visible item lacks hover")
	}
	cover := host.scene.Rects["cover"]
	if shape, _ := host.hover(cover.X+1, cover.Y+1); shape != pointerArrow || host.over["visible-item"] {
		t.Fatal("item behind overlay gained hover")
	}
}
