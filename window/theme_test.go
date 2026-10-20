package window

import (
	"image/color"
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
	if !reflect.DeepEqual(themeFor(nil, true), Dark()) || !reflect.DeepEqual(themeFor(nil, false), Light()) {
		t.Fatal("no theme should follow the system appearance")
	}
	// A partial theme keeps the base's other fields; a custom one is not
	// switched by the system.
	red := color.RGBA{0xff, 0, 0, 0xff}
	got := themeFor(&Theme{Accent: red, FontSize: 16, Radius: -1}, false)
	want := Dark()
	want.Accent, want.FontSize, want.Radius = red, 16, -1
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial theme = %+v, want Dark with the overrides", got)
	}
	light := Light()
	light.Accent = red
	if got := themeFor(&light, true); !reflect.DeepEqual(got, light) {
		t.Fatal("an explicit light theme followed the dark system appearance")
	}
}
