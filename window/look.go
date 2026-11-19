package window

import (
	"image/color"
	"math"

	ds "github.com/lunguini/flatte/theme"
)

// look is the design system's control recipe in a theme, shared by the
// AppKit and canvas painters: buttons, fields and toggles take the medium
// radius, hover and press are the palette's translucent washes, and focus
// is a ring in the Focus color at the focus stroke width.
//
// Colors an app set on its window.Theme (Accent, Field, Border) win over
// the palette's, so a Theme built field by field keeps drawing in its own
// colors.
type look struct {
	// radius is a button's or field's corner radius; check a checkbox's
	// (both 0 for square).
	radius, check float64
	// focus is the focus ring, focusWidth its stroke, and control the
	// outline width of fields and secondary buttons.
	focus             rgba
	focusWidth        float64
	control           float64
	hover, pressed    rgba // washes, over whatever is below
	accent, accentHot rgba // a primary button and a checked toggle
	onAccent          rgba // text and marks on accent
	field, border     rgba // a field's or toggle's fill and outline
	knob              rgba // an unchecked switch's thumb
	disabled          rgba // a disabled control's text and marks
}

// look is t's control recipe.
func (t Theme) look() look {
	t = t.over(Dark())
	d := t.design
	if d == nil {
		d = &defaultDesign
	}
	p := t.palette()
	same := func(c color.Color, n color.NRGBA) bool { return rgbaOf(c) == rgbaOf(n) }
	l := look{
		focus: rgbaOf(p.Focus), focusWidth: positive(d.Stroke.Focus, 2), control: positive(d.Stroke.Control, 1),
		hover: rgbaOf(p.Hover), pressed: rgbaOf(p.Pressed),
		accent: rgbaOf(p.Accent), accentHot: rgbaOf(p.AccentHover), onAccent: rgbaOf(p.OnAccent),
		field: rgbaOf(p.Field), border: rgbaOf(p.ControlBorder),
		knob: rgbaOf(p.TextSecondary), disabled: rgbaOf(p.TextDisabled),
	}
	if !same(t.Accent, p.Accent) { // the app's own accent
		a := rgbaOf(t.Accent)
		l.accent, l.focus, l.onAccent = a, a, onColor(a)
		l.accentHot = mix(a, l.onAccent, 0.12)
	}
	if !same(t.Field, p.Field) {
		l.field = rgbaOf(t.Field)
	}
	if !same(t.Border, p.ControlBorder) {
		l.border = rgbaOf(t.Border)
	}
	switch {
	case t.Radius < 0:
		// square
	case t.design != nil && t.Radius == smallRadius(t.design):
		l.radius, l.check = max(d.Radius.Md, 0), min(max(d.Radius.Sm, 0), checkRadius)
	default: // the app's own Radius: controls a point rounder, as before
		l.radius, l.check = t.Radius+1, min(t.Radius, checkRadius)
	}
	return l
}

// checkRadius is the roundest a 14-point checkbox gets before it reads as
// a radio button.
const checkRadius = 4.0

// smallRadius is the Theme.Radius fromDesign gives design: its small
// radius, or -1 (square) for none.
func smallRadius(design *ds.Theme) float64 {
	if design.Radius.Sm <= 0 {
		return -1
	}
	return design.Radius.Sm
}

func positive(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}

// onColor is the text color that reads on c: white on a dark fill, near
// black on a light one.
func onColor(c rgba) rgba {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	// Above 0.179 relative luminance black text contrasts more than white.
	if 0.2126*lin(c.R)+0.7152*lin(c.G)+0.0722*lin(c.B) > 0.179 {
		return rgba{0.04, 0.07, 0.13, 1}
	}
	return rgba{1, 1, 1, 1}
}

// over is the wash w composited over c (both straight alpha, c opaque or
// not): what a hover or press wash looks like on a fill.
func (w rgba) over(c rgba) rgba {
	a := w.A + c.A*(1-w.A)
	if a == 0 {
		return rgba{}
	}
	ch := func(x, y float64) float64 { return (x*w.A + y*c.A*(1-w.A)) / a }
	return rgba{ch(w.R, c.R), ch(w.G, c.G), ch(w.B, c.B), a}
}

// wash is the hover wash at level hot blended toward the pressed wash at
// level down (0..1 each): a control's or row's pointer feedback.
func (l look) wash(hot, down float64) rgba {
	w := mix(l.hover, l.pressed, min(down, 1))
	w.A *= min(max(hot, down), 1)
	return w
}

// b2f is 1 for true.
func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
