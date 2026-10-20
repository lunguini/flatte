package window

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestRgbaOfIsStraightAlpha(t *testing.T) {
	// Core Graphics takes straight alpha; color.Color's RGBA is premultiplied.
	got := rgbaOf(color.NRGBA{0xff, 0x80, 0x00, 0x40})
	if got.R != 1 || got.G < 0.5 || got.G > 0.503 || got.B != 0 || got.A < 0.25 || got.A > 0.252 {
		t.Fatalf("rgbaOf = %+v, want straight {1 0.5 0 0.25}", got)
	}
	if got := rgbaOf(color.NRGBA{}); got != (rgba{}) {
		t.Fatalf("transparent = %+v", got)
	}
}

func TestDarkThemeIsTheDefault(t *testing.T) {
	if got := newTheme(Theme{}); got != darkTheme {
		t.Fatalf("zero Theme = %+v, want Dark", got)
	}
}

func TestSnapshotTheme(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Label{Text: "page"},
			layout.Meter{NodeBase: layout.NodeBase{ID: "meter"}, Value: 1},
		}}
	}
	m := metrics()
	meterMid := func(th *Theme) color.Color {
		img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 4, Theme: th}, 2)
		if err != nil {
			t.Fatal(err)
		}
		r := layout.LayoutScene(root(), 20*m.Column, 4*m.Line, m).Rects["meter"]
		if c := img.At(int((r.X+r.W/2)*2), int((r.Y+r.H-2)*2)); th != nil && th.Background != nil && !near(c, th.Background, 2) {
			t.Errorf("background %v, want %v", c, th.Background)
		}
		return img.At(int((r.X+r.W/2)*2), int((r.Y+r.H/2)*2))
	}
	light := Light()
	meterMid(&light)
	if c := meterMid(nil); !near(c, Dark().Accent, 30) {
		t.Errorf("no theme: meter %v, want Dark's accent (Snapshot draws Dark)", c)
	}
	orange := color.RGBA{0xff, 0x80, 0x00, 0xff}
	if c := meterMid(&Theme{Accent: orange}); !near(c, orange, 30) {
		t.Errorf("custom accent: meter %v, want %v", c, orange)
	}
}

func TestThemeType(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	base := darkTheme.metrics()
	big := newTheme(Theme{FontSize: 16}).metrics()
	if !(big.Line > base.Line) {
		t.Errorf("16pt line %v, want taller than 13pt's %v", big.Line, base.Line)
	}
	// The cell grid's mono font is set at the body size too.
	if !(big.Column > base.Column) {
		t.Errorf("16pt column %v, want wider than 13pt's %v", big.Column, base.Column)
	}
	if loose := newTheme(Theme{LineSpacing: 2}).metrics(); !(loose.Line > base.Line*1.4) {
		t.Errorf("spacing 2 line %v, want well above %v", loose.Line, base.Line)
	}
	// A named family is used: in a fixed-pitch family, i and M are as wide.
	menlo := newTheme(Theme{Font: "Menlo"})
	f := menlo.font(fontKey{})
	if i, M := f.width("iiii"), f.width("MMMM"); i != M {
		t.Errorf("Menlo UI: iiii %v vs MMMM %v, want equal", i, M)
	}
	if sys := darkTheme.font(fontKey{}); sys.width("iiii") >= sys.width("MMMM") {
		t.Error("system UI font is fixed-pitch?")
	}
	// Titles in a named family are its bold face, wider than its regular.
	sys := newTheme(Theme{Font: "Helvetica"})
	reg, title := sys.font(fontKey{}), sys.font(fontKey{role: layout.TextTitle})
	if !(title.width("Title") > reg.width("Title")) {
		t.Errorf("Helvetica title %v not wider than body %v", title.width("Title"), reg.width("Title"))
	}
	// And the window opens the same number of columns in either type.
	img, err := Snapshot(snapApp(func() layout.Node { return layout.Label{Text: "x"} }), Options{Width: 10, Height: 2, Theme: &Theme{FontSize: 16}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if h := img.Bounds().Dy(); h != int(2*big.Line) {
		t.Errorf("snapshot height %v, want 2 lines of %v", h, big.Line)
	}
}

func TestThemeRadius(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := lipgloss.Color("#ff0000")
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{ID: "box", H: layout.Fixed(4), Box: layout.Box{Border: layout.BorderNormal, BorderColor: red}}},
		}}
	}
	m := metrics()
	r := layout.LayoutScene(root(), 20*m.Column, 4*m.Line, m).Rects["box"]
	corner := func(radius float64) color.Color {
		img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 4, Theme: &Theme{Radius: radius}}, 2)
		if err != nil {
			t.Fatal(err)
		}
		// The stroke runs mid-band; its corner point is where it turns.
		return img.At(int((r.X+borderBand/2)*2), int((r.Y+borderBand/2)*2))
	}
	if c := corner(-1); !near(c, color.RGBA{255, 0, 0, 255}, 40) {
		t.Errorf("square corner %v, want the stroke", c)
	}
	if c := corner(6); near(c, color.RGBA{255, 0, 0, 255}, 120) {
		t.Errorf("rounded corner %v, want it cut off", c)
	}
}

func TestIsDarkFollowsAppearance(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	withPool(func() {
		view := alloc("NSView").Send(sel("initWithFrame:"), cgRect(0, 0, 10, 10))
		defer view.Send(sel("release"))
		for name, want := range map[string]bool{"NSAppearanceNameDarkAqua": true, "NSAppearanceNameAqua": false} {
			view.Send(sel("setAppearance:"), class("NSAppearance").Send(sel("appearanceNamed:"), nsString(name)))
			if got := isDark(view); got != want {
				t.Errorf("%s: isDark = %v", name, got)
			}
		}
		cls, err := registerViewClass()
		if err != nil {
			t.Fatal(err)
		}
		v := objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), cgRect(0, 0, 10, 10))
		defer v.Send(sel("release"))
		if !objc.Send[bool](v, sel("respondsToSelector:"), sel("viewDidChangeEffectiveAppearance")) {
			t.Error("view does not redraw on appearance changes")
		}
	})
}
