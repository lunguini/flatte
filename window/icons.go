package window

import "github.com/lunguini/flatte/flatui/layout"

// iconShape is the shape of a layout.Icon drawn in the rect (x, y, w, h):
// centered on the first column (where the terminal spells the marker) and
// on the line, half a line high and no wider than that column. It reports false for IconNone and unknown
// names, which draw their text.
func iconShape(name layout.IconName, x, y, w, h, column float64) (cellShape, bool) {
	col := min(column, w)
	cx, cy := x+col/2, y+h/2
	s := min(h/2, col) // the icon's box: half a line, within one column
	switch name {
	case layout.IconChevronRight:
		return cellShape{polys: []poly{{pts: []point{{cx - s/4, cy - s/2}, {cx + s/4, cy}, {cx - s/4, cy + s/2}}, width: 1.5}}}, true
	case layout.IconDisclosureClosed:
		return cellShape{polys: []poly{{pts: []point{{cx - s*0.3, cy - s*0.4}, {cx + s*0.4, cy}, {cx - s*0.3, cy + s*0.4}}, fill: true}}}, true
	case layout.IconDisclosureOpen:
		return cellShape{polys: []poly{{pts: []point{{cx - s*0.4, cy - s*0.3}, {cx + s*0.4, cy - s*0.3}, {cx, cy + s*0.4}}, fill: true}}}, true
	case layout.IconDot:
		return cellShape{circles: []circle{{c: point{cx, cy}, r: s * 0.35, fill: true}}}, true
	case layout.IconCircle:
		return cellShape{circles: []circle{{c: point{cx, cy}, r: s*0.35 - 0.5}}}, true
	}
	return cellShape{}, false
}
