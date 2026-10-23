package window

import (
	"image"
	"image/color"
	"testing"
)

func TestBlurredSmoothsAndKeepsColor(t *testing.T) {
	// Half red, half blue: blurred, the middle mixes, the ends keep their color.
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := range 100 {
		for x := range 200 {
			c := color.RGBA{0xff, 0, 0, 0xff}
			if x >= 100 {
				c = color.RGBA{0, 0, 0xff, 0xff}
			}
			src.SetRGBA(x, y, c)
		}
	}
	im := blurred(src, 400, 200, 40)
	b := im.Bounds()
	if b.Dx() > 192 || b.Dx() < 100 {
		t.Fatalf("blurred to %v", b)
	}
	mid := color.RGBAModel.Convert(im.At(b.Dx()/2, b.Dy()/2)).(color.RGBA)
	if mid.R < 60 || mid.B < 60 {
		t.Fatalf("middle %v is not a mix", mid)
	}
	end := color.RGBAModel.Convert(im.At(1, b.Dy()/2)).(color.RGBA)
	if end.R < 0xe0 || end.B > 0x20 {
		t.Fatalf("left end %v is not red", end)
	}
	// Neighbouring pixels differ little: no steps.
	for x := 1; x < b.Dx(); x++ {
		a := color.RGBAModel.Convert(im.At(x-1, b.Dy()/2)).(color.RGBA)
		c := color.RGBAModel.Convert(im.At(x, b.Dy()/2)).(color.RGBA)
		if d := int(a.R) - int(c.R); d > 24 || d < -24 {
			t.Fatalf("a step of %d at x=%d", d, x)
		}
	}
	if blurred(src, 400, 200, 40) != im {
		t.Fatal("not kept")
	}
	if blurred(src, 400, 200, 0) != image.Image(src) {
		t.Fatal("no blur copied it")
	}
}
