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
	if !(m.Column > 4 && m.Column < 12) || !(m.Line > 12 && m.Line < 28) || m.Step != 8 || m.Border != borderBand {
		t.Fatalf("metrics = %+v", m)
	}
	// A line is the body font's height with room around it, on whole points.
	if body := fontFor(fontKey{}).lineHeight(); !(m.Line >= body*1.2) || m.Line != math.Round(m.Line) {
		t.Fatalf("line %v for a %vpt-high body font", m.Line, body)
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
	// Titles are semibold: heavier than body text, lighter than bold.
	wt, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle})
	wtb, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle, Bold: true})
	if !(wtb > wt) {
		t.Fatalf("bold title %v is not wider than the semibold title %v", wtb, wt)
	}
	// Fonts are cached per style: asking for the bold title must not change
	// the plain one.
	if again, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle}); again != wt {
		t.Fatalf("title width %v after a bold title, was %v", again, wt)
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
		// The border is a stroke in the middle of its band, as a
		// box-drawing character sits mid-cell: nothing at the card's outer
		// edge, red half a band in.
		if !region(img, layout.PointRect{X: card.X + borderBand/2 - 1, Y: card.Y + 20, W: 2, H: 10}, scale, color.RGBA{255, 0, 0, 255}, 40) {
			t.Errorf("scale %v: no red border in the middle of the card's left band", scale)
		}
		if region(img, layout.PointRect{X: card.X, Y: card.Y + 20, W: 2, H: 10}, scale, color.RGBA{255, 0, 0, 255}, 80) {
			t.Errorf("scale %v: border drawn at the card's outer edge", scale)
		}
		// The focused input is outlined in the focus color.
		if !region(img, layout.PointRect{X: in.X + in.W/2, Y: in.Y, W: 1, H: 2}, scale, toColor(darkTheme.caret), 40) {
			t.Errorf("scale %v: focused input has no focus outline", scale)
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

// column returns the colors of the pixel column at x (points) from y0 to y1.
func column(img image.Image, x, y0, y1, scale float64) []color.Color {
	var cs []color.Color
	for y := int(y0 * scale); y < int(y1*scale); y++ {
		cs = append(cs, img.At(int(x*scale), y))
	}
	return cs
}

// TestSnapshotGridGlyphsJoinAcrossRows: rows are taller than the monospace
// font, so line and block characters must be shapes that fill their cells;
// drawn from the font, a vertical line breaks between every row.
func TestSnapshotGridGlyphsJoinAcrossRows(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	white := color.RGBA{255, 255, 255, 255}
	app := flatte.App[snapState]{
		State: &snapState{},
		View: func(*snapState, flatte.RenderContext) flatte.Frame {
			st := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
			return flatte.Frame{Content: st.Render("│ █\n│ █\n│ █")}
		},
	}
	for _, scale := range []float64{1, 2} {
		img, err := Snapshot(app, Options{Width: 10, Height: 4}, scale)
		if err != nil {
			t.Fatal(err)
		}
		line, _ := cellGlyph('│', 0, 0, m.Column, m.Line)
		lx := line.rects[0].X + 0.5
		for i, c := range column(img, lx, 0, 3*m.Line, scale) {
			if !near(c, white, 40) {
				t.Fatalf("scale %v: │ has a gap at pixel row %d (%v)", scale, i, c)
			}
		}
		for i, c := range column(img, 2*m.Column+m.Column/2, 0, 3*m.Line, scale) {
			if !near(c, white, 40) {
				t.Fatalf("scale %v: █ has a gap at pixel row %d (%v)", scale, i, c)
			}
		}
	}
}

// TestSnapshotOverlayIsRaised: an overlay's box is a raised surface, so the
// modal reads as above the page.
func TestSnapshotOverlayIsRaised(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{Text: "page"},
			layout.Col{
				NodeBase: layout.NodeBase{ID: "modal", Overlay: true, W: layout.Fixed(20), H: layout.Fixed(5), Box: layout.Box{Border: layout.BorderNormal}},
				Children: []layout.Node{layout.Label{Text: "modal"}},
			},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 12}, 2)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.LayoutScene(root(), 40*m.Column, 12*m.Line, m).Rects["modal"]
	if c := img.At(int((r.X+r.W-borderBand-2)*2), int((r.Y+r.H-borderBand-2)*2)); !near(c, toColor(darkTheme.surface), 2) {
		t.Errorf("inside the modal: %v, want the surface color", c)
	}
	if c := img.At(int((r.X+1)*2), int((r.Y+1)*2)); !near(c, toColor(darkTheme.bg), 2) {
		t.Errorf("outside the modal's stroke: %v, want the window background", c)
	}
}

// TestSnapshotGridHasNoSeams: columns are fractional, so adjacent cell
// fills must meet on whole pixels; antialiased edges would leave a lighter
// seam between every pair of cells.
func TestSnapshotGridHasNoSeams(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	white := color.RGBA{255, 255, 255, 255}
	bg := lipgloss.Color("#ffffff")
	app := flatte.App[snapState]{
		State: &snapState{},
		View: func(*snapState, flatte.RenderContext) flatte.Frame {
			blocks := lipgloss.NewStyle().Foreground(bg).Render("██████████")
			// Two runs (different foregrounds) on the same background.
			a := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#ff0000")).Render("     ")
			b := lipgloss.NewStyle().Background(bg).Foreground(lipgloss.Color("#00ff00")).Render("     ")
			shade := lipgloss.NewStyle().Foreground(bg).Render("░░░░░░░░░░")
			return flatte.Frame{Content: blocks + "\n" + a + b + "\n" + shade}
		},
	}
	for _, scale := range []float64{1, 2} {
		img, err := Snapshot(app, Options{Width: 12, Height: 4}, scale)
		if err != nil {
			t.Fatal(err)
		}
		for row := range 3 {
			y := int((float64(row) + 0.5) * m.Line * scale)
			want := color.Color(white)
			if row == 2 { // a translucent shade: uniform, whatever its color
				want = img.At(0, y)
			}
			for x := 0; x < int(10*m.Column*scale)-1; x++ {
				if c := img.At(x, y); !near(c, want, 3) {
					t.Fatalf("scale %v row %d: seam at pixel %d (%v, want %v)", scale, row, x, c, want)
				}
			}
		}
	}
}
