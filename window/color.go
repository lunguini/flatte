package window

import (
	"image/color"

	"github.com/lunguini/flatte/flatui/layout"
)

type rgba struct{ R, G, B, A float64 }

// rgbaOf is c with straight alpha, as drawing backends take it (color.Color's
// RGBA method returns premultiplied channels).
func rgbaOf(c color.Color) rgba {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return rgba{}
	}
	return rgba{float64(r) / float64(a), float64(g) / float64(a), float64(b) / float64(a), float64(a) / 0xffff}
}

func roleRGBA(style layout.TextStyle, body, subtle, title rgba) rgba {
	if style.Fg != nil {
		return rgbaOf(style.Fg)
	}
	switch style.Role {
	case layout.TextTitle:
		return title
	case layout.TextSubtle:
		return subtle
	default:
		return body
	}
}

func mix(a, b rgba, t float64) rgba {
	return rgba{a.R + (b.R-a.R)*t, a.G + (b.G-a.G)*t, a.B + (b.B-a.B)*t, a.A + (b.A-a.A)*t}
}
