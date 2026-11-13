package layout

import (
	"container/list"
	"image"
	"image/color"
	"math"
	"reflect"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// ImageFit is how an Image's picture fills its box in a GUI or terminal.
type ImageFit int

const (
	// ImageContain scales the picture to fit inside the box, keeping its
	// aspect ratio, centered.
	ImageContain ImageFit = iota
	// ImageCover scales the picture to cover the box, keeping its aspect ratio,
	// centered and cropped.
	ImageCover
	// ImageStretch scales the picture to the box exactly.
	ImageStretch
)

// Image is a picture. A GUI draws Source in its content box as Fit says, one
// image pixel to a point on Auto axes. A terminal draws Source as half-block
// color cells when it has color and the box is at least two cells wide; Text
// remains the subtle, wrapped fallback and supplies the natural size on Auto
// axes. Give art an explicit box size. Each terminal cell represents two
// vertically stacked image pixels. A Round Box does not round terminal art.
// A GUI reads Source when it first draws it: to change the picture, give a
// new image rather than changing its pixels.
type Image struct {
	NodeBase
	Source image.Image
	Text   string
	Fit    ImageFit
}

func (i Image) Size() (Size, Size) { return i.label().Size() }

// Render draws the image with an unspecified (color-capable) terminal
// profile. Frame composition supplies the output terminal's actual profile.
func (i Image) Render(r Rect) string {
	return i.renderWithColorProfile(r, colorprofile.Unknown)
}

func (i Image) renderWithColorProfile(r Rect, profile colorprofile.Profile) string {
	content := r.insetBy(i.insetSides())
	if isNilImage(i.Source) || content.W < 2 || content.H < 1 || i.pixelsInvalid() || profile == colorprofile.ASCII || profile == colorprofile.NoTTY {
		return i.textRender(r)
	}
	_, art := cachedTerminalImage(i.Source, content.W, content.H, i.Fit)
	return fillBox(art, r, i.padSides(), i.box())
}

func (i Image) textRender(r Rect) string {
	l := i.label()
	l.Wrap = true
	return l.Render(r)
}

func (i Image) label() Label {
	return Label{NodeBase: i.NodeBase, Text: i.Text, Style: TextStyle{Role: TextSubtle}}
}

// pixels is Source's size in pixels (0×0 without one).
func (i Image) pixels() (int, int) {
	if isNilImage(i.Source) {
		return 0, 0
	}
	b := i.Source.Bounds()
	return b.Dx(), b.Dy()
}

func (i Image) pixelsInvalid() bool {
	w, h := i.pixels()
	return w <= 0 || h <= 0
}

type terminalPixel struct {
	Color  color.RGBA
	Opaque bool
}

// resampleImage returns a targetW×targetH pixel image. Fit is evaluated in
// pixel space; a terminal's caller asks for two target rows per cell row.
// Downscaling integrates each source pixel's area (a box filter), and colors
// are averaged in premultiplied-alpha space to keep transparent RGB from
// bleeding into visible pixels. Nonzero alpha becomes a solid terminal color;
// fully transparent pixels stay unpainted.
func resampleImage(src image.Image, targetW, targetH int, fit ImageFit) []terminalPixel {
	if isNilImage(src) || targetW <= 0 || targetH <= 0 || targetH > maxInt/targetW {
		return nil
	}
	b := src.Bounds()
	sourceW, sourceH := b.Dx(), b.Dy()
	if sourceW <= 0 || sourceH <= 0 {
		return nil
	}
	pixels := make([]terminalPixel, targetW*targetH)
	put := func(x, y int, sx0, sy0, sx1, sy1 float64) {
		pixels[y*targetW+x] = averageSourceArea(src, b, sx0, sy0, sx1, sy1)
	}

	switch fit {
	case ImageStretch:
		for y := 0; y < targetH; y++ {
			sy0 := float64(y) * float64(sourceH) / float64(targetH)
			sy1 := float64(y+1) * float64(sourceH) / float64(targetH)
			for x := 0; x < targetW; x++ {
				sx0 := float64(x) * float64(sourceW) / float64(targetW)
				sx1 := float64(x+1) * float64(sourceW) / float64(targetW)
				put(x, y, sx0, sy0, sx1, sy1)
			}
		}
	case ImageCover:
		scale := math.Max(float64(targetW)/float64(sourceW), float64(targetH)/float64(sourceH))
		visibleW, visibleH := float64(targetW)/scale, float64(targetH)/scale
		startX, startY := (float64(sourceW)-visibleW)/2, (float64(sourceH)-visibleH)/2
		for y := 0; y < targetH; y++ {
			sy0 := startY + float64(y)/scale
			sy1 := startY + float64(y+1)/scale
			for x := 0; x < targetW; x++ {
				sx0 := startX + float64(x)/scale
				sx1 := startX + float64(x+1)/scale
				put(x, y, sx0, sy0, sx1, sy1)
			}
		}
	default: // ImageContain also handles unknown values conservatively.
		scale := math.Min(float64(targetW)/float64(sourceW), float64(targetH)/float64(sourceH))
		drawW := max(1, min(targetW, int(math.Round(float64(sourceW)*scale))))
		drawH := max(1, min(targetH, int(math.Round(float64(sourceH)*scale))))
		offsetX, offsetY := (targetW-drawW)/2, (targetH-drawH)/2
		for y := offsetY; y < offsetY+drawH; y++ {
			sy0 := float64(y-offsetY) * float64(sourceH) / float64(drawH)
			sy1 := float64(y-offsetY+1) * float64(sourceH) / float64(drawH)
			for x := offsetX; x < offsetX+drawW; x++ {
				sx0 := float64(x-offsetX) * float64(sourceW) / float64(drawW)
				sx1 := float64(x-offsetX+1) * float64(sourceW) / float64(drawW)
				put(x, y, sx0, sy0, sx1, sy1)
			}
		}
	}
	return pixels
}

func averageSourceArea(src image.Image, bounds image.Rectangle, x0, y0, x1, y1 float64) terminalPixel {
	sourceW, sourceH := bounds.Dx(), bounds.Dy()
	x0, y0 = max(x0, 0), max(y0, 0)
	x1, y1 = min(x1, float64(sourceW)), min(y1, float64(sourceH))
	if !(x1 > x0) || !(y1 > y0) {
		return terminalPixel{}
	}

	var red, green, blue, alpha float64
	for y := int(math.Floor(y0)); y < int(math.Ceil(y1)); y++ {
		yWeight := min(y1, float64(y+1)) - max(y0, float64(y))
		for x := int(math.Floor(x0)); x < int(math.Ceil(x1)); x++ {
			xWeight := min(x1, float64(x+1)) - max(x0, float64(x))
			weight := xWeight * yWeight
			r, g, b, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			red += float64(r) * weight
			green += float64(g) * weight
			blue += float64(b) * weight
			alpha += float64(a) * weight
		}
	}
	if alpha == 0 {
		return terminalPixel{}
	}
	return terminalPixel{
		Color: color.RGBA{
			R: averageChannel(red, alpha),
			G: averageChannel(green, alpha),
			B: averageChannel(blue, alpha),
			A: 255,
		},
		Opaque: true,
	}
}

func averageChannel(channel, alpha float64) uint8 {
	return uint8(min(255, max(0, math.Round(channel/alpha*255))))
}

func terminalArt(pixels []terminalPixel, width, rows int) string {
	if width <= 0 || rows <= 0 || rows > maxInt/2 || len(pixels) != width*rows*2 {
		return ""
	}
	var out strings.Builder
	for row := 0; row < rows; row++ {
		if row > 0 {
			out.WriteByte('\n')
		}
		for x := 0; x < width; x++ {
			top := pixels[(row*2)*width+x]
			bottom := pixels[(row*2+1)*width+x]
			switch {
			case top.Opaque && bottom.Opaque:
				out.WriteString(lipgloss.NewStyle().Foreground(top.Color).Background(bottom.Color).Render("▀"))
			case top.Opaque:
				out.WriteString(lipgloss.NewStyle().Foreground(top.Color).Render("▀"))
			case bottom.Opaque:
				// The lower-half block leaves the transparent top half untouched.
				out.WriteString(lipgloss.NewStyle().Foreground(bottom.Color).Render("▄"))
			default:
				out.WriteByte(' ')
			}
		}
	}
	return out.String()
}

const terminalImageCacheLimit = 16

type terminalImageKey struct {
	source image.Image
	width  int
	height int
	fit    ImageFit
}

type terminalImageEntry struct {
	key    terminalImageKey
	pixels []terminalPixel
	art    string
}

var terminalImageCache = struct {
	sync.Mutex
	entries map[terminalImageKey]*list.Element
	lru     list.List
}{entries: make(map[terminalImageKey]*list.Element)}

// cachedTerminalImage keeps the expensive resampling and styled image string
// for a small number of immutable Sources. Non-comparable value images bypass
// the map because Go cannot safely use them as identity keys.
func cachedTerminalImage(src image.Image, width, height int, fit ImageFit) ([]terminalPixel, string) {
	if isNilImage(src) || width <= 0 || height <= 0 || height > maxInt/2 || width > maxInt/(height*2) {
		return nil, ""
	}
	sourceValue := reflect.ValueOf(src)
	if !sourceValue.IsValid() || !sourceValue.Comparable() {
		pixels := resampleImage(src, width, height*2, fit)
		return pixels, terminalArt(pixels, width, height)
	}
	key := terminalImageKey{source: src, width: width, height: height, fit: fit}
	terminalImageCache.Lock()
	if element := terminalImageCache.entries[key]; element != nil {
		terminalImageCache.lru.MoveToFront(element)
		entry := element.Value.(*terminalImageEntry)
		terminalImageCache.Unlock()
		return entry.pixels, entry.art
	}
	terminalImageCache.Unlock()

	pixels := resampleImage(src, width, height*2, fit)
	entry := &terminalImageEntry{key: key, pixels: pixels, art: terminalArt(pixels, width, height)}
	terminalImageCache.Lock()
	defer terminalImageCache.Unlock()
	if element := terminalImageCache.entries[key]; element != nil {
		terminalImageCache.lru.MoveToFront(element)
		cached := element.Value.(*terminalImageEntry)
		return cached.pixels, cached.art
	}
	element := terminalImageCache.lru.PushFront(entry)
	terminalImageCache.entries[key] = element
	if terminalImageCache.lru.Len() > terminalImageCacheLimit {
		oldest := terminalImageCache.lru.Back()
		old := oldest.Value.(*terminalImageEntry)
		delete(terminalImageCache.entries, old.key)
		terminalImageCache.lru.Remove(oldest)
	}
	return entry.pixels, entry.art
}

func isNilImage(src image.Image) bool {
	if src == nil {
		return true
	}
	v := reflect.ValueOf(src)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
