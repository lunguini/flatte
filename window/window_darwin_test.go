package window

import (
	"image"
	"image/color"
	"math"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestViewImplementsTextInputClientAndEvents(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	cls, err := registerViewClass()
	if err != nil {
		t.Fatal(err)
	}
	view := objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), cgRect(0, 0, 10, 10))
	defer view.Send(sel("release"))
	if !objc.Send[bool](view, sel("conformsToProtocol:"), objc.GetProtocol("NSTextInputClient")) {
		t.Fatal("view does not conform to NSTextInputClient")
	}
	for _, name := range []string{
		"hasMarkedText", "markedRange", "selectedRange", "setMarkedText:selectedRange:replacementRange:",
		"unmarkText", "validAttributesForMarkedText", "attributedSubstringForProposedRange:actualRange:",
		"insertText:replacementRange:", "characterIndexForPoint:", "firstRectForCharacterRange:actualRange:",
		"doCommandBySelector:", "keyDown:", "mouseDown:", "mouseUp:", "mouseDragged:", "scrollWheel:",
		"rightMouseDown:", "otherMouseDown:", "drawRect:", "isFlipped", "acceptsFirstResponder",
	} {
		if !objc.Send[bool](view, sel("respondsToSelector:"), sel(name)) {
			t.Errorf("view does not respond to %s", name)
		}
	}
	// With no window running, the input methods answer safely.
	if objc.Send[bool](view, sel("hasMarkedText")) {
		t.Error("hasMarkedText with no window")
	}
	if r := objc.Send[NSRange](view, sel("markedRange")); r.Location != nsNotFound {
		t.Errorf("markedRange = %+v", r)
	}
}

func TestMetricsFromFonts(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	if !(m.Column > 4 && m.Column < 12) || !(m.Line > 12 && m.Line < 24) || m.Step != 8 || m.Border != 1 {
		t.Fatalf("metrics = %+v", m)
	}
	w1, h1 := m.MeasureText("abc", layout.TextStyle{})
	w2, h2 := m.MeasureText("abcabc\nx", layout.TextStyle{})
	if !(w1 > 0) || math.Abs(w2-2*w1) > 1 || h1 != m.Line || h2 != 2*m.Line {
		t.Fatalf("measure: %v×%v, %v×%v (line %v)", w1, h1, w2, h2, m.Line)
	}
	_, ht := m.MeasureText("Title", layout.TextStyle{Role: layout.TextTitle})
	wb, _ := m.MeasureText("Bold", layout.TextStyle{Bold: true})
	wr, _ := m.MeasureText("Bold", layout.TextStyle{})
	if !(ht > h1) || !(wb > wr) {
		t.Fatalf("title height %v vs %v, bold width %v vs %v", ht, h1, wb, wr)
	}
}

type snapState struct{}

func snapApp(root func() layout.Node) flatte.App[snapState] {
	return flatte.App[snapState]{
		State: &snapState{},
		View: func(*snapState, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: root(), Title: "snap"}
		},
	}
}

func near(a, b color.Color, tol uint32) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) uint32 {
		if x > y {
			return x - y
		}
		return y - x
	}
	return d(ar, br)>>8 <= tol && d(ag, bg)>>8 <= tol && d(ab, bb)>>8 <= tol
}

func toColor(c rgba) color.Color {
	return color.RGBA{uint8(c.R*255 + 0.5), uint8(c.G*255 + 0.5), uint8(c.B*255 + 0.5), 255}
}

// region reports whether any pixel in r (in points) matches want.
func region(img image.Image, r layout.PointRect, scale float64, want color.Color, tol uint32) bool {
	for y := int(r.Y * scale); y < int((r.Y+r.H)*scale); y++ {
		for x := int(r.X * scale); x < int((r.X+r.W)*scale); x++ {
			if near(img.At(x, y), want, tol) {
				return true
			}
		}
	}
	return false
}

// differs reports whether some pixel in r is not the background.
func differs(img image.Image, r layout.PointRect, scale float64) bool {
	bg := toColor(darkTheme.bg)
	for y := int(r.Y * scale); y < int((r.Y+r.H)*scale); y++ {
		for x := int(r.X * scale); x < int((r.X+r.W)*scale); x++ {
			if !near(img.At(x, y), bg, 8) {
				return true
			}
		}
	}
	return false
}

func TestSnapshotDrawsScene(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := lipgloss.Color("#ff0000")
	green := lipgloss.Color("#00ff00")
	root := func() layout.Node {
		return layout.Col{
			NodeBase: layout.NodeBase{ID: "card", Pad: 1, Box: layout.Box{Border: layout.BorderNormal, BorderColor: red}},
			Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "title"}, Text: "Snapshot", Style: layout.TextStyle{Role: layout.TextTitle}},
				layout.Input{NodeBase: layout.NodeBase{ID: "in"}, Value: "Ada", Caret: 1, Focused: true},
				layout.Text{NodeBase: layout.NodeBase{ID: "cells", H: layout.Fixed(1)}, String: lipgloss.NewStyle().Background(green).Render("  ") + " ok"},
				layout.NewSpacer(),
			},
		}
	}
	m := metrics()
	for _, scale := range []float64{1, 2} {
		img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 10}, scale)
		if err != nil {
			t.Fatal(err)
		}
		wantW, wantH := int(math.Ceil(40*m.Column*scale)), int(math.Ceil(10*m.Line*scale))
		if b := img.Bounds(); b.Dx() != wantW || b.Dy() != wantH {
			t.Fatalf("scale %v: size %v, want %dx%d", scale, b, wantW, wantH)
		}
		scene := layout.LayoutScene(root(), 40*m.Column, 10*m.Line, m)
		card, title, in, cells := scene.Rects["card"], scene.Rects["title"], scene.Rects["in"], scene.Rects["cells"]
		// The border is drawn in its color along the card's left edge.
		if !region(img, layout.PointRect{X: card.X, Y: card.Y + 10, W: 2, H: 10}, scale, color.RGBA{255, 0, 0, 255}, 40) {
			t.Errorf("scale %v: no red border on the card's left edge", scale)
		}
		// Glyphs: the title draws something other than background.
		if !differs(img, title, scale) {
			t.Errorf("scale %v: title draws nothing", scale)
		}
		// The focused input shows a caret in the caret color.
		if !region(img, in, scale, toColor(darkTheme.caret), 30) {
			t.Errorf("scale %v: no caret in the focused input", scale)
		}
		// The cell grid paints the two green background cells at its start.
		if !region(img, layout.PointRect{X: cells.X, Y: cells.Y + 2, W: m.Column, H: 4}, scale, color.RGBA{0, 255, 0, 255}, 40) {
			t.Errorf("scale %v: cell background not painted", scale)
		}
		// The empty area inside the card (it has no background): window
		// background, fully opaque, right to the last pixel.
		inside := img.At(int((card.X+card.W/2)*scale), int((card.Y+card.H-6)*scale))
		if !near(inside, toColor(darkTheme.bg), 2) {
			t.Errorf("scale %v: empty card area %v is not the background", scale, inside)
		}
		if _, _, _, a := img.At(img.Bounds().Dx()-1, img.Bounds().Dy()-1).RGBA(); a != 0xffff {
			t.Errorf("scale %v: last pixel is not opaque (alpha %d)", scale, a>>8)
		}
	}
}

func TestSnapshotStringFrame(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	app := flatte.App[snapState]{
		State: &snapState{},
		View: func(*snapState, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "hello\nworld", Cursor: &flatte.Cursor{X: 2, Y: 1}}
		},
	}
	m := metrics()
	img, err := Snapshot(app, Options{Width: 20, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !differs(img, layout.PointRect{W: 5 * m.Column, H: m.Line}, 2) {
		t.Error("string frame text not drawn")
	}
	if !region(img, layout.PointRect{X: 2 * m.Column, Y: m.Line, W: 2, H: m.Line}, 2, toColor(darkTheme.caret), 30) {
		t.Error("string frame cursor not drawn at its cell")
	}
}

func TestSnapshotRejectsBadScale(t *testing.T) {
	for _, s := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := Snapshot(snapApp(func() layout.Node { return layout.Label{Text: "x"} }), Options{}, s); err == nil {
			t.Errorf("scale %v accepted", s)
		}
	}
}

// TestSnapshotConcurrent: Snapshot may be called from any goroutine, so
// many at once must neither crash nor interfere (autorelease pools and the
// font cache are per-thread and shared state respectively).
func TestSnapshotConcurrent(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	app := func() flatte.App[snapState] {
		return snapApp(func() layout.Node {
			return layout.Col{Children: []layout.Node{
				layout.Label{Text: "concurrent", Style: layout.TextStyle{Role: layout.TextTitle, Bold: true}},
				layout.Input{Value: "x", Focused: true},
			}}
		})
	}
	errs := make(chan error, 64)
	for range 8 {
		go func() {
			for range 8 {
				_, err := Snapshot(app(), Options{Width: 30, Height: 6}, 1)
				errs <- err
			}
		}()
	}
	for range 64 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}
