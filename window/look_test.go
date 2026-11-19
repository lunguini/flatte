package window

import (
	"image/color"
	"testing"

	ds "github.com/lunguini/flatte/theme"
)

// Controls follow the design: the medium radius per roundness seed, the
// palette's focus, washes and accent.
func TestLookFollowsDesign(t *testing.T) {
	for _, tc := range []struct {
		r            ds.Roundness
		radius, chck float64
	}{{ds.Subtle, 6, 4}, {ds.Square, 0, 0}, {ds.Round, 10, 4}} {
		d := ds.Default().With(ds.Seeds{Roundness: tc.r})
		l := fromDesign(&d, true).look()
		if l.radius != tc.radius || l.check != tc.chck {
			t.Errorf("roundness %v: radius %v check %v, want %v %v", tc.r, l.radius, l.check, tc.radius, tc.chck)
		}
	}
	d := ds.Default()
	for _, dark := range []bool{true, false} {
		p := d.Palette(dark)
		l := fromDesign(&d, dark).look()
		if l.focus != rgbaOf(p.Focus) || l.hover != rgbaOf(p.Hover) || l.pressed != rgbaOf(p.Pressed) ||
			l.accent != rgbaOf(p.Accent) || l.accentHot != rgbaOf(p.AccentHover) || l.onAccent != rgbaOf(p.OnAccent) ||
			l.field != rgbaOf(p.Field) || l.border != rgbaOf(p.ControlBorder) {
			t.Errorf("dark=%v: look %+v not the palette's", dark, l)
		}
		if l.focusWidth != 2 || l.control != 1 {
			t.Errorf("strokes %v %v", l.focusWidth, l.control)
		}
	}
}

// A Theme an app set field by field keeps its own accent, field, border
// and radius.
func TestLookKeepsAppColors(t *testing.T) {
	th := Dark()
	th.Accent, th.Field, th.Border, th.Radius = color.RGBA{0xff, 0x80, 0, 0xff}, color.RGBA{1, 2, 3, 0xff}, color.RGBA{9, 9, 9, 0xff}, 3
	l := th.look()
	if l.accent != rgbaOf(th.Accent) || l.focus != rgbaOf(th.Accent) || l.field != rgbaOf(th.Field) || l.border != rgbaOf(th.Border) {
		t.Errorf("app colors lost: %+v", l)
	}
	if l.onAccent != (rgba{0.04, 0.07, 0.13, 1}) {
		t.Errorf("text on a light orange accent %+v", l.onAccent)
	}
	if l.radius != 4 || l.check != 3 {
		t.Errorf("app radius: %v %v", l.radius, l.check)
	}
	th.Radius = -1
	if l := th.look(); l.radius != 0 || l.check != 0 {
		t.Errorf("square: %v %v", l.radius, l.check)
	}
}

func TestWash(t *testing.T) {
	l := Dark().look()
	if w := l.wash(0, 0); w.A != 0 {
		t.Errorf("idle wash %+v", w)
	}
	if w := l.wash(1, 0); w != l.hover {
		t.Errorf("hover %+v, want %+v", w, l.hover)
	}
	if w := l.wash(1, 1); w != l.pressed {
		t.Errorf("pressed %+v, want %+v", w, l.pressed)
	}
	bg := rgba{0.1, 0.1, 0.1, 1}
	if got := l.hover.over(bg); got.A != 1 || got.R <= bg.R {
		t.Errorf("white wash over dark %+v", got)
	}
}
