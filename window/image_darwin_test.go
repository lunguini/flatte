package window

import (
	"image"
	"image/color"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestFitRect(t *testing.T) {
	box := layout.PointRect{X: 10, Y: 20, W: 100, H: 50}
	for _, c := range []struct {
		fit  layout.ImageFit
		w, h float64
		want layout.PointRect
	}{
		{layout.ImageContain, 200, 200, layout.PointRect{X: 35, Y: 20, W: 50, H: 50}},
		{layout.ImageContain, 400, 100, layout.PointRect{X: 10, Y: 32.5, W: 100, H: 25}},
		{layout.ImageCover, 200, 200, layout.PointRect{X: 10, Y: -5, W: 100, H: 100}},
		{layout.ImageStretch, 3, 7, box},
		{layout.ImageContain, 0, 7, box},
	} {
		if got := fitRect(box, c.w, c.h, c.fit); got != c.want {
			t.Errorf("fit %v %vx%v = %+v, want %+v", c.fit, c.w, c.h, got, c.want)
		}
	}
}

// quarters is a 2×2 picture: red, green on top; blue, white below.
func quarters() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(5, 5, 7, 7)) // not at the origin
	img.Set(5, 5, color.NRGBA{R: 255, A: 255})
	img.Set(6, 5, color.NRGBA{G: 255, A: 255})
	img.Set(5, 6, color.NRGBA{B: 255, A: 255})
	img.Set(6, 6, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	return img
}

func TestSnapshotImage(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	src := quarters()
	op := layout.ImageOp{Rect: layout.PointRect{X: 10, Y: 10, W: 40, H: 40}, Clip: layout.PointRect{W: 100, H: 100}, Source: src, Fit: layout.ImageStretch}
	img := renderImage(layout.Scene{Ops: []layout.Op{op}}, darkTheme, metrics(), 60, 60, 2, nil, presentation{})
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	// Upright, each quarter where the picture has it (sampled away from the
	// smoothed middle).
	for _, c := range []struct {
		x, y float64
		want color.Color
	}{
		{14, 14, color.RGBA{R: 255, A: 255}},
		{46, 14, color.RGBA{G: 255, A: 255}},
		{14, 46, color.RGBA{B: 255, A: 255}},
		{46, 46, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{55, 30, toColor(darkTheme.bg)},
	} {
		if got := at(c.x, c.y); !near(got, c.want, 40) {
			t.Errorf("at (%v, %v): %v, want %v", c.x, c.y, got, c.want)
		}
	}
	// Cover crops to its box: nothing drawn outside it.
	op.Rect, op.Fit = layout.PointRect{X: 10, Y: 10, W: 40, H: 10}, layout.ImageCover
	img = renderImage(layout.Scene{Ops: []layout.Op{op}}, darkTheme, metrics(), 60, 60, 2, nil, presentation{})
	if got := img.At(30*2, 25*2); !near(got, toColor(darkTheme.bg), 10) {
		t.Errorf("cover drew outside its box: %v", got)
	}
}

func TestImageCache(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	scene := layout.Scene{Ops: []layout.Op{layout.ImageOp{Rect: layout.PointRect{W: 4, H: 4}, Clip: layout.PointRect{W: 10, H: 10}, Source: src}}}
	cached := func() (uintptr, bool) {
		images.Lock()
		defer images.Unlock()
		c, ok := images.m[image.Image(src)]
		if !ok {
			return 0, false
		}
		return c.ref, true
	}
	renderImage(scene, darkTheme, metrics(), 10, 10, 1, nil, presentation{})
	first, ok := cached()
	if !ok || first == 0 {
		t.Fatal("not cached")
	}
	renderImage(scene, darkTheme, metrics(), 10, 10, 1, nil, presentation{})
	if again, _ := cached(); again != first {
		t.Error("converted again while still drawn")
	}
	renderImage(layout.Scene{}, darkTheme, metrics(), 10, 10, 1, nil, presentation{})
	if _, ok := cached(); ok {
		t.Error("kept after a frame without it")
	}
	// An empty picture and an uncomparable one draw nothing and leak
	// nothing.
	empty := layout.ImageOp{Rect: layout.PointRect{W: 4, H: 4}, Clip: layout.PointRect{W: 10, H: 10}, Source: image.NewRGBA(image.Rect(0, 0, 0, 0))}
	odd := layout.ImageOp{Rect: layout.PointRect{W: 4, H: 4}, Clip: layout.PointRect{W: 10, H: 10}, Source: uncomparable{image.NewRGBA(image.Rect(0, 0, 2, 2)), nil}}
	renderImage(layout.Scene{Ops: []layout.Op{empty, odd}}, darkTheme, metrics(), 10, 10, 1, nil, presentation{})
	images.Lock()
	n := len(images.m)
	images.Unlock()
	if n != 1 { // the empty one, cached as unconvertible
		t.Errorf("%d cached", n)
	}
}

// uncomparable is an image type that cannot be a map key.
type uncomparable struct {
	*image.RGBA
	_ []int
}
