package window

import (
	"image"
	"image/draw"
	"reflect"
	"sync"
	"unsafe"

	"github.com/lunguini/flatte/flatui/layout"
)

// kCGInterpolationHigh.
const cgInterpolationHigh = 3

// images holds the CGImages of pictures drawn recently, by Source: one not
// drawn in the last frame is released.
var images = struct {
	sync.Mutex
	m   map[image.Image]*cachedImage
	gen uint64 // the frame being drawn
}{m: map[image.Image]*cachedImage{}}

type cachedImage struct {
	ref  uintptr // 0: the picture could not be converted
	used uint64
}

// beginImages starts a frame's use of the image cache; endImages releases
// the pictures the frame did not draw.
func beginImages() {
	images.Lock()
	images.gen++
	images.Unlock()
}

func endImages() {
	images.Lock()
	defer images.Unlock()
	for src, c := range images.m {
		if c.used != images.gen {
			if c.ref != 0 {
				cgImageRelease(c.ref)
			}
			delete(images.m, src)
		}
	}
}

// cgImageOf is src as a CGImage, and a release for it (a no-op for a
// cached one); 0 when it cannot be converted.
func cgImageOf(src image.Image) (uintptr, func()) {
	if !reflect.TypeOf(src).Comparable() {
		ref := toCGImage(src)
		return ref, func() {
			if ref != 0 {
				cgImageRelease(ref)
			}
		}
	}
	images.Lock()
	defer images.Unlock()
	c, ok := images.m[src]
	if !ok {
		c = &cachedImage{ref: toCGImage(src)}
		images.m[src] = c
	}
	c.used = images.gen
	return c.ref, func() {}
}

// toCGImage copies src into a new CGImage (0 when empty or too large).
func toCGImage(src image.Image) uintptr {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return 0
	}
	rgba, ok := src.(*image.RGBA)
	if !ok || rgba.Rect.Min != (image.Point{}) {
		rgba = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Rect, src, b.Min, draw.Src)
	}
	cs := cgColorSpaceCreateDeviceRGB()
	defer cgColorSpaceRelease(cs)
	// image.RGBA is premultiplied RGBA, as this context.
	ctx := cgBitmapContextCreate(0, uintptr(w), uintptr(h), 8, 0, cs, cgImageAlphaPremultipliedLast|cgBitmapByteOrder32Big)
	if ctx == 0 {
		return 0
	}
	defer cgContextRelease(ctx)
	stride := int(cgBitmapContextGetBytesPer(ctx))
	dst := unsafe.Slice((*byte)(cgBitmapContextGetData(ctx)), stride*h)
	for y := range h {
		copy(dst[y*stride:y*stride+w*4], rgba.Pix[y*rgba.Stride:y*rgba.Stride+w*4])
	}
	return cgBitmapContextCreateImage(ctx)
}

// fitRect is where a w×h picture goes in r under fit.
func fitRect(r layout.PointRect, w, h float64, fit layout.ImageFit) layout.PointRect {
	if fit == layout.ImageStretch || w <= 0 || h <= 0 {
		return r
	}
	s := min(r.W/w, r.H/h)
	if fit == layout.ImageCover {
		s = max(r.W/w, r.H/h)
	}
	fw, fh := w*s, h*s
	return layout.PointRect{X: r.X + (r.W-fw)/2, Y: r.Y + (r.H-fh)/2, W: fw, H: fh}
}

// image draws op's picture, clipped to its box.
func (p *painter) image(op layout.ImageOp) {
	if op.Source == nil {
		return
	}
	ref, release := cgImageOf(op.Source)
	defer release()
	if ref == 0 {
		return
	}
	b := op.Source.Bounds()
	r := fitRect(op.Rect, float64(b.Dx()), float64(b.Dy()), op.Fit)
	// The space is flipped and CoreGraphics draws images bottom up: flip
	// the image's rect back.
	cgContextSaveGState(p.ctx)
	cgContextSetInterpolationQ(p.ctx, cgInterpolationHigh)
	cgContextTranslateCTM(p.ctx, r.X, r.Y+r.H)
	cgContextScaleCTM(p.ctx, 1, -1)
	cgContextDrawImage(p.ctx, cgRect(0, 0, r.W, r.H), ref)
	cgContextRestoreGState(p.ctx)
}
