package window

import (
	"math"

	"github.com/lunguini/flatte/flatui/layout"
)

// iconShape is the shape of a layout.Icon drawn in the rect (x, y, w, h).
// The first shapes are centered on the first column (where the terminal
// spells the marker) and on the line, half a line high and no wider than
// that column; the rest are 0.7 of a line high (no wider than the rect),
// centered in the rect. It reports false for IconNone and unknown names,
// which draw their text.
func iconShape(name layout.IconName, x, y, w, h, column float64) (cellShape, bool) {
	col := min(column, w)
	cx, cy := x+col/2, y+h/2
	s := min(h/2, col) // the icon's box: half a line, within one column
	chevron := []point{{cx - s/4, cy - s/2}, {cx + s/4, cy}, {cx - s/4, cy + s/2}}
	switch name {
	case layout.IconChevronRight:
		return cellShape{polys: []poly{{pts: chevron, width: 1.5}}}, true
	case layout.IconChevronLeft, layout.IconChevronDown, layout.IconChevronUp:
		turns := map[layout.IconName]int{layout.IconChevronDown: 1, layout.IconChevronLeft: 2, layout.IconChevronUp: 3}[name]
		return cellShape{polys: []poly{{pts: rotated(chevron, point{cx, cy}, turns), width: 1.5}}}, true
	case layout.IconDisclosureClosed:
		return cellShape{polys: []poly{{pts: []point{{cx - s*0.3, cy - s*0.4}, {cx + s*0.4, cy}, {cx - s*0.3, cy + s*0.4}}, fill: true}}}, true
	case layout.IconDisclosureOpen:
		return cellShape{polys: []poly{{pts: []point{{cx - s*0.4, cy - s*0.3}, {cx + s*0.4, cy - s*0.3}, {cx, cy + s*0.4}}, fill: true}}}, true
	case layout.IconDot:
		return cellShape{circles: []circle{{c: point{cx, cy}, r: s * 0.35, fill: true}}}, true
	case layout.IconCircle:
		return cellShape{circles: []circle{{c: point{cx, cy}, r: s*0.35 - 0.5}}}, true
	case layout.IconBlank:
		return cellShape{}, true
	}
	return symbol(name, x+w/2, cy, min(0.7*h, w))
}

// symbol is one of the larger icons, in an s-point box centered on (cx, cy).
func symbol(name layout.IconName, cx, cy, s float64) (cellShape, bool) {
	// at is a point of the box in units of s from its center.
	at := func(a, b float64) point { return point{cx + a*s, cy + b*s} }
	sw := max(1.25, s/9) // stroke width
	line := func(pts ...point) poly { return poly{pts: pts, width: sw, round: true} }
	ring := func(c point, r float64) circle { return circle{c: c, r: r - sw/2, width: sw} }
	dot := func(c point, r float64) circle { return circle{c: c, r: r, fill: true} }
	cross := func(d float64) []poly {
		return []poly{line(at(-d, -d), at(d, d)), line(at(d, -d), at(-d, d))}
	}
	arrow := []point{at(-0.4, 0), at(0.4, 0)}
	head := []point{at(0.08, -0.32), at(0.4, 0), at(0.08, 0.32)}
	turns := map[layout.IconName]int{layout.IconArrowDown: 1, layout.IconArrowLeft: 2, layout.IconArrowUp: 3}
	switch name {
	case layout.IconCheck:
		return cellShape{polys: []poly{line(at(-0.4, 0.02), at(-0.12, 0.32), at(0.42, -0.32))}}, true
	case layout.IconClose:
		return cellShape{polys: cross(0.32)}, true
	case layout.IconPlus:
		return cellShape{polys: []poly{line(at(-0.38, 0), at(0.38, 0)), line(at(0, -0.38), at(0, 0.38))}}, true
	case layout.IconMinus:
		return cellShape{polys: []poly{line(at(-0.38, 0), at(0.38, 0))}}, true
	case layout.IconSearch:
		return cellShape{
			circles: []circle{ring(at(-0.08, -0.08), 0.32*s)},
			polys:   []poly{{pts: []point{at(0.16, 0.16), at(0.42, 0.42)}, width: sw * 1.4, round: true}},
		}, true
	case layout.IconGear:
		g := cellShape{circles: []circle{{c: at(0, 0), r: 0.24 * s, width: 0.16 * s}}}
		for k := range 8 {
			a := float64(k) * math.Pi / 4
			c, sn := math.Cos(a), math.Sin(a)
			g.polys = append(g.polys, poly{pts: []point{at(0.28*c, 0.28*sn), at(0.47*c, 0.47*sn)}, width: 0.17 * s})
		}
		return g, true
	case layout.IconInfo:
		return cellShape{
			circles: []circle{ring(at(0, 0), 0.5*s), dot(at(0, -0.2), max(0.065*s, sw*0.6))},
			polys:   []poly{line(at(0, -0.04), at(0, 0.26))},
		}, true
	case layout.IconWarning:
		return cellShape{
			polys:   []poly{line(at(0, -0.44), at(0.48, 0.4), at(-0.48, 0.4), at(0, -0.44)), line(at(0, -0.14), at(0, 0.1))},
			circles: []circle{dot(at(0, 0.24), max(0.06*s, sw*0.6))},
		}, true
	case layout.IconError:
		return cellShape{circles: []circle{ring(at(0, 0), 0.5*s)}, polys: cross(0.17)}, true
	case layout.IconArrowRight, layout.IconArrowDown, layout.IconArrowLeft, layout.IconArrowUp:
		c := at(0, 0)
		return cellShape{polys: []poly{line(rotated(arrow, c, turns[name])...), line(rotated(head, c, turns[name])...)}}, true
	case layout.IconEllipsis:
		r := max(0.07*s, 1)
		return cellShape{circles: []circle{dot(at(-0.32, 0), r), dot(at(0, 0), r), dot(at(0.32, 0), r)}}, true
	case layout.IconFolder:
		return cellShape{polys: []poly{line(at(-0.45, -0.3), at(-0.14, -0.3), at(-0.04, -0.2), at(0.45, -0.2), at(0.45, 0.34), at(-0.45, 0.34), at(-0.45, -0.3))}}, true
	case layout.IconFile:
		return cellShape{polys: []poly{
			line(at(-0.32, -0.45), at(0.1, -0.45), at(0.32, -0.23), at(0.32, 0.45), at(-0.32, 0.45), at(-0.32, -0.45)),
			line(at(0.1, -0.45), at(0.1, -0.23), at(0.32, -0.23)),
		}}, true
	}
	return cellShape{}, false
}

// rotated is pts turned clockwise (down is +y) by quarter turns about c.
func rotated(pts []point, c point, turns int) []point {
	out := make([]point, len(pts))
	for i, p := range pts {
		dx, dy := p.X-c.X, p.Y-c.Y
		for range turns % 4 {
			dx, dy = -dy, dx
		}
		out[i] = point{c.X + dx, c.Y + dy}
	}
	return out
}
