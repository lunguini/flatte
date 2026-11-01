package window

import (
	"bytes"
	"github.com/lunguini/flatte/flatui/layout"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

func TestInteractiveHoverUsesThemeAndExplicitOverrides(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	luminance := func(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return r + g + b }
	for _, test := range []struct {
		name    string
		theme   Theme
		lighter bool
	}{{"dark", Dark(), true}, {"light", Light(), false}} {
		t.Run(test.name, func(t *testing.T) {
			th := newTheme(test.theme)
			m := th.metrics()
			draw := func(box layout.Box, hover bool) color.Color {
				root := layout.Row{NodeBase: layout.NodeBase{ID: "item", Box: box}, Children: []layout.Node{layout.NewSpacer()}}
				scene := layout.LayoutScene(root, 200, 32, m)
				pres := presentation{}
				if hover {
					pres.hovers = map[string]float64{"item": 1}
				}
				image := renderImage(scene, th, m, 200, 32, 1, nil, pres)
				return image.At(100, 16)
			}
			item := layout.Box{Interactive: true, Round: true}
			plain, hot := draw(item, false), draw(item, true)
			if test.lighter && luminance(hot) <= luminance(plain) || !test.lighter && luminance(hot) >= luminance(plain) {
				t.Fatalf("default hover not theme-aware plain=%v hot=%v", plain, hot)
			}
			item.Hover = color.Transparent
			if got := draw(item, true); !near(got, plain, 1) {
				t.Fatalf("transparent override painted: %v want %v", got, plain)
			}
			item.Hover = color.NRGBA{R: 255, A: 128}
			if got := draw(item, true); near(got, hot, 2) {
				t.Fatal("explicit hover color ignored")
			}
			if got := draw(layout.Box{}, true); !near(got, plain, 1) {
				t.Fatal("plain box acquired hover")
			}
		})
	}
}

func TestRoundedListHoverMatchesSelectionRadius(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	th := newTheme(Dark())
	m := th.metrics()
	render := func(box layout.Box) *image.RGBA {
		rect := layout.PointRect{W: 100, H: 100}
		scene := layout.Scene{Ops: []layout.Op{layout.BoxOp{Rect: rect, Clip: rect, Box: box, ID: "item"}}}
		pres := presentation{}
		if box.Interactive {
			pres.hovers = map[string]float64{"item": 1}
		}
		return renderImage(scene, th, m, 100, 100, 1, nil, pres)
	}
	selected := render(layout.Box{Selected: true, Interactive: true, Round: true})
	hovered := render(layout.Box{Interactive: true, Round: true})
	playing := render(layout.Box{Background: color.NRGBA{R: 180, G: 40, B: 80, A: 255}, Interactive: true, Round: true})
	decorative := render(layout.Box{Background: color.NRGBA{R: 180, G: 40, B: 80, A: 255}, Round: true})
	plain := render(layout.Box{})
	// Interactive selection, hover and playing fills share the theme radius;
	// ordinary decorative rounded fills retain their pill radius.
	base := plain.At(6, 0)
	for name, img := range map[string]*image.RGBA{"selected": selected, "hovered": hovered, "playing": playing} {
		if near(img.At(6, 0), base, 1) || !near(img.At(0, 0), plain.At(0, 0), 1) {
			t.Errorf("%s interactive radius differs: base=%v at6=%v at0=%v", name, base, img.At(6, 0), img.At(0, 0))
		}
	}
	if near(decorative.At(6, 0), selected.At(6, 0), 1) || near(decorative.At(11, 0), plain.At(11, 0), 1) {
		t.Fatalf("decorative radius changed/matched interactive: base6=%v decorative6=%v selected6=%v base11=%v decorative11=%v", base, decorative.At(6, 0), selected.At(6, 0), plain.At(11, 0), decorative.At(11, 0))
	}
}

func TestExplicitDecorativeHoverKeepsBoxAndPictureRadii(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	th := newTheme(Dark())
	m := th.metrics()
	fill := color.NRGBA{R: 180, G: 40, B: 80, A: 255}
	picture := image.NewNRGBA(image.Rect(0, 0, 20, 100))
	draw.Draw(picture, picture.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	render := func(box layout.Box, hover bool, rect layout.PointRect) *image.RGBA {
		scene := layout.Scene{Ops: []layout.Op{layout.BoxOp{Rect: rect, Clip: rect, Box: box, ID: "item"}}}
		pres := presentation{}
		if hover {
			pres.hovers = map[string]float64{"item": 1}
		}
		return renderImage(scene, th, m, rect.W, rect.H, 1, nil, pres)
	}
	for _, test := range []struct {
		name       string
		background layout.Box
		hover      layout.Box
	}{
		{
			name:       "decorative rounded box",
			background: layout.Box{Background: fill, Round: true},
			hover:      layout.Box{Hover: fill, Round: true},
		},
		{
			name:       "picture backdrop",
			background: layout.Box{Backdrop: &layout.Backdrop{Source: picture}, Round: true},
			hover:      layout.Box{Backdrop: &layout.Backdrop{Source: picture}, Hover: fill, Round: true},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			rect := layout.PointRect{W: 100, H: 100}
			base := render(test.background, false, rect)
			hot := render(test.hover, true, rect)
			if test.name == "decorative rounded box" {
				if !bytes.Equal(base.Pix, hot.Pix) {
					t.Fatal("explicit hover changed the existing rounded geometry")
				}
				return
			}
			// Picture radius is 2.4pt on this narrow source. At x=5 the
			// rounded backdrop and hover both paint; the old 10pt pill would
			// clip that point. The top-left corner remains clipped in both.
			rect = layout.PointRect{W: 20, H: 100}
			base = render(test.background, false, rect)
			hot = render(test.hover, true, rect)
			for _, img := range []*image.RGBA{base, hot} {
				if !near(img.At(5, 0), fill, 10) || near(img.At(0, 0), fill, 20) {
					t.Fatalf("picture radius changed: inside=%v corner=%v", img.At(5, 0), img.At(0, 0))
				}
			}
		})
	}
}
