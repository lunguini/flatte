package window

import (
	"image"
	"math"
	"reflect"
	"sync"
)

// blurredKey is a picture blurred for a box: the source, and the size it
// is shrunk to.
type blurredKey struct {
	src  image.Image
	w, h int
}

var blurs = struct {
	sync.Mutex
	m map[blurredKey]image.Image
}{m: map[blurredKey]image.Image{}}

// blurred is src as a backdrop of a w×h-point box, blurred by blur
// points: all of src stretched to the box's shape — a heavy blur hides the
// stretch, and a crop would keep only a strip of a square cover in a wide
// box — sampled at most 192 pixels wide, box-blurred three times (near a
// Gaussian) and drawn back up smoothly. Sharp (blur under 1) sources are
// src itself; results are kept per source and size.
func blurred(src image.Image, w, h, blur float64) image.Image {
	if blur < 1 || w <= 0 || h <= 0 {
		return src
	}
	b := src.Bounds()
	if b.Empty() {
		return src
	}
	per := max(w/192, 2.0) // points per pixel of the blurred image
	k := blurredKey{w: max(int(math.Round(w/per)), 2), h: max(int(math.Round(h/per)), 2)}
	if reflect.TypeOf(src).Comparable() {
		k.src = src
		blurs.Lock()
		im, ok := blurs.m[k]
		blurs.Unlock()
		if ok {
			return im
		}
	}
	cw, ch := float64(b.Dx()), float64(b.Dy())
	x0, y0 := float64(b.Min.X), float64(b.Min.Y)
	px := make([][4]float64, k.w*k.h)
	for y := range k.h {
		for x := range k.w {
			sx, sy := x0+(float64(x)+0.5)*cw/float64(k.w), y0+(float64(y)+0.5)*ch/float64(k.h)
			r, g, bl, a := src.At(int(sx), int(sy)).RGBA()
			px[y*k.w+x] = [4]float64{float64(r), float64(g), float64(bl), float64(a)}
		}
	}
	radius := max(int(math.Round(blur/per/2)), 1)
	for range 3 {
		px = boxBlur(px, k.w, k.h, radius, true)
		px = boxBlur(px, k.w, k.h, radius, false)
	}
	out := image.NewRGBA(image.Rect(0, 0, k.w, k.h))
	for i, c := range px {
		out.Pix[i*4], out.Pix[i*4+1], out.Pix[i*4+2], out.Pix[i*4+3] = uint8(c[0]/257), uint8(c[1]/257), uint8(c[2]/257), uint8(c[3]/257)
	}
	if k.src != nil {
		blurs.Lock()
		if len(blurs.m) > 64 {
			blurs.m = map[blurredKey]image.Image{}
		}
		blurs.m[k] = out
		blurs.Unlock()
	}
	return out
}

// boxBlur averages each pixel with the radius pixels on either side along
// one axis (the edge pixel repeated past the edges).
func boxBlur(px [][4]float64, w, h, radius int, horizontal bool) [][4]float64 {
	out := make([][4]float64, len(px))
	n, lines, at := w, h, func(line, i int) int { return line*w + i }
	if !horizontal {
		n, lines, at = h, w, func(line, i int) int { return i*w + line }
	}
	span := float64(2*radius + 1)
	for line := range lines {
		var sum [4]float64
		get := func(i int) [4]float64 { return px[at(line, min(max(i, 0), n-1))] }
		for i := -radius; i <= radius; i++ {
			c := get(i)
			for ch := range 4 {
				sum[ch] += c[ch]
			}
		}
		for i := range n {
			for ch := range 4 {
				out[at(line, i)][ch] = sum[ch] / span
			}
			in, gone := get(i+radius+1), get(i-radius)
			for ch := range 4 {
				sum[ch] += in[ch] - gone[ch]
			}
		}
	}
	return out
}
