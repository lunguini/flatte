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
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 9}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 30*m.Column, 9*m.Line, m)
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
	if c := at(box.X+8+indW/2-4, box.Y+box.H/2); !near(c, accent, 12) {
		t.Errorf("checked checkbox %v, want the accent", c)
	}
	on, off := s.Rects["on"], s.Rects["offsw"]
	swW := float64(indicatorCols(layout.ControlSwitch)) * m.Column
	if c := at(on.X+8+(swW-switchW)/2+3, on.Y+on.H/2); !near(c, accent, 30) {
		t.Errorf("switch on %v, want the accent track", c)
	}
	if c := at(off.X+8+(swW-switchW)/2+switchW-4, off.Y+off.H/2); !near(c, toColor(darkTheme.lk.field), 12) {
		t.Errorf("switch off %v, want the field track", c)
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
	// Dark washes are white: the pressed wash is the stronger one.
	if !(lum(down) > lum(hot)) {
		t.Errorf("pressed %v not stronger than hovered %v", down, hot)
	}
	if other := fill("other", "other"); other != plain {
		t.Errorf("another control's hover changed this one: %v", other)
	}
}

// Controls draw the design recipe: a secondary button is outlined with no
// fill, focus is a ring in the theme's focus color, a focused field's
// outline turns the focus color.
func TestControlRecipe(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	root := layout.Col{NodeBase: layout.NodeBase{Pad: 1}, Children: []layout.Node{
		layout.Row{Children: []layout.Node{layout.Button{NodeBase: layout.NodeBase{ID: "b"}, Label: "Button", Focused: true}, layout.NewSpacer()}},
		layout.Row{Children: []layout.Node{layout.Input{NodeBase: layout.NodeBase{ID: "in", W: layout.Fixed(12)}, Value: "x", Focused: true}, layout.NewSpacer()}},
	}}
	scene := layout.LayoutScene(root, 30*m.Column, 6*m.Line, m)
	img := renderImage(scene, darkTheme, m, 30*m.Column, 6*m.Line, 2, nil, presentation{})
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	b, in := scene.Rects["b"], scene.Rects["in"]
	if c := at(b.X+b.W/2, b.Y+b.H/2+5); !near(c, toColor(darkTheme.bg), 6) {
		t.Errorf("secondary button inside %v, want no fill", c)
	}
	if c := at(b.X+b.W/2, b.Y+1.5); !near(c, toColor(darkTheme.lk.border), 30) {
		t.Errorf("secondary button edge %v, want the control border", c)
	}
	focus := toColor(darkTheme.lk.focus)
	if c := at(b.X+b.W/2, b.Y); !near(c, focus, 40) {
		t.Errorf("button focus ring %v, want %v", c, focus)
	}
	if c := at(in.X+in.W/2, in.Y+1.5); !near(c, focus, 40) {
		t.Errorf("focused field outline %v, want %v", c, focus)
	}
}

// A Box.Edge is a band of the accent along its side, the focus stroke
// thick.
func TestBoxEdge(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	root := layout.Row{Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{ID: "tab", W: layout.Fixed(8), Box: layout.Box{Fill: layout.FillContent, Edge: layout.SideTop}}, Text: "tab"},
		layout.Label{NodeBase: layout.NodeBase{ID: "row", W: layout.Fixed(8), Box: layout.Box{Edge: layout.SideLeft}}, Text: "row"},
	}}
	scene := layout.LayoutScene(root, 20*m.Column, 2*m.Line, m)
	img := renderImage(scene, darkTheme, m, 20*m.Column, 2*m.Line, 2, nil, presentation{})
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	tab, row := scene.Rects["tab"], scene.Rects["row"]
	accent := toColor(darkTheme.lk.accent)
	if c := at(tab.X+tab.W/2, tab.Y+0.5); !near(c, accent, 6) {
		t.Errorf("tab top %v, want the accent", c)
	}
	if c := at(tab.X+tab.W/2, tab.Y+3); near(c, accent, 30) {
		t.Errorf("edge thicker than the focus stroke: %v", c)
	}
	if c := at(row.X+0.5, row.Y+row.H/2); !near(c, accent, 6) {
		t.Errorf("row left %v, want the accent", c)
	}
}

// A Round box with a Flat side rounds only the other corners, and its Edge
// stays inside the rounded shape.
func TestFlatSideRoundsTheOtherCorners(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	root := layout.Row{Children: []layout.Node{
		layout.Col{NodeBase: layout.NodeBase{ID: "tab", W: layout.Points(60), H: layout.Points(30), Box: layout.Box{Fill: layout.FillAccent, Round: true, Flat: layout.SideBottom, Edge: layout.SideTop}}},
	}}
	scene := layout.LayoutScene(root, 100, 40, m)
	img := renderImage(scene, darkTheme, m, 100, 40, 2, nil, presentation{})
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	r := scene.Rects["tab"]
	bg, accent := toColor(darkTheme.bg), toColor(darkTheme.lk.accent)
	if c := at(r.X+0.5, r.Y+0.5); !near(c, bg, 8) {
		t.Errorf("top-left corner %v, want rounded off (background)", c)
	}
	if c := at(r.X+r.W-0.5, r.Y+0.5); !near(c, bg, 8) {
		t.Errorf("top-right corner %v, want rounded off", c)
	}
	if c := at(r.X+0.5, r.Y+r.H-0.5); !near(c, accent, 8) {
		t.Errorf("bottom-left corner %v, want square (filled)", c)
	}
	if c := at(r.X+r.W-0.5, r.Y+r.H-0.5); !near(c, accent, 8) {
		t.Errorf("bottom-right corner %v, want square", c)
	}
	if c := at(r.X+r.W/2, r.Y+r.H/2); !near(c, accent, 8) {
		t.Errorf("middle %v", c)
	}
}
