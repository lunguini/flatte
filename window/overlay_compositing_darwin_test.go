package window

import (
	"github.com/lunguini/flatte/flatui/layout"
	"image"
	"image/color"
	"testing"
)

func overlayPage() []layout.Op {
	src := image.NewRGBA(image.Rect(0, 0, 120, 100))
	for y := range 100 {
		for x := range 120 {
			src.SetRGBA(x, y, color.RGBA{uint8(45 + x), uint8(90 + y), 170, 255})
		}
	}
	clip := layout.PointRect{W: 120, H: 100}
	return []layout.Op{
		layout.ImageOp{Rect: clip, Clip: clip, Source: src, Fit: layout.ImageStretch},
		layout.TextOp{Rect: layout.PointRect{X: 18, Y: 16, W: 30, H: 22}, Clip: clip, Text: "M", Style: layout.TextStyle{Size: 20}},
		layout.IconOp{Rect: layout.PointRect{X: 86, Y: 14, W: 18, H: 18}, Clip: clip, Name: layout.IconHeart},
	}
}
func TestRoundedOverlayPreservesPageOutsideItsCorners(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	const scale = 2.0
	m := metrics()
	clip := layout.PointRect{W: 120, H: 100}
	r := layout.PointRect{X: 20, Y: 20, W: 80, H: 60}
	base := overlayPage()
	before := renderImage(layout.Scene{Ops: base}, darkTheme, m, 120, 100, scale, nil, presentation{})
	ops := append([]layout.Op{}, base...)
	ops = append(ops, layout.LayerOp{Rect: r}, layout.BoxOp{Rect: r, Clip: clip, Box: layout.Box{Border: layout.BorderRounded}})
	after := renderImage(layout.Scene{Ops: ops}, darkTheme, m, 120, 100, scale, nil, presentation{})
	for _, pt := range [][2]float64{{20.5, 20.5}, {99.5, 20.5}, {20.5, 79.5}, {99.5, 79.5}, {18, 21}} {
		x, y := int(pt[0]*scale), int(pt[1]*scale)
		if got, want := after.At(x, y), before.At(x, y); !near(got, want, 1) {
			t.Errorf("page erased outside rounded surface at %v: got %v want %v", pt, got, want)
		}
	}
	if got := after.At(60*2, 50*2); !near(got, toColor(darkTheme.surface), 2) {
		t.Errorf("panel center not opaque raised surface: %v", got)
	}
}
func TestOverlayCompositingPreservesUndecoratedClearAndBoxClip(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	r := layout.PointRect{X: 20, Y: 20, W: 80, H: 60}
	all := layout.PointRect{W: 120, H: 100}
	base := overlayPage()
	plain := append(append([]layout.Op{}, base...), layout.LayerOp{Rect: r}, layout.TextOp{Rect: r, Clip: all, Text: "plain"})
	img := renderImage(layout.Scene{Ops: plain}, darkTheme, m, 120, 100, 2, nil, presentation{})
	if got := img.At(60*2, 65*2); !near(got, toColor(darkTheme.bg), 2) {
		t.Errorf("undecorated overlay lost its clear: %v", got)
	}
	before := renderImage(layout.Scene{Ops: base}, darkTheme, m, 120, 100, 2, nil, presentation{})
	clip := layout.PointRect{X: 35, Y: 20, W: 65, H: 60}
	ops := append(append([]layout.Op{}, base...), layout.LayerOp{Rect: r}, layout.BoxOp{Rect: r, Clip: clip, Box: layout.Box{Border: layout.BorderRounded}})
	img = renderImage(layout.Scene{Ops: ops}, darkTheme, m, 120, 100, 2, nil, presentation{})
	if got, want := img.At(25*2, 45*2), before.At(25*2, 45*2); !near(got, want, 1) {
		t.Errorf("layer clear escaped the surface clip: %v want %v", got, want)
	}
	if got := img.At(60*2, 45*2); !near(got, toColor(darkTheme.surface), 2) {
		t.Errorf("clipped surface missing: %v", got)
	}
}

func TestRoundedOverlayPreservesEarlierOverlayAtItsCorners(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	clip := layout.PointRect{W: 120, H: 100}
	first := layout.PointRect{X: 20, Y: 20, W: 80, H: 60}
	second := layout.PointRect{X: 40, Y: 30, W: 40, H: 35}
	ops := append(overlayPage(), layout.LayerOp{Rect: first}, layout.BoxOp{Rect: first, Clip: clip, Box: layout.Box{Border: layout.BorderRounded}})
	before := renderImage(layout.Scene{Ops: ops}, darkTheme, m, 120, 100, 2, nil, presentation{})
	pink := color.RGBA{180, 75, 130, 255}
	ops = append(ops, layout.LayerOp{Rect: second}, layout.BoxOp{Rect: second, Clip: clip, Box: layout.Box{Border: layout.BorderRounded, Background: pink}})
	after := renderImage(layout.Scene{Ops: ops}, darkTheme, m, 120, 100, 2, nil, presentation{})
	for _, pt := range [][2]float64{{40.5, 30.5}, {79.5, 30.5}, {40.5, 64.5}, {79.5, 64.5}} {
		x, y := int(pt[0]*2), int(pt[1]*2)
		if got, want := after.At(x, y), before.At(x, y); !near(got, want, 1) {
			t.Errorf("second popup erased first popup at %v: %v want %v", pt, got, want)
		}
	}
	if got := after.At(60*2, 48*2); !near(got, pink, 2) {
		t.Errorf("second popup did not paint its own surface: %v", got)
	}
}
