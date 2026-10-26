package layout

import (
	"image/color"
	"testing"
)

func TestPointDefaultPadding(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	scene := LayoutScene(Col{Align: AlignStart, Children: []Node{
		Button{NodeBase: NodeBase{ID: "button"}, Label: "OK"},
		Input{NodeBase: NodeBase{ID: "input"}, Value: "abc"},
		Input{NodeBase: NodeBase{ID: "bare"}, Bare: true, Value: "abc"},
		Button{NodeBase: NodeBase{ID: "explicit", PadLeft: 2}, Label: "OK"},
		Button{NodeBase: NodeBase{ID: "optout", NoPadding: true}, Label: "OK"},
	}}, 200, 200, m)
	if got := scene.Rects["button"]; got.W != 28 || got.H != 24 {
		t.Errorf("button=%v want 28x24", got)
	}
	if got := scene.Rects["input"]; got.H != 24 {
		t.Errorf("input=%v want height 24", got)
	}
	if got := scene.Rects["bare"]; got.H != 16 {
		t.Errorf("bare=%v want height 16", got)
	}
	if got := scene.Rects["explicit"]; got.W != 24 || got.H != 16 {
		t.Errorf("explicit=%v want 24x16", got)
	}
	if got := scene.Rects["optout"]; got.W != 12 || got.H != 16 {
		t.Errorf("optout=%v want 12x16", got)
	}
	for _, op := range scene.Ops {
		switch o := op.(type) {
		case InputOp:
			if o.ID == "input" {
				r := scene.Rects["input"]
				if o.Field != r || o.Rect != (PointRect{r.X + 8, r.Y + 4, r.W - 16, r.H - 8}) {
					t.Errorf("input field/content=%+v", o)
				}
			}
		case ControlOp:
			if o.ID == "button" {
				r := scene.Rects["button"]
				if o.Rect != r || o.Content != (PointRect{r.X + 8, r.Y + 4, r.W - 16, r.H - 8}) {
					t.Errorf("button bezel/content=%+v", o)
				}
			}
		}
	}
}

func TestPointContainerDefaultPaddingAndOverride(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	for _, tc := range []struct {
		nb    NodeBase
		inset float64
	}{
		{NodeBase{Bordered: true}, 13},
		{NodeBase{Bordered: true, PadLeft: 1}, 7},
		{NodeBase{Bordered: true, NoPadding: true}, 1},
		{NodeBase{}, 0},
		{NodeBase{Box: Box{Background: color.White, Backdrop: &Backdrop{}}}, 0},
	} {
		s := LayoutScene(Row{NodeBase: tc.nb, Children: []Node{Label{NodeBase: NodeBase{ID: "child"}, Text: "x"}}}, 100, 100, m)
		if got := s.Rects["child"].X; got != tc.inset {
			t.Errorf("%+v child X=%v want %v", tc.nb, got, tc.inset)
		}
	}
}

func TestPointFixedIconButtonKeepsOuterSize(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	s := LayoutScene(Row{Align: AlignStart, Children: []Node{Button{NodeBase: NodeBase{ID: "play", W: Points(40), H: Points(40)}, Label: "play", Icon: IconPlay}}}, 100, 100, m)
	for _, op := range s.Ops {
		if c, ok := op.(ControlOp); ok {
			if c.Rect != (PointRect{0, 0, 40, 40}) || c.Content != (PointRect{8, 4, 24, 32}) {
				t.Fatalf("control %+v", c)
			}
		}
	}
	if id, _ := s.HitTest(1, 1); id != "play" {
		t.Fatalf("padding lost hit target: %q", id)
	}
}

func TestItemPaddingIsNativeOnlyAndCanBeOverridden(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	for _, tc := range []struct {
		nb   NodeBase
		x, y float64
	}{
		{NodeBase{ItemPadding: true}, 8, 4},
		{NodeBase{ItemPadding: true, NoPadding: true}, 0, 0},
		{NodeBase{ItemPadding: true, PadLeft: 1}, m.Step, 0},
	} {
		root := Col{Children: []Node{Row{NodeBase: tc.nb, Children: []Node{Label{NodeBase: NodeBase{ID: "label"}, Text: "x"}}}}}
		scene := LayoutScene(root, 100, 100, m)
		if r := scene.Rects["label"]; r.X != tc.x || r.Y != tc.y {
			t.Fatalf("item insets %+v label %v", tc.nb, r)
		}
	}
	plain := Row{Children: []Node{Label{Text: "x"}}}
	item := plain
	item.ItemPadding = true
	if Compose(item, 20, 3).Frame != Compose(plain, 20, 3).Frame {
		t.Fatal("item padding changed terminal rendering")
	}
}
