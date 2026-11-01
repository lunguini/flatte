package window

import "github.com/lunguini/flatte/flatui/layout"

// fitRect is where a w×h picture goes in r under fit.
func fitRect(r layout.PointRect, w, h float64, fit layout.ImageFit) layout.PointRect {
	if fit == layout.ImageStretch || w <= 0 || h <= 0 {
		return r
	}
	s := min(r.W/w, r.H/h)
	if fit == layout.ImageCover {
		s = max(r.W/w, r.H/h)
	}
	fw, fh := w*s, h*s
	return layout.PointRect{X: r.X + (r.W-fw)/2, Y: r.Y + (r.H-fh)/2, W: fw, H: fh}
}
