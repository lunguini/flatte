package window

import (
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"testing"
)

func TestWindowDefaultPaddingFieldAndCaret(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "button"}, Label: "Save"},
			layout.Input{NodeBase: layout.NodeBase{ID: "field"}, Value: "abc", Focused: true},
		}}
	}
	m := metrics()
	if !m.DefaultPadding {
		t.Fatal("native window does not enable default padding")
	}
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 6}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 30*m.Column, 6*m.Line, m)
	var input layout.InputOp
	for _, op := range s.Ops {
		if in, ok := op.(layout.InputOp); ok {
			input = in
		}
	}
	outer := s.Rects["field"]
	if input.Field != outer || input.Rect.X != outer.X+8 || input.Rect.Y != outer.Y+4 || input.Rect.H != outer.H-8 {
		t.Fatalf("field/content mismatch: %+v outer %v", input, outer)
	}
	fl := darkTheme.layoutField(input, "")
	if fl.x0 != input.Rect.X || fl.y0 != input.Rect.Y {
		t.Fatalf("text has extra padding: x=%v y=%v content=%v", fl.x0, fl.y0, input.Rect)
	}
	text := windowText{th: func() theme { return darkTheme }}
	if got := text.caretAt(input, input.Rect.X, input.Rect.Y+input.Rect.H/2); got != 0 {
		t.Fatalf("left-edge caret=%d", got)
	}
	x := input.Rect.X + darkTheme.styleFont(input.Style).width("abc") + 1
	if got := text.caretAt(input, x, input.Rect.Y+input.Rect.H/2); got != len(input.Value) {
		t.Fatalf("end caret=%d", got)
	}
	if id, _ := s.HitTest(outer.X+1, outer.Y+1); id != "field" {
		t.Fatalf("field padding lost hit target %q", id)
	}
	// Padding remains editable: hit testing targets the full field, while
	// the native text layout clamps left/right margin clicks to text ends.
	for _, tc := range []struct {
		x    float64
		want int
	}{{outer.X + 1, 0}, {outer.X + outer.W - 1, len(input.Value)}} {
		if got := text.caretAt(input, tc.x, outer.Y+outer.H/2); got != tc.want {
			t.Errorf("margin click x=%v caret=%d want %d", tc.x, got, tc.want)
		}
	}
	state := &testState{root: func(*testState) layout.Node { return root() }}
	h, _ := newTestHost(t, state)
	h.metrics = m
	h.text = text
	h.start(30*m.Column, 6*m.Line)
	h.render()
	for _, x := range []float64{outer.X + 1, outer.X + outer.W - 1} {
		if shape, _ := h.hover(x, outer.Y+outer.H/2); shape != pointerText {
			t.Errorf("margin x=%v pointer=%v", x, shape)
		}
		h.mouse(x, outer.Y+outer.H/2, flatte.MouseLeft, flatte.MousePress, 0)
		event := lastMouse(t, state)
		if event.Target != "field" || !event.HasCaret {
			t.Errorf("margin click %+v", event)
		}
		h.mouse(x, outer.Y+outer.H/2, flatte.MouseLeft, flatte.MouseRelease, 0)
	}

	if !region(img, layout.PointRect{X: outer.X + outer.W/2, Y: outer.Y, W: 1, H: 2}, 2, toColor(darkTheme.caret), 40) {
		t.Fatal("padded field bezel was clipped away")
	}
	if !region(img, input.Rect, 2, toColor(darkTheme.text), 40) {
		t.Fatal("padded field text missing")
	}
}

func TestNativeRegularControlsUseThemeHeight(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	for _, minimum := range []float64{32, 40} {
		th := newTheme(Theme{ControlHeight: minimum})
		m := th.metrics()
		if m.ControlHeight != minimum {
			t.Fatalf("metric height %v want %v", m.ControlHeight, minimum)
		}
		root := layout.Col{Align: layout.AlignStart, Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "button"}, Label: "Save"},
			layout.Input{NodeBase: layout.NodeBase{ID: "field"}, Value: "abc"},
			layout.Toggle{NodeBase: layout.NodeBase{ID: "toggle"}, Label: "Enabled"},
			layout.Button{NodeBase: layout.NodeBase{ID: "icon"}, Icon: layout.IconSearch, Label: "Search"},
			layout.Input{NodeBase: layout.NodeBase{ID: "bare"}, Bare: true},
		}}
		scene := layout.LayoutScene(root, 300, 300, m)
		for _, id := range []string{"button", "field", "toggle", "icon"} {
			if scene.Rects[id].H != minimum {
				t.Errorf("height %v %s %v", minimum, id, scene.Rects[id])
			}
		}
		if r := scene.Rects["icon"]; r.W != r.H {
			t.Fatalf("icon is not square: %v", r)
		}
		if scene.Rects["bare"].H != m.Line {
			t.Fatalf("Bare field minimum applied: %v", scene.Rects["bare"])
		}
		button, field := controlBezel(scene.Rects["button"]), controlBezel(scene.Rects["field"])
		if button.Size.Height != field.Size.Height {
			t.Fatal("regular bezels use different heights")
		}
	}
}

func TestNativeButtonAndFieldPaintAlignedBezels(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Row{Align: layout.AlignStart, Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "button", W: layout.Points(100)}, Label: "Save"},
			layout.Input{NodeBase: layout.NodeBase{ID: "field", W: layout.Points(100)}, Value: "abc"},
		}}
	}
	img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	m := metrics()
	s := layout.LayoutScene(root(), 40*m.Column, 4*m.Line, m)
	firstBorder := func(id string) int {
		r := s.Rects[id]
		x := int((r.X + r.W/2) * 2)
		for y := 0; y < int(r.H*2); y++ {
			if near(img.At(x, int(r.Y*2)+y), toColor(darkTheme.border), 40) {
				return y
			}
		}
		return -1
	}
	button, field := firstBorder("button"), firstBorder("field")
	if button < 0 || field < 0 || button != field {
		t.Fatalf("native borders not aligned: button pixel %d field pixel %d", button, field)
	}
}
