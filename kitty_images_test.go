package flatte

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestKittyProtocolBytes(t *testing.T) {
	if got, want := kittyTransmit(7, []byte{0x00, 0xff, 0x10}), "\x1b_Ga=t,f=100,t=d,i=7,m=0,q=1;AP8Q\x1b\\"; got != want {
		t.Fatalf("transmit = %q, want %q", got, want)
	}
	if got, want := kittyPlace(7, 9, layout.Rect{X: 2, Y: 3, W: 4, H: 5}), "\x1b_Ga=p,i=7,p=9,c=4,r=5,C=1,z=1,q=1\x1b\\"; got != want {
		t.Fatalf("place = %q, want %q", got, want)
	}
	if got, want := kittyDeletePlacement(7, 9), "\x1b_Ga=d,d=i,i=7,p=9,q=1\x1b\\"; got != want {
		t.Fatalf("delete placement = %q, want %q", got, want)
	}
	if got, want := kittyDeleteImage(7), "\x1b_Ga=d,d=I,i=7,q=1\x1b\\"; got != want {
		t.Fatalf("delete image = %q, want %q", got, want)
	}
}

func TestKittyTransmitChunksAt4096Base64Bytes(t *testing.T) {
	got := kittyTransmit(11, bytes.Repeat([]byte{0}, 3075))
	first := "\x1b_Ga=t,f=100,t=d,i=11,m=1,q=1;" + strings.Repeat("A", 4096) + "\x1b\\"
	last := "\x1b_Ga=t,f=100,t=d,i=11,m=0,q=1;AAAA\x1b\\"
	if want := first + last; got != want {
		t.Fatalf("chunked transmit differs from exact 4096-byte continuation sequence (got length %d, want %d)", len(got), len(want))
	}
}

func TestKittyDetectionMatrix(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
		want bool
	}{
		{"kitty window", []string{"KITTY_WINDOW_ID=3"}, true},
		{"kitty TERM", []string{"TERM=xterm-kitty"}, true},
		{"Ghostty", []string{"TERM_PROGRAM=ghostty"}, true},
		{"WezTerm", []string{"TERM_PROGRAM=WezTerm"}, true},
		{"Warp", []string{"TERM_PROGRAM=WarpTerminal"}, true},
		{"ordinary terminal", []string{"TERM=xterm-256color"}, false},
		{"tmux disabled", []string{"KITTY_WINDOW_ID=3", "TMUX=/tmp/tmux-1/default,2,0"}, false},
		{"screen disabled", []string{"TERM=screen-256color"}, false},
		{"opt out", []string{"TERM_PROGRAM=ghostty", "FLATTE_IMAGES=cells"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := kittyImagesEnabled(tc.env); got != tc.want {
				t.Fatalf("kittyImagesEnabled(%v) = %t, want %t", tc.env, got, tc.want)
			}
		})
	}
}

func TestKittyImageDiffSkipsUnchangedAndDeletesMovedOrRemoved(t *testing.T) {
	img := kittyTestImage(color.RGBA{R: 255, A: 255})
	plan := layout.TerminalImagePlacement{Rect: layout.Rect{X: 2, Y: 1, W: 1, H: 1}, Source: img, Fit: layout.ImageStretch}
	renderer := &kittyBufferRenderer{}
	state := newKittyImageState(true)
	state.nextImageID, state.nextPlacementID = 7, 9

	state.update([]layout.TerminalImagePlacement{plan}, renderer)
	pngBytes := kittyTestPNG(t, img, 8, 16, layout.ImageStretch)
	want := kittyTransmit(7, pngBytes) + "\x1b[2;3H" + kittyPlace(7, 9, plan.Rect) + "\x1b[1;1H"
	if got := renderer.String(); got != want {
		t.Fatalf("first frame = %q, want %q", got, want)
	}

	renderer.Reset()
	state.update([]layout.TerminalImagePlacement{plan}, renderer)
	if got := renderer.String(); got != "" {
		t.Fatalf("unchanged frame output = %q, want empty", got)
	}

	renderer.Reset()
	moved := plan
	moved.Rect.X++
	state.update([]layout.TerminalImagePlacement{moved}, renderer)
	want = kittyDeletePlacement(7, 9) + "\x1b[2;4H" + kittyPlace(7, 10, moved.Rect) + "\x1b[1;1H"
	if got := renderer.String(); got != want {
		t.Fatalf("move output = %q, want %q", got, want)
	}

	renderer.Reset()
	state.update(nil, renderer)
	if got, want := renderer.String(), kittyDeletePlacement(7, 10); got != want {
		t.Fatalf("remove output = %q, want %q", got, want)
	}
}

func TestKittyImagesShareUploadAndCleanupOwnedIDs(t *testing.T) {
	img := kittyTestImage(color.RGBA{B: 255, A: 255})
	plans := []layout.TerminalImagePlacement{
		{Rect: layout.Rect{W: 1, H: 1}, Source: img, Fit: layout.ImageContain},
		{Rect: layout.Rect{X: 3, W: 1, H: 1}, Source: img, Fit: layout.ImageContain},
	}
	renderer := &kittyBufferRenderer{}
	state := newKittyImageState(true)
	state.nextImageID, state.nextPlacementID = 4, 20
	state.update(plans, renderer)
	got := renderer.String()
	if count := strings.Count(got, "\x1b_Ga=t,"); count != 1 {
		t.Fatalf("same source upload count = %d, want one: %q", count, got)
	}
	if count := strings.Count(got, "\x1b_Ga=p,"); count != 2 {
		t.Fatalf("placement count = %d, want two: %q", count, got)
	}

	renderer.Reset()
	state.clear(renderer)
	if got, want := renderer.String(), kittyDeleteImage(4); got != want {
		t.Fatalf("cleanup = %q, want %q", got, want)
	}
	if len(state.assets) != 0 || len(state.placements) != 0 {
		t.Fatalf("state after cleanup retains assets=%d placements=%d", len(state.assets), len(state.placements))
	}
}

func TestKittyImageCacheEvictsLeastRecentlyUsedUnplacedImage(t *testing.T) {
	first := kittyTestImage(color.RGBA{R: 1, A: 255})
	second := kittyTestImage(color.RGBA{G: 2, A: 255})
	state := newKittyImageState(true)
	state.nextImageID, state.nextPlacementID = 1, 100
	renderer := &kittyBufferRenderer{}
	placement := func(x int, img image.Image) layout.TerminalImagePlacement {
		return layout.TerminalImagePlacement{Rect: layout.Rect{X: x, W: 1, H: 1}, Source: img, Fit: layout.ImageStretch}
	}

	state.update([]layout.TerminalImagePlacement{placement(0, first), placement(1, second)}, renderer)
	renderer.Reset()
	state.update([]layout.TerminalImagePlacement{placement(0, first)}, renderer) // Touch first; second is now the older unplaced asset.

	for i := 0; i < kittyImageLimit-1; i++ {
		renderer.Reset()
		img := kittyTestImage(color.RGBA{R: uint8(10 + i), G: uint8(i + 1), A: 255})
		state.update([]layout.TerminalImagePlacement{placement(0, first), placement(1, img)}, renderer)
	}
	if got := renderer.String(); !strings.Contains(got, kittyDeleteImage(2)) {
		t.Fatalf("cache did not delete the least recently used image id 2: %q", got)
	}
	if len(state.assets) > kittyImageLimit || state.assetBytes > kittyImageBytesLimit || len(state.memo) > kittyImageMemoLimit {
		t.Fatalf("cache bounds exceeded: assets=%d bytes=%d memo=%d", len(state.assets), state.assetBytes, len(state.memo))
	}
}

func TestKittyUnsupportedEmitsNoEscapes(t *testing.T) {
	renderer := &kittyBufferRenderer{}
	state := newKittyImageState(false)
	state.update([]layout.TerminalImagePlacement{{Rect: layout.Rect{W: 1, H: 1}, Source: kittyTestImage(color.RGBA{A: 255})}}, renderer)
	state.clear(renderer)
	if got := renderer.String(); got != "" {
		t.Fatalf("unsupported terminal output = %q, want no image escapes", got)
	}
}

func TestRunTerminalImagesDoNotWriteGraphicsWithoutTTY(t *testing.T) {
	for _, key := range []string{"KITTY_WINDOW_ID", "TERM", "TERM_PROGRAM", "TMUX", "FLATTE_IMAGES"} {
		t.Setenv(key, "")
	}
	t.Setenv("TERM", "xterm-256color")

	for _, tc := range []struct {
		name    string
		term    string
		program string
		want    bool
	}{
		{name: "unsupported", term: "xterm-256color", want: false},
		{name: "Ghostty hint with redirected output", term: "xterm-256color", program: "ghostty", want: false},
		{name: "opt out", term: "xterm-kitty", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			t.Setenv("TERM_PROGRAM", tc.program)
			t.Setenv("KITTY_WINDOW_ID", "")
			t.Setenv("TMUX", "")
			t.Setenv("FLATTE_IMAGES", "")
			if tc.name == "opt out" {
				t.Setenv("FLATTE_IMAGES", "cells")
			}

			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			var output bytes.Buffer
			done := make(chan error, 1)
			go func() {
				done <- Run(t.Context(), App[struct{}]{
					State: &struct{}{},
					Handle: func(_ *struct{}, event Event, fx Effects[struct{}]) {
						if key, ok := event.(KeyEvent); ok && key.Rune == 'q' {
							fx.Quit()
						}
					},
					View: func(*struct{}, RenderContext) Frame {
						return Frame{Root: layout.Image{
							NodeBase: layout.NodeBase{W: layout.Fixed(2), H: layout.Fixed(1)},
							Source:   kittyTestImage(color.RGBA{R: 255, A: 255}),
						}}
					},
				}, WithInput(reader), WithOutput(&output))
			}()
			if _, err := writer.Write([]byte("q")); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Run did not stop after q")
			}
			got := strings.Contains(output.String(), "\x1b_G")
			if got != tc.want {
				t.Fatalf("image protocol emitted = %t, want %t; output %q", got, tc.want, output.String())
			}
		})
	}
}

func TestKittyRuntimeImagesRespectWindowMenuAndDialogCoverage(t *testing.T) {
	img := kittyTestImage(color.RGBA{G: 255, A: 255})
	main := layout.Composition{TerminalImages: []layout.TerminalImagePlacement{
		{Rect: layout.Rect{X: 1, Y: 1, W: 1, H: 1}, Source: img},
		{Rect: layout.Rect{X: 6, Y: 5, W: 1, H: 1}, Source: img},
	}}
	windows := &windowHost{open: []*hostWindow{{
		x: 0, y: 0, w: 2, h: 1,
		comp: layout.Composition{TerminalImages: []layout.TerminalImagePlacement{{
			Rect: layout.Rect{W: 1, H: 1}, Source: img,
		}}},
	}}}
	placements := kittyPlacementsForRuntime(main, windows, nil, nil, 10, 8)
	if len(placements) != 2 || placements[0].Rect != (layout.Rect{X: 6, Y: 5, W: 1, H: 1}) || placements[1].Rect != (layout.Rect{X: 1, Y: 1, W: 1, H: 1}) {
		t.Fatalf("window-composited placements = %#v, want the unobscured base image and translated window image", placements)
	}

	menus := &menuHost{levels: []menuLevel{{x: 5, y: 4, w: 3, h: 2}}}
	placements = kittyPlacementsForRuntime(main, windows, menus, nil, 10, 8)
	if len(placements) != 1 || placements[0].Rect != (layout.Rect{X: 1, Y: 1, W: 1, H: 1}) {
		t.Fatalf("menu-composited placements = %#v, want only the window image", placements)
	}

	dialogs := &termDialogs{cur: &termDialog{x: 0, y: 0, w: 4, h: 3}}
	placements = kittyPlacementsForRuntime(main, windows, nil, dialogs, 10, 8)
	if len(placements) != 1 || placements[0].Rect != (layout.Rect{X: 6, Y: 5, W: 1, H: 1}) {
		t.Fatalf("dialog-composited placements = %#v, want only the image outside the dialog", placements)
	}
}

func TestKittyPNGFitModes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	src.SetRGBA(1, 0, color.RGBA{B: 255, A: 255})
	for _, tc := range []struct {
		fit     layout.ImageFit
		w, h    int
		alphaAt image.Point
	}{
		{layout.ImageContain, 4, 4, image.Pt(0, 0)},
		{layout.ImageCover, 2, 2, image.Pt(0, 0)},
		{layout.ImageStretch, 4, 4, image.Point{}},
	} {
		data := kittyTestPNG(t, src, tc.w, tc.h, tc.fit)
		decoded, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if got := decoded.Bounds().Size(); got != image.Pt(tc.w, tc.h) {
			t.Fatalf("fit %d PNG bounds = %v, want %dx%d", tc.fit, got, tc.w, tc.h)
		}
		_, _, _, alpha := decoded.At(tc.alphaAt.X, tc.alphaAt.Y).RGBA()
		switch tc.fit {
		case layout.ImageContain:
			if alpha != 0 {
				t.Fatalf("contain corner alpha = %d, want transparent padding", alpha)
			}
		case layout.ImageCover, layout.ImageStretch:
			if alpha == 0 {
				t.Fatalf("fit %d corner alpha = 0, want filled target", tc.fit)
			}
		}
	}
}

type kittyBufferRenderer struct {
	bytes.Buffer
	x, y int
}

func (r *kittyBufferRenderer) Position() (int, int) { return r.x, r.y }

func (r *kittyBufferRenderer) MoveTo(x, y int) {
	if x == r.x && y == r.y {
		return
	}
	fmt.Fprintf(&r.Buffer, "\x1b[%d;%dH", y+1, x+1)
	r.x, r.y = x, y
}

func (r *kittyBufferRenderer) WriteString(s string) (int, error) { return r.Buffer.WriteString(s) }

func (r *kittyBufferRenderer) Reset() {
	r.Buffer.Reset()
	r.x, r.y = 0, 0
}

func kittyTestImage(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func kittyTestPNG(t *testing.T, src image.Image, width, height int, fit layout.ImageFit) []byte {
	t.Helper()
	data, err := prepareKittyPNG(src, width, height, fit)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestKittyPixelSizePreparesDenseButNotAboveSource(t *testing.T) {
	for _, tc := range []struct {
		rect   layout.Rect
		source image.Rectangle
		w, h   int
	}{
		{layout.Rect{W: 4, H: 2}, image.Rect(0, 0, 96, 96), 64, 64},      // a thumbnail: kittySmallDensity
		{layout.Rect{W: 6, H: 3}, image.Rect(0, 0, 640, 640), 192, 192},  // past kittySmallCells: 4×
		{layout.Rect{W: 16, H: 8}, image.Rect(0, 0, 640, 640), 512, 512}, // capped at kittyDensity
		{layout.Rect{W: 16, H: 8}, image.Rect(0, 0, 64, 64), 128, 128},   // never below nominal
	} {
		w, h, ok := kittyPixelSize(tc.rect, tc.source)
		if !ok || w != tc.w || h != tc.h {
			t.Errorf("kittyPixelSize(%v, %v) = %d×%d %v, want %d×%d", tc.rect, tc.source, w, h, ok, tc.w, tc.h)
		}
	}
}

func TestClearUnderKittyImagesKeepsBackgrounds(t *testing.T) {
	screen := uv.NewScreenBuffer(4, 2)
	uv.NewStyledString("\x1b[41m▀▀\x1b[0m▄▄\n▀▀▀▀").Draw(screen, screen.Bounds())
	clearUnderKittyImages(screen, []layout.TerminalImagePlacement{{Rect: layout.Rect{X: 1, W: 2, H: 1}}, {Rect: layout.Rect{X: 3, Y: 1, W: 5, H: 5}}})
	for _, tc := range []struct {
		x, y    int
		content string
		bg      bool
	}{{0, 0, "▀", true}, {1, 0, " ", true}, {2, 0, " ", false}, {3, 0, "▄", false}, {2, 1, "▀", false}, {3, 1, " ", false}} {
		c := screen.CellAt(tc.x, tc.y)
		if c.Content != tc.content || (c.Style.Bg != nil) != tc.bg {
			t.Errorf("cell (%d,%d) = %q bg %v, want %q bg %v", tc.x, tc.y, c.Content, c.Style.Bg, tc.content, tc.bg)
		}
	}
}

func TestKittyPlacementsSurviveAnInsertedPictureBeforeThem(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	other := image.NewRGBA(image.Rect(0, 0, 8, 8))
	other.Pix[3] = 0xff
	state := newKittyImageState(true)
	renderer := &kittyBufferRenderer{}
	keep := layout.TerminalImagePlacement{Rect: layout.Rect{X: 4, Y: 4, W: 2, H: 1}, Source: src}
	state.update([]layout.TerminalImagePlacement{keep}, renderer)
	renderer.Reset()
	state.update([]layout.TerminalImagePlacement{{Rect: layout.Rect{W: 2, H: 1}, Source: other}, keep}, renderer)
	if got := renderer.String(); strings.Contains(got, "a=d") || strings.Count(got, "a=p") != 1 {
		t.Fatalf("inserting a picture re-placed the others: %q", got)
	}
}

func TestCellImagesFrameRepaintsWhenToggled(t *testing.T) {
	if framesEqual(Frame{Content: "x"}, Frame{Content: "x", CellImages: true}) {
		t.Fatal("frames differing only in CellImages compare equal; the cleared cells would never be redrawn")
	}
}

func TestKittyPlacesAScreenOfThumbnails(t *testing.T) {
	state := newKittyImageState(true)
	renderer := &kittyBufferRenderer{}
	var placements []layout.TerminalImagePlacement
	for i := range 40 {
		src := image.NewRGBA(image.Rect(0, 0, 64, 64))
		src.Pix[0] = uint8(i)
		placements = append(placements, layout.TerminalImagePlacement{Rect: layout.Rect{X: 0, Y: i * 2, W: 4, H: 2}, Source: src})
	}
	state.update(placements, renderer)
	if got := strings.Count(renderer.String(), "a=p,"); got != 40 {
		t.Fatalf("placed %d of 40 thumbnails", got)
	}
}
