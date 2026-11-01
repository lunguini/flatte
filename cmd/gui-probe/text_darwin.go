package main

import "github.com/ebitengine/purego/objc"

// C functions and constants bound by loadFrameworks.
var (
	cgContextSetRGBFillColor         func(ctx uintptr, r, g, b, a float64)
	cgContextFillRect                func(ctx uintptr, rect CGRect)
	cgContextSetTextMatrix           func(ctx uintptr, t CGAffineTransform)
	cgContextSetTextPosition         func(ctx uintptr, x, y float64)
	ctLineCreateWithAttributedString func(attr objc.ID) uintptr
	ctLineDraw                       func(line, ctx uintptr)
	ctLineGetTypographicBounds       func(line uintptr, ascent, descent, leading *float64) float64
	ctLineGetOffsetForStringIndex    func(line uintptr, index int, secondary *float64) float64
	cfRelease                        func(ref uintptr)

	kCTFontAttributeName                       objc.ID
	kCTForegroundColorFromContextAttributeName objc.ID
	kCTUnderlineStyleAttributeName             objc.ID
)

type rgba struct{ R, G, B, A float64 }

// Probe palette and metrics, in points. Layout is fixed: the probe tests the
// platform path, not layout.
var (
	colorBackground = rgba{0.118, 0.122, 0.133, 1}
	colorText       = rgba{0.871, 0.878, 0.898, 1}
	colorDim        = rgba{0.553, 0.569, 0.608, 1}
	colorField      = rgba{0.169, 0.176, 0.196, 1}
	colorCaret      = rgba{0.400, 0.655, 1.000, 1}
)

const (
	padding       = 28.0
	fieldTop      = 236.0
	fieldHeight   = 44.0
	fieldInset    = 12.0
	fieldFontSize = 16.0
	sampleText    = "The quick brown fox jumps — 0123456789 · Ünïcödé · 日本語 · 😀"
	placeholder   = "Type here — try an input method such as Japanese Kana"
)

func setFill(ctx uintptr, c rgba) { cgContextSetRGBFillColor(ctx, c.R, c.G, c.B, c.A) }

func systemFont(size float64) objc.ID {
	return class("NSFont").Send(sel("systemFontOfSize:"), size)
}

func boldSystemFont(size float64) objc.ID {
	return class("NSFont").Send(sel("boldSystemFontOfSize:"), size)
}

// textLine is a CoreText line and its typographic metrics. Call release.
type textLine struct {
	ref                    uintptr
	width, ascent, descent float64
}

// newLine shapes text in font, underlining underline when present. Color
// comes from the context fill at draw time.
func newLine(text string, font objc.ID, underline span) textLine {
	attr := alloc("NSMutableAttributedString").Send(sel("initWithString:"), nsString(text))
	defer attr.Send(sel("release"))
	attrs := class("NSMutableDictionary").Send(sel("dictionary"))
	attrs.Send(sel("setObject:forKey:"), font, kCTFontAttributeName)
	attrs.Send(sel("setObject:forKey:"), class("NSNumber").Send(sel("numberWithBool:"), true), kCTForegroundColorFromContextAttributeName)
	attr.Send(sel("addAttributes:range:"), attrs, NSRange{Length: uint(utf16Len(text))})
	if !underline.Missing && underline.Len > 0 {
		single := class("NSNumber").Send(sel("numberWithInt:"), int32(1))
		attr.Send(sel("addAttribute:value:range:"), kCTUnderlineStyleAttributeName, single, fromSpan(underline))
	}
	l := textLine{ref: ctLineCreateWithAttributedString(attr)}
	var leading float64
	l.width = ctLineGetTypographicBounds(l.ref, &l.ascent, &l.descent, &leading)
	return l
}

func (l textLine) release() { cfRelease(l.ref) }

// draw paints l with its baseline origin at (x, y) in flipped view space.
func (l textLine) draw(ctx uintptr, x, y float64, c rgba) {
	setFill(ctx, c)
	cgContextSetTextPosition(ctx, x, y)
	ctLineDraw(l.ref, ctx)
}

func drawText(ctx uintptr, text string, font objc.ID, x, y float64, c rgba) {
	l := newLine(text, font, noSpan)
	defer l.release()
	l.draw(ctx, x, y, c)
}

// fieldBaseline is the input text's baseline origin. draw and caretRect
// must agree on it.
func fieldBaseline() (x, y float64) { return padding + fieldInset, fieldTop + 28 }

// caretRect locates the caret in flipped view coordinates.
func caretRect(b *editBuffer) CGRect {
	font := systemFont(fieldFontSize)
	l := newLine(b.Text(), font, noSpan)
	defer l.release()
	x, y := fieldBaseline()
	off := ctLineGetOffsetForStringIndex(l.ref, b.SelectedRange().Loc, nil)
	ascent := objc.Send[float64](font, sel("ascender"))
	descent := -objc.Send[float64](font, sel("descender"))
	return CGRect{Origin: CGPoint{X: x + off, Y: y - ascent}, Size: CGSize{Width: 1.5, Height: ascent + descent}}
}

// drawFrame paints the whole probe frame into ctx, a CoreGraphics context
// for a flipped (top-left origin) view of the given bounds.
func drawFrame(ctx uintptr, bounds CGRect, b *editBuffer) {
	setFill(ctx, colorBackground)
	cgContextFillRect(ctx, bounds)
	// Flipped view space would mirror glyphs; flip the text matrix back.
	cgContextSetTextMatrix(ctx, CGAffineTransform{A: 1, D: -1})

	drawText(ctx, "Flatte GUI probe", boldSystemFont(22), padding, padding+22, colorText)
	drawText(ctx, "AppKit + CoreText through purego · CGO_ENABLED=0", systemFont(13), padding, padding+46, colorDim)
	y := padding + 92.0
	for _, size := range []float64{12, 14, 17} {
		drawText(ctx, sampleText, systemFont(size), padding, y, colorText)
		y += size + 14
	}

	setFill(ctx, colorField)
	cgContextFillRect(ctx, CGRect{
		Origin: CGPoint{X: padding, Y: fieldTop},
		Size:   CGSize{Width: bounds.Size.Width - 2*padding, Height: fieldHeight},
	})
	x, baseline := fieldBaseline()
	if b.Text() == "" {
		drawText(ctx, placeholder, systemFont(fieldFontSize), x, baseline, colorDim)
	} else {
		l := newLine(b.Text(), systemFont(fieldFontSize), b.MarkedRange())
		l.draw(ctx, x, baseline, colorText)
		l.release()
	}
	setFill(ctx, colorCaret)
	cgContextFillRect(ctx, caretRect(b))

	drawText(ctx, "Esc quits when no composition is active · ⌘Q quits", systemFont(12), padding, fieldTop+fieldHeight+28, colorDim)
}
