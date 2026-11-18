package window

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
	"image/color"
	"math"
	"reflect"
	"testing"
)

func TestBaseThemesAreComplete(t *testing.T) {
	for name, th := range map[string]Theme{"dark": Dark(), "light": Light()} {
		v := reflect.ValueOf(th)
		for i := range v.NumField() {
			f := v.Field(i)
			if f.Kind() == reflect.Interface && f.IsNil() {
				t.Errorf("%s: %s is unset", name, v.Type().Field(i).Name)
			}
		}
		if th.FontSize <= 0 || th.TitleSize <= th.FontSize || th.LineSpacing < 1 || th.Radius <= 0 {
			t.Errorf("%s: sizes %+v", name, th)
		}
	}
	// Light is light: its background is brighter than its text.
	lum := func(c color.Color) uint32 { r, g, b, _ := c.RGBA(); return r + g + b }
	if l := Light(); lum(l.Background) <= lum(l.Text) {
		t.Error("light theme background is darker than its text")
	}
	if d := Dark(); lum(d.Background) >= lum(d.Text) {
		t.Error("dark theme background is lighter than its text")
	}
}

func TestThemeFor(t *testing.T) {
	if !reflect.DeepEqual(themeFor(nil, true, nil), Dark()) || !reflect.DeepEqual(themeFor(nil, false, nil), Light()) {
		t.Fatal("no theme should follow the system appearance")
	}
	// A partial theme keeps the base's other fields; a custom one is not
	// switched by the system.
	red := color.RGBA{0xff, 0, 0, 0xff}
	got := themeFor(&Theme{Accent: red, FontSize: 16, Radius: -1}, false, nil)
	want := Dark()
	want.Accent, want.FontSize, want.Radius = red, 16, -1
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial theme = %+v, want Dark with the overrides", got)
	}
	light := Light()
	light.Accent = red
	if got := themeFor(&light, true, nil); !reflect.DeepEqual(got, light) {
		t.Fatal("an explicit light theme followed the dark system appearance")
	}
}

func TestControlHeightThemeDefaultAndOverride(t *testing.T) {
	if want := ds.Default().Size.Control; Dark().ControlHeight != want || Light().ControlHeight != want {
		t.Fatal("themes do not share regular control height")
	}
	if got := themeFor(&Theme{ControlHeight: 40}, true, nil); got.ControlHeight != 40 {
		t.Fatalf("control height override %+v", got)
	}
	if got := themeFor(&Theme{ControlHeight: -1}, true, nil); got.ControlHeight != -1 {
		t.Fatal("minimum optout lost")
	}
}

// Cell grids draw the 16 ANSI colors in the theme's palette, tuned to read
// on its background; other colors pass through.
func TestThemeMapsANSIColors(t *testing.T) {
	for name, th := range map[string]Theme{"dark": Dark(), "light": Light()} {
		for i := range 16 {
			if th.ANSI[i] == nil {
				t.Fatalf("%s: ANSI[%d] unset", name, i)
			}
			if got := th.cellColor(ansi.BasicColor(i)); got != th.ANSI[i] {
				t.Fatalf("%s: basic %d → %v, want the palette's", name, i, got)
			}
			if got := th.cellColor(ansi.IndexedColor(i)); got != th.ANSI[i] {
				t.Fatalf("%s: indexed %d → %v, want the palette's", name, i, got)
			}
		}
		// Normal text colors must read on the background (WCAG 3:1 for
		// large-ish mono text); black on dark and white on light are the
		// exceptions every terminal palette has.
		for i := 1; i <= 6; i++ {
			if r := contrast(th.ANSI[i], th.Field); r < 3 {
				t.Errorf("%s: ANSI %d contrast %.2f on the field", name, i, r)
			}
		}
		for _, c := range []color.Color{ansi.IndexedColor(16), ansi.IndexedColor(208), color.RGBA{1, 2, 3, 255}} {
			if got := th.cellColor(c); got != c {
				t.Fatalf("%s: %v changed to %v", name, c, got)
			}
		}
	}
	// A partial palette keeps the base's other entries.
	custom := Theme{ANSI: [16]color.Color{4: color.RGBA{9, 9, 9, 255}}}.over(Dark())
	if custom.ANSI[4] != (color.RGBA{9, 9, 9, 255}) || custom.ANSI[1] != Dark().ANSI[1] {
		t.Fatal("partial palette not merged over the base")
	}
}

func contrast(a, b color.Color) float64 {
	lum := func(c color.Color) float64 {
		r, g, bl, _ := c.RGBA()
		ch := func(v uint32) float64 {
			x := float64(v) / 0xffff
			if x <= 0.03928 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(r) + 0.7152*ch(g) + 0.0722*ch(bl)
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Density sets the layout step and default insets: regular is today's
// 8pt step with 4×8 control insets and 12pt panels.
func TestThemeSpacingFollowsDensity(t *testing.T) {
	step, in := Dark().spacing()
	if step != 8 || in != (layout.Insets{Vertical: 4, Horizontal: 8, Panel: 12}) {
		t.Fatalf("regular spacing %v %+v", step, in)
	}
	compact := ds.Default().With(ds.Seeds{Density: ds.Compact})
	step, in = themeFor(nil, true, &compact).spacing()
	if step != compact.Space[1] || in != (layout.Insets{Vertical: compact.Space[0], Horizontal: compact.Space[1], Panel: compact.Space[2]}) || step >= 8 {
		t.Fatalf("compact spacing %v %+v", step, in)
	}
	// A partial custom theme keeps the base's design spacing.
	if step, _ := themeFor(&Theme{FontSize: 15}, true, &compact).spacing(); step != compact.Space[1] {
		t.Fatalf("custom theme lost the design step: %v", step)
	}
}
