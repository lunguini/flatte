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
