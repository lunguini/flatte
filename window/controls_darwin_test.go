package window

import (
	"image/color"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestSnapshotControls(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Row{Children: []layout.Node{
				layout.Button{NodeBase: layout.NodeBase{ID: "save"}, Label: "Save", Default: true},
				layout.Button{NodeBase: layout.NodeBase{ID: "off"}, Label: "Save", Default: true, Disabled: true},
			}},
			layout.Toggle{NodeBase: layout.NodeBase{ID: "box"}, Label: "x", Checked: true},
			layout.Toggle{NodeBase: layout.NodeBase{ID: "on"}, Kind: layout.ToggleSwitch, Checked: true},
			layout.Toggle{NodeBase: layout.NodeBase{ID: "offsw"}, Kind: layout.ToggleSwitch},
			layout.Slider{NodeBase: layout.NodeBase{ID: "vol", W: layout.Fixed(20)}, Value: 0.5},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 6}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 30*m.Column, 6*m.Line, m)
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	mid := func(id string, fx float64) color.Color { r := s.Rects[id]; return at(r.X+r.W*fx, r.Y+r.H/2) }
	accent := toColor(darkTheme.caret)
	if c := mid("save", 0.1); !near(c, accent, 12) {
		t.Errorf("default button fill %v, want the accent", c)
	}
	if c := mid("off", 0.1); near(c, accent, 30) {
		t.Errorf("disabled default button %v, want it dimmed", c)
	}
	box := s.Rects["box"]
	indW := float64(indicatorCols(layout.ControlCheckbox)) * m.Column
	if c := at(box.X+indW/2-4, box.Y+box.H/2); !near(c, accent, 12) {
		t.Errorf("checked checkbox %v, want the accent", c)
	}
	on, off := s.Rects["on"], s.Rects["offsw"]
	swW := float64(indicatorCols(layout.ControlSwitch)) * m.Column
	if c := at(on.X+(swW-switchW)/2+3, on.Y+on.H/2); !near(c, accent, 30) {
		t.Errorf("switch on %v, want the accent track", c)
	}
	if c := at(off.X+(swW-switchW)/2+switchW-3, off.Y+off.H/2); !near(c, toColor(darkTheme.track), 30) {
		t.Errorf("switch off %v, want the plain track", c)
	}
	vol := s.Rects["vol"]
	if c := at(vol.X+knobSize, vol.Y+vol.H/2); !near(c, accent, 30) {
		t.Errorf("slider before the knob %v, want the fill", c)
	}
	if c := at(vol.X+vol.W-knobSize, vol.Y+vol.H/2); !near(c, toColor(darkTheme.track), 30) {
		t.Errorf("slider after the knob %v, want the track", c)
	}
}

func TestViewTracksThePointer(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	cls, err := registerViewClass()
	if err != nil {
		t.Fatal(err)
	}
	view := objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), cgRect(0, 0, 10, 10))
	defer view.Send(sel("release"))
	for _, name := range []string{"mouseMoved:", "mouseExited:"} {
		if !objc.Send[bool](view, sel("respondsToSelector:"), sel(name)) {
			t.Errorf("view does not respond to %s", name)
		}
	}
}

func TestControlHoverAndPress(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	root := layout.Row{Children: []layout.Node{layout.Button{NodeBase: layout.NodeBase{ID: "b"}, Label: "Button"}, layout.NewSpacer()}}
	scene := layout.LayoutScene(root, 20*m.Column, m.Line, m)
	r := scene.Rects["b"]
	fill := func(hovered, pressed string) color.Color {
		img := renderImage(scene, darkTheme, m, 20*m.Column, m.Line, 2, nil, presentation{hovered: hovered, pressed: pressed})
		return img.At(int((r.X+3)*2), int((r.Y+r.H/2)*2))
	}
	lum := func(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return r + g + b }
	plain, hot, down := fill("", ""), fill("b", ""), fill("b", "b")
	if !(lum(hot) > lum(plain)) {
		t.Errorf("hovered %v not lighter than plain %v", hot, plain)
	}
	if !(lum(down) < lum(plain)) {
		t.Errorf("pressed %v not darker than plain %v", down, plain)
	}
	if other := fill("other", "other"); other != plain {
		t.Errorf("another control's hover changed this one: %v", other)
	}
}
