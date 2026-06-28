package layout

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// Overlay draws layer centered over base as an ANSI-aware cell composite.
func Overlay(base, layer string) string {
	x, y := OverlayOrigin(base, layer)
	return compositeAt(base, layer, x, y)
}

// OverlayOrigin is where Overlay places layer's top-left cell. Overlay calls
// this, so the two cannot drift apart.
func OverlayOrigin(base, layer string) (x, y int) {
	bb := uv.NewStyledString(base).Bounds()
	lb := uv.NewStyledString(layer).Bounds()
	if bb.Empty() || lb.Empty() {
		return 0, 0
	}
	w := max(bb.Dx(), lb.Dx())
	h := max(bb.Dy(), lb.Dy())
	return max(0, (w-lb.Dx())/2), max(0, (h-lb.Dy())/2)
}

// compositeAt paints layer onto base at cell (x,y); the layer rectangle covers
// the base (so styled backgrounds survive on both sides).
func compositeAt(base, layer string, x, y int) string {
	baseStyled := uv.NewStyledString(base)
	layerStyled := uv.NewStyledString(layer)
	bb := baseStyled.Bounds()
	if bb.Empty() {
		return base
	}
	lb := layerStyled.Bounds()
	w := max(bb.Dx(), lb.Dx())
	h := max(bb.Dy(), lb.Dy())
	canvas := uv.NewScreenBuffer(w, h)
	baseStyled.Draw(canvas, canvas.Bounds())
	if !lb.Empty() {
		area := uv.Rect(x, y, lb.Dx(), lb.Dy())
		canvas.FillArea(&uv.EmptyCell, area)
		layerStyled.Draw(canvas, area)
	}
	return trimTrailingSpaceLayout(canvas.Render())
}

func trimTrailingSpaceLayout(s string) string {
	rows := strings.Split(s, "\n")
	for i, row := range rows {
		rows[i] = strings.TrimRight(row, " \t")
	}
	return strings.Join(rows, "\n")
}
