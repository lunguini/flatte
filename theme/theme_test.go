package theme

import (
	"image/color"
	"math"
	"testing"
)

func TestDefaultPassesItsOwnCheck(t *testing.T) {
	if problems := Default().Check(); len(problems) != 0 {
		for _, p := range problems {
			t.Error(p)
		}
	}
}

func TestContrastMatchesWCAG(t *testing.T) {
	for _, c := range []struct {
		a, b color.NRGBA
		want float64
	}{
		{Hex("#000000"), Hex("#ffffff"), 21},
		{Hex("#ffffff"), Hex("#ffffff"), 1},
		{Hex("#767676"), Hex("#ffffff"), 4.54},
	} {
		if got := Contrast(c.a, c.b); math.Abs(got-c.want) > 0.01 {
			t.Errorf("Contrast(%v, %v) = %.3f, want %.2f", c.a, c.b, got, c.want)
		}
	}
}

func TestHexAndAlpha(t *testing.T) {
	if got := Hex("#1f5fd1"); got != (color.NRGBA{0x1f, 0x5f, 0xd1, 0xff}) {
		t.Fatalf("Hex = %v", got)
	}
	if got := Hex("#1f5fd180"); got.A != 0x80 {
		t.Fatalf("Hex with alpha = %v", got)
	}
	if got := Hex("fff"); got != (color.NRGBA{0xff, 0xff, 0xff, 0xff}) {
		t.Fatalf("short Hex = %v", got)
	}
	// A wash over a surface composes to an opaque color.
	if got := Over(color.NRGBA{0, 0, 0, 0x80}, Hex("#ffffff")); got.A != 0xff || got.R < 0x7e || got.R > 0x80 {
		t.Fatalf("Over = %v", got)
	}
}

func TestDensityAndRoundness(t *testing.T) {
	d := Default()
	c := d.With(Seeds{Density: Compact, Roundness: Square})
	if c.Size.Row >= d.Size.Row || c.Space[2] >= d.Space[2] || c.Radius.Md != 0 {
		t.Fatalf("compact square: size %+v space %v radius %+v", c.Size, c.Space, c.Radius)
	}
	r := d.With(Seeds{Density: Comfortable, Roundness: Round})
	if r.Size.Row <= d.Size.Row || r.Radius.Lg <= d.Radius.Lg {
		t.Fatalf("comfortable round: size %+v radius %+v", r.Size, r.Radius)
	}
	// Regular / Subtle are the defaults: With leaves them as they are.
	if got := d.With(Seeds{}); got.Size != d.Size || got.Radius != d.Radius || got.Space != d.Space {
		t.Fatal("empty seeds changed the theme")
	}
}

func TestAccentSeedDerivesItsFamily(t *testing.T) {
	teal := Pair{Light: Hex("#0e7468"), Dark: Hex("#4fd1c0")}
	th := Default().With(Seeds{Accent: &teal})
	if th.Light.Accent != teal.Light || th.Dark.Accent != teal.Dark {
		t.Fatal("accent not set")
	}
	for _, p := range []Palette{th.Light, th.Dark} {
		if p.Focus != p.Accent || p.AccentHover == p.Accent || p.SelectionActive == Default().Light.SelectionActive {
			t.Fatalf("accent family not derived: %+v", p)
		}
	}
	if problems := th.Check(); len(problems) != 0 {
		t.Fatalf("a reasonable accent fails: %v", problems)
	}
	// A poor accent is reported, with the pair that fails.
	bad := Pair{Light: Hex("#ffe14d"), Dark: Hex("#3a3a3a")}
	problems := Default().With(Seeds{Accent: &bad}).Check()
	if len(problems) == 0 {
		t.Fatal("an unreadable accent passes")
	}
	if problems[0].Fg == "" || problems[0].Bg == "" || problems[0].Ratio >= problems[0].Need {
		t.Fatalf("problem lacks detail: %+v", problems[0])
	}
}

func TestPaletteForAppearance(t *testing.T) {
	d := Default()
	if d.Palette(true) != &d.Dark || d.Palette(false) != &d.Light {
		t.Fatal("Palette picks the wrong appearance")
	}
}

func TestShowcaseAccentsAllPass(t *testing.T) {
	for name, p := range map[string]Pair{
		"blue":     {Hex("#1f5fd1"), Hex("#6ea2ff")},
		"teal":     {Hex("#0e7468"), Hex("#4fd1c0")},
		"violet":   {Hex("#6d3fd8"), Hex("#b69bff")},
		"graphite": {Hex("#3d4450"), Hex("#c3c9d3")},
	} {
		p := p
		for _, s := range []Seeds{{Accent: &p}, {Accent: &p, Density: Compact, Roundness: Square}} {
			if problems := Default().With(s).Check(); len(problems) != 0 {
				t.Errorf("%s: %v", name, problems)
			}
		}
	}
}
