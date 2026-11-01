package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestHostInputClicksAndClearAreAppOwned(t *testing.T) {
	field := flatui.TextField{Value: "hello world"}
	s := &testState{}
	s.root = func(*testState) layout.Node {
		in := field.Layout("field", true)
		in.Clearable = true
		return layout.Col{Children: []layout.Node{in}}
	}
	app := testApp(s)
	original := app.Handle
	app.Handle = func(s *testState, e flatte.Event, fx flatte.Effects[testState]) {
		original(s, e, fx)
		if mouse, ok := e.(flatte.MouseEvent); ok {
			field.HandleMouse("field", mouse)
		}
	}
	h, _ := newTestHostWithApp(t, app)
	h.text = &fakeText{}
	h.start(200, 80)
	h.render()
	r := h.scene.Rects["field"]
	h.mouseClicks(r.X+7, r.Y+4, flatte.MouseLeft, flatte.MousePress, 0, 2)
	if m := lastMouse(t, s); m.Clicks != 2 || field.SelectedText() != "world" {
		t.Fatalf("double click %+v selection %q", m, field.SelectedText())
	}
	h.mouseClicks(r.X+7, r.Y+4, flatte.MouseLeft, flatte.MouseRelease, 0, 2)
	if field.SelectedText() != "world" {
		t.Fatal("release collapsed selection")
	}
	h.mouseClicks(r.X+2, r.Y+4, flatte.MouseLeft, flatte.MousePress, 0, 3)
	if field.SelectedText() != field.Value {
		t.Fatal("triple click missing")
	}
	h.render()
	op, _ := h.input("field")
	clear, ok := inputClearRect(op)
	if !ok {
		t.Fatal("nonempty clearable input has no affordance")
	}
	if shape, tip := h.hover(clear.X+clear.W/2, clear.Y+clear.H/2); shape != pointerHand || tip != "Clear text" {
		t.Fatalf("clear hover %v %q", shape, tip)
	}
	h.mouse(clear.X+clear.W/2, clear.Y+clear.H/2, flatte.MouseLeft, flatte.MousePress, 0)
	if m := lastMouse(t, s); !m.Clear || m.Target != "field" || m.HasCaret || field.Value != "" || field.Cursor != 0 {
		t.Fatalf("clear event %+v field %+v", m, field)
	}
	h.render()
	op, _ = h.input("field")
	if _, ok := inputClearRect(op); ok {
		t.Fatal("empty field keeps clear affordance")
	}
}
func TestInputClearAreaRespectsClipAndSmallFields(t *testing.T) {
	op := layout.InputOp{Rect: layout.PointRect{W: 100, H: 20}, Clip: layout.PointRect{W: 90, H: 20}, Value: "text", Clearable: true}
	if inputClearAt(op, 95, 10) || !inputClearAt(op, 85, 10) {
		t.Fatal("clear hit ignores clip")
	}
	if r := inputTextRect(op); r.W != 78 {
		t.Fatalf("text overlaps clear %v", r)
	}
	op.Rect.W = 20
	if _, ok := inputClearRect(op); ok || inputTextRect(op) != op.Rect {
		t.Fatal("small field loses text area")
	}
	op.Rect.W = 100
	op.Multiline = true
	if _, ok := inputClearRect(op); ok {
		t.Fatal("multiline field claims singleline clear control")
	}
}
