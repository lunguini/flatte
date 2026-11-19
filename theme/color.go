package theme

import (
	"image/color"
	"math"
	"strconv"
	"strings"
)

// Hex is a color written #rgb, #rrggbb or #rrggbbaa (the # is optional). A
// malformed string is opaque magenta, so a typo shows instead of hiding.
func Hex(s string) color.NRGBA {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) == 6 {
		s += "ff"
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if len(s) != 8 || err != nil {
		return color.NRGBA{0xff, 0, 0xff, 0xff}
	}
	return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

// Over is c composited over an opaque bg: what a translucent wash looks
// like on that surface (a terminal paints only opaque cells).
func Over(c, bg color.NRGBA) color.NRGBA {
	a := float64(c.A) / 255
	mixc := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*a + float64(y)*(1-a))) }
	return color.NRGBA{mixc(c.R, bg.R), mixc(c.G, bg.G), mixc(c.B, bg.B), 0xff}
}

// Contrast is the WCAG 2 contrast ratio of two colors, 1 to 21; a
// translucent fg is first composited over bg.
func Contrast(fg, bg color.NRGBA) float64 {
	if fg.A < 0xff {
		fg = Over(fg, bg)
	}
	a, b := luminance(fg), luminance(bg)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

func luminance(c color.NRGBA) float64 {
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

func linear(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func unlinear(c float64) uint8 {
	c = math.Max(0, math.Min(1, c))
	if c <= 0.0031308 {
		c *= 12.92
	} else {
		c = 1.055*math.Pow(c, 1/2.4) - 0.055
	}
	return uint8(math.Round(c * 255))
}

// oklab is a color in OKLab, where mixing and lightening look even.
type oklab struct{ L, a, b float64 }

func toOKLab(c color.NRGBA) oklab {
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return oklab{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

func (o oklab) nrgba() color.NRGBA {
	l := o.L + 0.3963377774*o.a + 0.2158037573*o.b
	m := o.L - 0.1055613458*o.a - 0.0638541728*o.b
	s := o.L - 0.0894841775*o.a - 1.2914855480*o.b
	l, m, s = l*l*l, m*m*m, s*s*s
	return color.NRGBA{
		unlinear(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		unlinear(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		unlinear(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s),
		0xff,
	}
}

// Mix is a and b mixed in OKLab, t of the way from a to b.
func Mix(a, b color.NRGBA, t float64) color.NRGBA {
	x, y := toOKLab(a), toOKLab(b)
	return oklab{x.L + (y.L-x.L)*t, x.a + (y.a-x.a)*t, x.b + (y.b-x.b)*t}.nrgba()
}

// shift moves c's lightness by d (OKLab L, 0–1), keeping its hue.
func shift(c color.NRGBA, d float64) color.NRGBA {
	o := toOKLab(c)
	o.L = math.Max(0, math.Min(1, o.L+d))
	return o.nrgba()
}
