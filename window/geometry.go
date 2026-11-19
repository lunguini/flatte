package window

import "github.com/lunguini/flatte/flatui/layout"

// Point geometry shared by the native and canvas painters.
const (
	stepPoints             = 8.0
	borderBand             = 12.0
	strokeWidth            = 1.0
	fieldInset             = 4.0
	maxFontSize            = 512.0
	pictureRound           = 0.12
	minLeading, maxLeading = 0.8, 3.0
)

func intersect(a, b layout.PointRect) layout.PointRect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	return layout.PointRect{X: x0, Y: y0, W: max(x1-x0, 0), H: max(y1-y0, 0)}
}

func runeStart(s string, i int) bool { return i == len(s) || s[i]&0xC0 != 0x80 }

// edgeRect is the band of r a Box.Edge on side covers, width thick.
func edgeRect(r layout.PointRect, side layout.Side, width float64) (layout.PointRect, bool) {
	width = min(width, r.W, r.H)
	switch side {
	case layout.SideTop:
		return layout.PointRect{X: r.X, Y: r.Y, W: r.W, H: width}, true
	case layout.SideBottom:
		return layout.PointRect{X: r.X, Y: r.Y + r.H - width, W: r.W, H: width}, true
	case layout.SideLeft:
		return layout.PointRect{X: r.X, Y: r.Y, W: width, H: r.H}, true
	case layout.SideRight:
		return layout.PointRect{X: r.X + r.W - width, Y: r.Y, W: width, H: r.H}, true
	}
	return layout.PointRect{}, false
}

// flatExtend is r reaching rad past its side s, so a rounded rect drawn
// over it and clipped to r has square corners along s (Box.Flat); false
// for no side.
func flatExtend(r layout.PointRect, s layout.Side, rad float64) (layout.PointRect, bool) {
	switch s {
	case layout.SideTop:
		r.Y -= rad
		r.H += rad
	case layout.SideBottom:
		r.H += rad
	case layout.SideLeft:
		r.X -= rad
		r.W += rad
	case layout.SideRight:
		r.W += rad
	default:
		return r, false
	}
	return r, true
}
