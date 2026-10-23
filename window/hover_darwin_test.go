package window

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestHostHoverFadesNestedNodes(t *testing.T) {
	now := time.Unix(100, 0)
	s := &testState{root: func(*testState) layout.Node {
		return layout.Col{Children: []layout.Node{layout.Row{NodeBase: layout.NodeBase{ID: "row", Box: layout.Box{Hover: color.White}}, Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "heart"}, Label: "♥"},
			layout.Label{Text: "song"},
		}}}}
	}}
	h, _ := newTestHost(t, s)
	h.surface.clock = func() time.Time { return now }
	h.start(200, 60)
	h.render()
	r := h.scene.Rects["heart"]
	h.hover(r.X+1, r.Y+1) // over the heart, inside the row
	p := h.presentation()
	if p.hovers["row"] != 0 || !p.moving {
		t.Fatalf("at the start: %v moving %v", p.hovers, p.moving)
	}
	now = now.Add(hoverFade)
	p = h.presentation()
	if p.hovers["row"] != 1 || p.hovers["heart"] != 1 || p.moving {
		t.Fatalf("faded in: %v moving %v", p.hovers, p.moving)
	}
	// A press shows pressed on what is under it, until released.
	h.mouse(r.X+1, r.Y+1, flatte.MouseLeft, flatte.MousePress, 0)
	now = now.Add(pressFade)
	if p := h.presentation(); p.presses["heart"] != 1 {
		t.Fatalf("pressed %v", p.presses)
	}
	h.hover(-1, -1)
	now = now.Add(hoverFade / 2)
	if p := h.presentation(); p.hovers["row"] <= 0 || p.hovers["row"] >= 1 || !p.moving {
		t.Fatalf("fading out: %v", p.hovers)
	}
}

func TestPainterHoverAndBackdrop(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	red := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for i := range red.Pix {
		red.Pix[i] = []uint8{0xe0, 0x30, 0x30, 0xff}[i%4]
	}
	root := layout.Col{Children: []layout.Node{
		layout.Row{NodeBase: layout.NodeBase{ID: "row", H: layout.Fixed(1), Box: layout.Box{Hover: color.NRGBA{0xff, 0xff, 0xff, 0x80}}}, Children: []layout.Node{layout.NewSpacer()}},
		layout.Row{NodeBase: layout.NodeBase{ID: "head", H: layout.Fixed(3), Box: layout.Box{Backdrop: &layout.Backdrop{Source: red, Blur: 20}}}, Children: []layout.Node{layout.NewSpacer()}},
	}}
	scene := layout.LayoutScene(root, 20*m.Column, 4*m.Line, m)
	at := func(pres presentation, id string) color.Color {
		r := scene.Rects[id]
		img := renderImage(scene, darkTheme, m, 20*m.Column, 4*m.Line, 1, nil, pres)
		return img.At(int(r.X+r.W/2), int(r.Y+r.H/2))
	}
	bg := at(presentation{}, "row")
	half := at(presentation{hovers: map[string]float64{"row": 0.5}}, "row")
	full := at(presentation{hovers: map[string]float64{"row": 1}}, "row")
	lum := func(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return r + g + b }
	if !(lum(bg) < lum(half) && lum(half) < lum(full)) {
		t.Fatalf("hover fill by level: none %v half %v full %v", bg, half, full)
	}
	if got := at(presentation{}, "head"); !near(got, color.RGBA{0xe0, 0x30, 0x30, 0xff}, 0x18) {
		t.Fatalf("backdrop %v, want the picture's red", got)
	}
}

func TestEqualizerMoves(t *testing.T) {
	a := equalizer(10, 10, 20, time.Unix(0, 0))
	b := equalizer(10, 10, 20, time.Unix(0, 2e8))
	if len(a.polys) != 3 || a.polys[0].pts[0] == b.polys[0].pts[0] {
		t.Fatalf("bars did not move: %v %v", a.polys[0].pts[0], b.polys[0].pts[0])
	}
	for _, p := range append(a.polys, b.polys...) {
		for _, pt := range p.pts {
			if pt.Y < 10-10 || pt.Y > 10+10 {
				t.Fatalf("bar outside its box: %+v", pt)
			}
		}
	}
}
