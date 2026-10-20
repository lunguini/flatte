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
	w1, h1 := m.MeasureText("abc", layout.TextStyle{}, 0)
	w2, h2 := m.MeasureText("abcabc\nx", layout.TextStyle{}, 0)
	if !(w1 > 0) || math.Abs(w2-2*w1) > 1 || h1 != m.Line || h2 != 2*m.Line {
		t.Fatalf("measure: %v×%v, %v×%v (line %v)", w1, h1, w2, h2, m.Line)
	}
	_, ht := m.MeasureText("Title", layout.TextStyle{Role: layout.TextTitle}, 0)
	wb, _ := m.MeasureText("Bold", layout.TextStyle{Bold: true}, 0)
	wr, _ := m.MeasureText("Bold", layout.TextStyle{}, 0)
	if !(ht > h1) || !(wb > wr) {
		t.Fatalf("title height %v vs %v, bold width %v vs %v", ht, h1, wb, wr)
	}
	// Titles are semibold: heavier than body text, lighter than bold.
	wt, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle}, 0)
	wtb, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle, Bold: true}, 0)
	if !(wtb > wt) {
		t.Fatalf("bold title %v is not wider than the semibold title %v", wtb, wt)
	}
	// With a width, text wraps by words at it, in the font's advances.
	limit, _ := m.MeasureText("the quick brown", layout.TextStyle{}, 0)
	ww, wh := m.MeasureText("the quick brown fox", layout.TextStyle{}, limit)
	if wh != 2*m.Line || ww != limit {
		t.Fatalf("wrapped at %v: %v×%v, want 2 lines, the first exactly the limit", limit, ww, wh)
	}
	// Fonts are cached per style: asking for the bold title must not change
	// the plain one.
	if again, _ := m.MeasureText("Title weight", layout.TextStyle{Role: layout.TextTitle}, 0); again != wt {
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

// TestSnapshotIconsAndSelection: icons draw as shapes in their style's
// color, and a selected row is highlighted behind its content.
func TestSnapshotIconsAndSelection(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	green := lipgloss.Color("#00ff00")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Row{NodeBase: layout.NodeBase{ID: "sel", Box: layout.Box{Selected: true}}, Children: []layout.Node{
				// The text is an underscore so a shape cannot be mistaken for
				// the font's glyph: an underscore has nothing at the center.
				layout.Icon{NodeBase: layout.NodeBase{ID: "dot"}, Name: layout.IconDot, Text: "_ ", Style: layout.TextStyle{Fg: green}},
				layout.Label{Text: "running"},
			}},
			layout.Row{NodeBase: layout.NodeBase{ID: "plain"}, Children: []layout.Node{
				layout.Icon{NodeBase: layout.NodeBase{ID: "ring"}, Name: layout.IconCircle, Text: "_ ", Style: layout.TextStyle{Fg: green}},
				layout.Label{Text: "stopped"},
			}},
			layout.Row{Children: []layout.Node{
				layout.Icon{NodeBase: layout.NodeBase{ID: "none"}, Text: "_ ", Style: layout.TextStyle{Fg: green}},
			}},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 5}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 30*m.Column, 5*m.Line, m)
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	dot, ring := s.Rects["dot"], s.Rects["ring"]
	if c := at(dot.X+m.Column/2, dot.Y+dot.H/2); !near(c, color.RGBA{0, 255, 0, 255}, 40) {
		t.Errorf("dot center %v, want the icon's green", c)
	}
	// A ring is hollow: its center is the row's background, its edge green.
	if c := at(ring.X+m.Column/2, ring.Y+ring.H/2); !near(c, toColor(darkTheme.bg), 8) {
		t.Errorf("ring center %v, want the background", c)
	}
	if !region(img, layout.PointRect{X: ring.X, Y: ring.Y + ring.H/2 - 1, W: m.Column, H: 2}, 2, color.RGBA{0, 255, 0, 255}, 90) {
		t.Error("ring draws no green across its middle")
	}
	// IconNone draws its text: the underscore, nothing at the center.
	none := s.Rects["none"]
	if c := at(none.X+m.Column/2, none.Y+none.H/2); !near(c, toColor(darkTheme.bg), 8) {
		t.Errorf("IconNone center %v, want its text (an underscore) to leave it empty", c)
	}
	if !differs(img, none, 2) {
		t.Error("IconNone draws nothing; it should draw its text")
	}
	// The selected row is highlighted where it has no content; the other
	// row is the plain background there.
	sel, plain := s.Rects["sel"], s.Rects["plain"]
	if c := at(sel.X+sel.W-4, sel.Y+sel.H/2); near(c, toColor(darkTheme.bg), 8) {
		t.Errorf("selected row end %v is the plain background, want the highlight", c)
	}
	if c := at(plain.X+plain.W-4, plain.Y+plain.H/2); !near(c, toColor(darkTheme.bg), 2) {
		t.Errorf("unselected row end %v, want the background", c)
	}
}

// TestSnapshotMeter: a Meter is a native bar — a thin rounded track with the
// filled part in the fill color (the accent by default), not cell blocks.
func TestSnapshotMeter(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	green := lipgloss.Color("#00ff00")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Meter{NodeBase: layout.NodeBase{ID: "custom"}, Value: 0.25, Fill: green},
			layout.Meter{NodeBase: layout.NodeBase{ID: "default"}, Value: 0.5},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 3}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 40*m.Column, 3*m.Line, m)
	at := func(x, y float64) color.Color { return img.At(int(x*2), int(y*2)) }
	c, d := s.Rects["custom"], s.Rects["default"]
	mid := c.Y + c.H/2
	if got := at(c.X+c.W*0.1, mid); !near(got, color.RGBA{0, 255, 0, 255}, 30) {
		t.Errorf("filled part %v, want the fill color", got)
	}
	if got := at(c.X+c.W*0.6, mid); near(got, color.RGBA{0, 255, 0, 255}, 60) || near(got, toColor(darkTheme.bg), 4) {
		t.Errorf("unfilled part %v, want the track", got)
	}
	// Thin: the row's top edge is background, not bar.
	if got := at(c.X+c.W*0.1, c.Y+1); !near(got, toColor(darkTheme.bg), 4) {
		t.Errorf("bar fills its whole line at the top edge: %v", got)
	}
	if got := at(d.X+d.W*0.25, d.Y+d.H/2); !near(got, toColor(darkTheme.caret), 30) {
		t.Errorf("default fill %v, want the accent", got)
	}
}

// TestSnapshotWrappedText: a wrapping Label is laid out and painted on as
// many lines as the font needs at its width; unwrapped, the same text stays
// on one line.
func TestSnapshotWrappedText(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	text := "the quick brown fox jumps over the lazy dog"
	for _, wrap := range []bool{true, false} {
		root := func() layout.Node {
			return layout.Col{Children: []layout.Node{
				layout.Row{Children: []layout.Node{
					layout.Label{NodeBase: layout.NodeBase{ID: "p", W: layout.Fixed(14)}, Text: text, Wrap: wrap},
					layout.NewSpacer(),
				}},
				layout.Label{NodeBase: layout.NodeBase{ID: "after"}, Text: "after"},
			}}
		}
		img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 8}, 2)
		if err != nil {
			t.Fatal(err)
		}
		s := layout.LayoutScene(root(), 40*m.Column, 8*m.Line, m)
		p := s.Rects["p"]
		second := layout.PointRect{X: p.X, Y: p.Y + m.Line, W: p.W, H: m.Line}
		if wrap {
			if p.H < 3*m.Line || !differs(img, second, 2) {
				t.Errorf("wrapped: label is %v tall (line %v); second line drawn: %v", p.H, m.Line, differs(img, second, 2))
			}
			if s.Rects["after"].Y != p.Y+p.H {
				t.Errorf("next label at %v, want right after the wrapped text (%v)", s.Rects["after"].Y, p.Y+p.H)
			}
		} else if p.H != m.Line {
			t.Errorf("unwrapped label is %v tall, want one line", p.H)
		}
	}
}

// TestSnapshotAdjacentBoxesHaveNoSeam: filled boxes side by side at
// fractional positions (a tab's label and its padding) meet without a
// lighter seam.
func TestSnapshotAdjacentBoxesHaveNoSeam(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	fill := lipgloss.Color("#ffffff")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{layout.Row{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{PadLeft: 2, Box: layout.Box{Background: fill}}, Text: "tab"},
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(2), Box: layout.Box{Background: fill}}},
			layout.NewSpacer(),
		}}}}
	}
	m := metrics()
	for _, scale := range []float64{1, 2} {
		img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 2}, scale)
		if err != nil {
			t.Fatal(err)
		}
		s := layout.LayoutScene(root(), 20*m.Column, 2*m.Line, m)
		var end float64
		for _, op := range s.Ops {
			if b, ok := op.(layout.BoxOp); ok {
				end = max(end, b.Rect.X+b.Rect.W)
			}
		}
		y := int(1 * scale) // the top row of the fill, above the text
		for x := 0; x < int(end*scale)-1; x++ {
			if c := img.At(x, y); !near(c, color.RGBA{255, 255, 255, 255}, 3) {
				t.Fatalf("scale %v: seam at pixel %d (%v)", scale, x, c)
			}
		}
	}
}

// TestSnapshotRoundBox: a Round box is a pill: filled in the middle, the
// window background at its corners.
func TestSnapshotRoundBox(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	fill := lipgloss.Color("#ffffff")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{layout.Row{Children: []layout.Node{
			layout.Row{NodeBase: layout.NodeBase{ID: "pill", Box: layout.Box{Background: fill, Round: true}}, Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{PadLeft: 2, PadRight: 2}, Text: "tab"},
			}},
			layout.NewSpacer(),
		}}}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 2}, 2)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.LayoutScene(root(), 20*m.Column, 2*m.Line, m).Rects["pill"]
	if c := img.At(int((r.X+1)*2), int((r.Y+1)*2)); !near(c, toColor(darkTheme.bg), 8) {
		t.Errorf("pill corner %v, want the background", c)
	}
	if c := img.At(int((r.X+r.W/2)*2), int((r.Y+2)*2)); !near(c, color.RGBA{255, 255, 255, 255}, 8) {
		t.Errorf("pill top middle %v, want the fill", c)
	}
}

// TestSnapshotTitledBox: the title sits on the top stroke, which breaks
// around it and runs on past it.
func TestSnapshotTitledBox(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := lipgloss.Color("#ff0000")
	green := lipgloss.Color("#00ff00")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: "box", H: layout.Fixed(4), Box: layout.Box{
				Border: layout.BorderRounded, BorderColor: red, Title: "WWWW", TitleStyle: layout.TextStyle{Fg: green},
			}}},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.LayoutScene(root(), 30*m.Column, 4*m.Line, m).Rects["box"]
	in := strokeRect(r, borderBand)
	rad := darkTheme.radiusOf(layout.BorderRounded)
	gap0 := in.Origin.X + max(rad, titleGap) + titleGap
	stroke := func(x0, x1 float64) layout.PointRect {
		return layout.PointRect{X: x0, Y: in.Origin.Y - 1, W: x1 - x0, H: 2}
	}
	if !region(img, stroke(gap0+titleGap, gap0+titleGap+20), 2, color.RGBA{0, 255, 0, 255}, 60) {
		t.Error("title not drawn on the top stroke")
	}
	if region(img, stroke(gap0+1, gap0+titleGap+20), 2, color.RGBA{255, 0, 0, 255}, 40) {
		t.Error("stroke not broken under the title")
	}
	if !region(img, stroke(r.X+r.W/2+20, r.X+r.W-30), 2, color.RGBA{255, 0, 0, 255}, 40) {
		t.Error("stroke does not run on after the title")
	}
}

// TestSnapshotScrollBar: a thumb as far down its track as the window is
// down its content, and nothing when the content fits.
func TestSnapshotScrollBar(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	green := lipgloss.Color("#00ff00")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(10)}, Children: []layout.Node{
			layout.ScrollBar{NodeBase: layout.NodeBase{ID: "top"}, Offset: 0, Visible: 10, Total: 40, Color: green},
			layout.ScrollBar{NodeBase: layout.NodeBase{ID: "end"}, Offset: 30, Visible: 10, Total: 40, Color: green},
			layout.ScrollBar{NodeBase: layout.NodeBase{ID: "fits"}, Visible: 10, Total: 10, Color: green},
		}}}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 10, Height: 10}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 10*m.Column, 10*m.Line, m)
	g := color.RGBA{0, 255, 0, 255}
	band := func(id string, from, to float64) layout.PointRect {
		r := s.Rects[id]
		return layout.PointRect{X: r.X + r.W/2 - 1, Y: r.Y + from*r.H, W: 2, H: (to - from) * r.H}
	}
	if !region(img, band("top", 0.02, 0.2), 2, g, 30) || region(img, band("top", 0.3, 1), 2, g, 30) {
		t.Error("a scrolled-to-top thumb is not the top quarter")
	}
	if !region(img, band("end", 0.8, 0.98), 2, g, 30) || region(img, band("end", 0, 0.7), 2, g, 30) {
		t.Error("a scrolled-to-end thumb is not the bottom quarter")
	}
	if region(img, band("fits", 0, 1), 2, g, 60) {
		t.Error("content that fits shows a thumb")
	}
}
