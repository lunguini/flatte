package layout

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestImageAutoSizeUsesTextAndTextFallbacks(t *testing.T) {
	img := Image{Text: "logo"}
	if w, h := img.Size(); w != (Size{Kind: SizeContent, Value: 4}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Size = %+v %+v", w, h)
	}
	got := img.Render(Rect{W: 6, H: 1})
	if ansi.Strip(got) != "logo  " {
		t.Errorf("Render = %q", got)
	}
	if got != (Label{Text: "logo", Style: TextStyle{Role: TextSubtle}}).Render(Rect{W: 6, H: 1}) {
		t.Errorf("not drawn as a subtle label: %q", got)
	}
	// The text wraps in a box narrower than it.
	long := Image{Text: "a sun over hills"}
	if got := ansi.Strip(long.Render(Rect{W: 9, H: 2})); got != "a sun    \nover     " {
		t.Errorf("wrapped = %q", got)
	}

	// Source does not change Auto sizing: callers give art explicit boxes,
	// while Text remains the fallback description and natural size.
	withSource := Image{Source: solidImage(64, 32, color.RGBA{R: 255, A: 255}), Text: "logo"}
	if w, h := withSource.Size(); w != (Size{Kind: SizeContent, Value: 4}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Errorf("Source Auto size = %+v %+v, want Text size", w, h)
	}
	for _, tc := range []struct {
		name string
		r    Rect
	}{
		{name: "one cell wide", r: Rect{W: 1, H: 1}},
		{name: "zero rows", r: Rect{W: 2, H: 0}},
		{name: "empty width", r: Rect{W: 0, H: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withSource.Text = "logo"
			if got, want := withSource.Render(tc.r), withSource.textRender(tc.r); got != want {
				t.Errorf("fallback = %q, want %q", got, want)
			}
		})
	}
}

func TestImageColorlessProfilesUseText(t *testing.T) {
	img := Image{NodeBase: NodeBase{W: Fixed(2), H: Fixed(1)}, Source: solidImage(4, 2, color.RGBA{R: 255, A: 255}), Text: "logo"}
	want := Compose(Image{NodeBase: img.NodeBase, Text: img.Text}, 2, 1).Frame
	for _, profile := range []colorprofile.Profile{colorprofile.ASCII, colorprofile.NoTTY} {
		if got := ComposeWithColorProfile(img, 2, 1, profile).Frame; got != want {
			t.Errorf("profile %s output = %q, want text fallback %q", profile, got, want)
		}
	}
}

func TestImageResampleFitModesAndOddSizes(t *testing.T) {
	src := colorImage([][]color.RGBA{
		{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, A: 255}},
		{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, A: 255}},
	})

	contain := resampleImage(src, 4, 4, ImageContain)
	if len(contain) != 16 {
		t.Fatalf("contain pixels = %d, want 16", len(contain))
	}
	for x := 0; x < 4; x++ {
		if contain[x].Opaque || contain[12+x].Opaque {
			t.Errorf("contain letterbox rows should be transparent: top=%+v bottom=%+v", contain[x], contain[12+x])
		}
	}
	if got := contain[4+1].Color; got != (color.RGBA{G: 255, A: 255}) {
		t.Errorf("contain image pixel = %#v, want green", got)
	}

	cover := resampleImage(src, 2, 2, ImageCover)
	if got := cover[0].Color; got != (color.RGBA{G: 255, A: 255}) {
		t.Errorf("cover left pixel = %#v, want centered crop's green pixel", got)
	}
	if got := cover[1].Color; got != (color.RGBA{B: 255, A: 255}) {
		t.Errorf("cover right pixel = %#v, want centered crop's blue pixel", got)
	}

	stretched := resampleImage(src, 3, 3, ImageStretch)
	if len(stretched) != 9 {
		t.Fatalf("odd stretch = %d pixels, want 9", len(stretched))
	}
	if !stretched[4].Opaque {
		t.Errorf("odd stretch center = %+v, want opaque", stretched[4])
	}

	oddContain := resampleImage(colorImage([][]color.RGBA{{{R: 255, A: 255}, {G: 255, A: 255}}}), 3, 3, ImageContain)
	var visible int
	for _, pixel := range oddContain {
		if pixel.Opaque {
			visible++
		}
	}
	if visible != 6 {
		t.Errorf("odd contain paints %d pixels, want the nearest 3×2 fit with letterboxing", visible)
	}
}

func TestImageResampleUsesAreaAverageAndPremultipliedAlpha(t *testing.T) {
	checker := colorImage([][]color.RGBA{{
		{R: 0, G: 0, B: 0, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	}})
	if got := resampleImage(checker, 1, 1, ImageStretch)[0].Color; got != (color.RGBA{R: 128, G: 128, B: 128, A: 255}) {
		t.Errorf("downsampled checker = %#v, want area average gray", got)
	}

	alpha := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	alpha.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 0}) // hidden green must not bleed into the average
	alpha.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 128})
	got := resampleImage(alpha, 1, 1, ImageStretch)[0]
	if !got.Opaque || got.Color != (color.RGBA{R: 255, A: 255}) {
		t.Errorf("partially transparent average = %+v, want visible red without transparent green bleed", got)
	}

	transparent := colorImage([][]color.RGBA{{{B: 255, A: 0}}})
	if got := resampleImage(transparent, 1, 1, ImageStretch)[0]; got.Opaque {
		t.Errorf("fully transparent source pixel = %+v, want unpainted", got)
	}
}

func TestImageTerminalAlphaUsesHalfBlockAndPlainSpace(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.SetRGBA(1, 1, color.RGBA{B: 255, A: 255}) // bottom half only
	src.SetRGBA(2, 0, color.RGBA{R: 255, A: 255}) // top half only
	img := Image{Source: src, Fit: ImageStretch}
	if got := ansi.Strip(img.Render(Rect{W: 3, H: 1})); got != " ▄▀" {
		t.Fatalf("alpha glyphs = %q, want plain space, lower block, upper block", got)
	}
}

func TestImageTerminalGolden(t *testing.T) {
	src := colorImage([][]color.RGBA{{
		{R: 255, A: 255}, {G: 255, A: 255},
	}, {
		{B: 255, A: 255}, {R: 255, G: 255, A: 255},
	}})
	img := Image{NodeBase: NodeBase{W: Fixed(2), H: Fixed(1)}, Source: src, Fit: ImageStretch}
	got := img.Render(Rect{W: 2, H: 1})
	want, err := os.ReadFile(filepath.Join("testdata", "image-colour.golden"))
	if err != nil {
		t.Fatalf("read image golden: %v (actual output %q)", err, got)
	}
	if got != string(want) {
		t.Fatalf("image output = %q, want golden %q", got, want)
	}
}

func TestImageUsesTerminalRendererANSI256Mapping(t *testing.T) {
	// A near-red RGB value should be converted to the nearest xterm-256 entry
	// by the same cell-buffer renderer that handles other Flatte styles.
	src := solidImage(2, 2, color.RGBA{R: 244, G: 1, A: 255})
	content := ComposeWithColorProfile(Image{NodeBase: NodeBase{W: Fixed(2), H: Fixed(1)}, Source: src}, 2, 1, colorprofile.ANSI256).Frame
	buf := uv.NewScreenBuffer(2, 1)
	uv.NewStyledString(content).Draw(buf, buf.Bounds())
	var output bytes.Buffer
	renderer := uv.NewTerminalRenderer(&output, []string{"TERM=xterm-256color"})
	renderer.SetColorProfile(colorprofile.ANSI256)
	renderer.Resize(2, 1)
	renderer.Render(buf.RenderBuffer)
	if err := renderer.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, "38;5;196") || !strings.Contains(got, "48;5;196") {
		t.Fatalf("ANSI256 renderer output = %q, want nearest red in foreground and background", got)
	}
}

func TestImageTerminalCacheHitsAndEvictsLRU(t *testing.T) {
	first := &countingImage{bounds: image.Rect(0, 0, 2, 2)}
	cachedTerminalImage(first, 2, 1, ImageStretch)
	firstCalls := first.calls
	cachedTerminalImage(first, 2, 1, ImageStretch)
	if first.calls != firstCalls {
		t.Fatalf("cache hit sampled source again: calls %d -> %d", firstCalls, first.calls)
	}
	cachedTerminalImage(first, 2, 1, ImageCover)
	if first.calls == firstCalls {
		t.Fatal("Fit was not part of the cache key")
	}
	fitCalls := first.calls
	cachedTerminalImage(first, 3, 1, ImageCover)
	if first.calls == fitCalls {
		t.Fatal("content box size was not part of the cache key")
	}

	for i := 0; i < terminalImageCacheLimit; i++ {
		img := &countingImage{bounds: image.Rect(0, 0, 2, 2)}
		cachedTerminalImage(img, 2, 1, ImageFit(i%3))
	}
	// Use distinct box sizes to fill and exceed the small cache even though
	// only three Fit values exist.
	for i := 1; i <= terminalImageCacheLimit; i++ {
		img := &countingImage{bounds: image.Rect(0, 0, 2, 2)}
		cachedTerminalImage(img, 2+i, 1, ImageStretch)
	}
	before := first.calls
	cachedTerminalImage(first, 2, 1, ImageStretch)
	if first.calls == before {
		t.Fatal("oldest image remained cached after the LRU exceeded its limit")
	}
}

func TestImageTerminalCacheConcurrentUse(t *testing.T) {
	img := solidImage(4, 2, color.RGBA{R: 255, A: 255})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cachedTerminalImage(img, 12, 4, ImageContain)
		}()
	}
	wg.Wait()
}

func TestImageTerminalCacheSkipsDynamicallyUncomparableValues(t *testing.T) {
	calls := 0
	img := nestedValueImage{c: sliceColor{255, 0, 0, 255}, calls: &calls}
	if !reflect.TypeOf(img).Comparable() {
		t.Fatal("test image type should be comparable while its value is not")
	}
	if reflect.ValueOf(img).Comparable() {
		t.Fatal("test image value should be dynamically uncomparable")
	}
	cachedTerminalImage(img, 2, 1, ImageStretch)
	first := calls
	cachedTerminalImage(img, 2, 1, ImageStretch)
	if calls == first {
		t.Fatal("dynamically uncomparable image value should bypass the cache")
	}
}

func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func colorImage(rows [][]color.RGBA) *image.RGBA {
	h, w := len(rows), len(rows[0])
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range rows {
		for x := range rows[y] {
			img.SetRGBA(x, y, rows[y][x])
		}
	}
	return img
}

type countingImage struct {
	bounds image.Rectangle
	calls  int
}

func (i *countingImage) ColorModel() color.Model { return color.RGBAModel }
func (i *countingImage) Bounds() image.Rectangle { return i.bounds }
func (i *countingImage) At(x, y int) color.Color {
	i.calls++
	return color.RGBA{R: 255, A: 255}
}

type nestedValueImage struct {
	c     color.Color
	calls *int
}

func (i nestedValueImage) ColorModel() color.Model { return color.RGBAModel }
func (i nestedValueImage) Bounds() image.Rectangle { return image.Rect(0, 0, 1, 1) }
func (i nestedValueImage) At(_, _ int) color.Color {
	*i.calls = *i.calls + 1
	return i.c
}

type sliceColor []uint8

func (c sliceColor) RGBA() (r, g, b, a uint32) {
	return uint32(c[0]) * 0x101, uint32(c[1]) * 0x101, uint32(c[2]) * 0x101, uint32(c[3]) * 0x101
}

func TestImageScene(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 32))
	root := Col{Children: []Node{
		Image{NodeBase: NodeBase{ID: "auto"}, Source: src, Text: "logo"},
		Image{NodeBase: NodeBase{ID: "sized", W: Points(20), H: Points(10)}, Source: src, Fit: ImageCover},
		Image{NodeBase: NodeBase{ID: "none"}, Text: "missing"},
	}}
	s := LayoutScene(root, 400, 300, testMetrics)
	// One pixel to a point on Auto axes (the width stretches in a Col).
	if r := s.Rects["auto"]; r.H != 32 {
		t.Errorf("auto rect %+v, want 32pt tall", r)
	}
	if w, _ := (pointMetric{testMetrics}).leaf(root.Children[0], -1); w.Value != 64 {
		t.Errorf("auto width %+v, want 64pt", w)
	}
	if r := s.Rects["sized"]; r.W != 20 || r.H != 10 {
		t.Errorf("sized rect %+v", r)
	}
	var images []ImageOp
	var texts []TextOp
	for _, op := range s.Ops {
		switch op := op.(type) {
		case ImageOp:
			images = append(images, op)
		case TextOp:
			texts = append(texts, op)
		}
	}
	if len(images) != 2 || images[0].Source != image.Image(src) || images[1].Fit != ImageCover || images[1].Rect != s.Rects["sized"] {
		t.Errorf("image ops %+v", images)
	}
	// Without a picture a GUI shows the text.
	if len(texts) != 1 || texts[0].Text != "missing" || texts[0].Style.Role != TextSubtle || !texts[0].Wrap || s.Rects["none"].H != 16 {
		t.Errorf("text ops %+v, rect %+v", texts, s.Rects["none"])
	}
}
