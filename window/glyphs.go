package window

import "math"

// Terminal content in a window is drawn on a grid whose rows are taller
// than the font, so line-drawing and block characters drawn from the font
// would leave gaps between rows. As terminal emulators do, the grid draws
// them as shapes that fill their cell exactly. cellGlyph is the geometry;
// the platform painter only fills and strokes it.

type point struct{ X, Y float64 }

// shapeRect is a filled rect; Alpha scales the foreground's opacity.
type shapeRect struct{ X, Y, W, H, Alpha float64 }

// arc is a rounded corner: a 1pt stroke from from toward corner, turning
// with radius r, and on to to.
type arc struct {
	from, corner, to point
	r                float64
}

// poly is a closed filled polygon, or an open stroked polyline width points
// wide (0 means 1pt).
type poly struct {
	pts   []point
	fill  bool
	width float64
}

// circle is a filled disc, or a 1pt ring, centered on c.
type circle struct {
	c    point
	r    float64
	fill bool
}

type cellShape struct {
	rects   []shapeRect
	arcs    []arc
	polys   []poly
	circles []circle
}

// Arm weights of a line-drawing character, clockwise from up.
const (
	armLight  = 1
	armHeavy  = 2
	armDouble = 3
)

type arms [4]uint8 // up, right, down, left

var lineArms = map[rune]arms{
	'─': {0, 1, 0, 1}, '│': {1, 0, 1, 0}, '┌': {0, 1, 1, 0}, '┐': {0, 0, 1, 1},
	'└': {1, 1, 0, 0}, '┘': {1, 0, 0, 1}, '├': {1, 1, 1, 0}, '┤': {1, 0, 1, 1},
	'┬': {0, 1, 1, 1}, '┴': {1, 1, 0, 1}, '┼': {1, 1, 1, 1},
	'━': {0, 2, 0, 2}, '┃': {2, 0, 2, 0}, '┏': {0, 2, 2, 0}, '┓': {0, 0, 2, 2},
	'┗': {2, 2, 0, 0}, '┛': {2, 0, 0, 2}, '┣': {2, 2, 2, 0}, '┫': {2, 0, 2, 2},
	'┳': {0, 2, 2, 2}, '┻': {2, 2, 0, 2}, '╋': {2, 2, 2, 2},
	'╴': {0, 0, 0, 1}, '╵': {1, 0, 0, 0}, '╶': {0, 1, 0, 0}, '╷': {0, 0, 1, 0},
	'╸': {0, 0, 0, 2}, '╹': {2, 0, 0, 0}, '╺': {0, 2, 0, 0}, '╻': {0, 0, 2, 0},
	// Dashed lines draw solid.
	'┄': {0, 1, 0, 1}, '┈': {0, 1, 0, 1}, '╌': {0, 1, 0, 1},
	'┆': {1, 0, 1, 0}, '┊': {1, 0, 1, 0}, '╎': {1, 0, 1, 0},
	'┅': {0, 2, 0, 2}, '┉': {0, 2, 0, 2}, '╍': {0, 2, 0, 2},
	'┇': {2, 0, 2, 0}, '┋': {2, 0, 2, 0}, '╏': {2, 0, 2, 0},
	// Double lines: straight runs and corners. Double tees and crosses keep
	// the font glyph.
	'═': {0, 3, 0, 3}, '║': {3, 0, 3, 0}, '╔': {0, 3, 3, 0}, '╗': {0, 0, 3, 3},
	'╚': {3, 3, 0, 0}, '╝': {3, 0, 0, 3},
}

var roundCorners = map[rune]arms{
	'╭': {0, 1, 1, 0}, '╮': {0, 0, 1, 1}, '╯': {1, 0, 0, 1}, '╰': {1, 1, 0, 0},
}

// Quadrant blocks: top-left, top-right, bottom-left, bottom-right.
var quadrants = map[rune][4]bool{
	'▖': {false, false, true, false}, '▗': {false, false, false, true},
	'▘': {true, false, false, false}, '▝': {false, true, false, false},
	'▙': {true, false, true, true}, '▚': {true, false, false, true},
	'▛': {true, true, true, false}, '▜': {true, true, false, true},
	'▞': {false, true, true, false}, '▟': {false, true, true, true},
}

// Powerline separators, in fractions of the cell.
var powerline = map[rune]poly{
	'': {pts: []point{{0, 0}, {1, 0.5}, {0, 1}}, fill: true},
	'': {pts: []point{{0, 0}, {1, 0.5}, {0, 1}}},
	'': {pts: []point{{1, 0}, {0, 0.5}, {1, 1}}, fill: true},
	'': {pts: []point{{1, 0}, {0, 0.5}, {1, 1}}},
	'': {pts: []point{{0, 0}, {1, 1}, {0, 1}}, fill: true},
	'': {pts: []point{{1, 0}, {1, 1}, {0, 1}}, fill: true},
	'': {pts: []point{{0, 0}, {1, 0}, {0, 1}}, fill: true},
	'': {pts: []point{{0, 0}, {1, 0}, {1, 1}}, fill: true},
}

// cellGlyph returns the shape of r drawn in the cell (x, y, w, h), or false
// when r is ordinary text for the font.
func cellGlyph(r rune, x, y, w, h float64) (cellShape, bool) {
	// Lines sit on whole points, the same in every cell of a row or column,
	// so they are crisp and join their neighbors.
	cx, cy := math.Floor(x+w/2), math.Floor(y+h/2)
	if a, ok := lineArms[r]; ok {
		if a[0] == armDouble || a[1] == armDouble {
			return cellShape{rects: doubleLines(a, x, y, w, h, cx, cy)}, true
		}
		return cellShape{rects: singleLines(a, x, y, w, h, cx, cy)}, true
	}
	if a, ok := roundCorners[r]; ok {
		c := point{cx + 0.5, cy + 0.5}
		from, to := point{x + w, c.Y}, point{c.X, y + h}
		if a[3] != 0 {
			from.X = x
		}
		if a[0] != 0 {
			to.Y = y
		}
		return cellShape{arcs: []arc{{from: from, corner: c, to: to, r: max(min(w, h)/2-1, 1)}}}, true
	}
	frac := func(fx, fy, fw, fh, alpha float64) cellShape {
		return cellShape{rects: []shapeRect{{x + fx*w, y + fy*h, fw * w, fh * h, alpha}}}
	}
	switch {
	case r == '▀':
		return frac(0, 0, 1, 0.5, 1), true
	case r >= '▁' && r <= '█':
		k := float64(r-'▀') / 8
		return frac(0, 1-k, 1, k, 1), true
	case r >= '▉' && r <= '▏':
		return frac(0, 0, float64(0x2590-r)/8, 1, 1), true
	case r == '▐':
		return frac(0.5, 0, 0.5, 1, 1), true
	case r == '▔':
		return frac(0, 0, 1, 1.0/8, 1), true
	case r == '▕':
		return frac(7.0/8, 0, 1.0/8, 1, 1), true
	case r >= '░' && r <= '▓':
		return frac(0, 0, 1, 1, float64(r-'░'+1)/4), true
	}
	if q, ok := quadrants[r]; ok {
		var s cellShape
		for i, on := range q {
			if on {
				s.rects = append(s.rects, frac(float64(i%2)/2, float64(i/2)/2, 0.5, 0.5, 1).rects[0])
			}
		}
		return s, true
	}
	if p, ok := powerline[r]; ok {
		pts := make([]point, len(p.pts))
		for i, f := range p.pts {
			pts[i] = point{x + f.X*w, y + f.Y*h}
		}
		return cellShape{polys: []poly{{pts: pts, fill: p.fill}}}, true
	}
	return cellShape{}, false
}

// singleLines draws light and heavy arms from the cell's center to its
// edges; each arm covers the center so arms of a junction join.
func singleLines(a arms, x, y, w, h, cx, cy float64) []shapeRect {
	var rs []shapeRect
	// A straight run is one rect.
	if a[1] == a[3] && a[0] == 0 && a[2] == 0 {
		return []shapeRect{{x, cy, w, float64(a[1]), 1}}
	}
	if a[0] == a[2] && a[1] == 0 && a[3] == 0 {
		return []shapeRect{{cx, y, float64(a[0]), h, 1}}
	}
	if t := float64(a[0]); t > 0 {
		rs = append(rs, shapeRect{cx, y, t, cy + t - y, 1})
	}
	if t := float64(a[1]); t > 0 {
		rs = append(rs, shapeRect{cx, cy, x + w - cx, t, 1})
	}
	if t := float64(a[2]); t > 0 {
		rs = append(rs, shapeRect{cx, cy, t, y + h - cy, 1})
	}
	if t := float64(a[3]); t > 0 {
		rs = append(rs, shapeRect{x, cy, cx + t - x, t, 1})
	}
	return rs
}

// doubleLines draws a straight double run or a double corner: two lines a
// point apart either side of the center, the outer and inner pair each
// meeting at their own corner.
func doubleLines(a arms, x, y, w, h, cx, cy float64) []shapeRect {
	switch {
	case a[1] != 0 && a[3] != 0:
		return []shapeRect{{x, cy - 1, w, 1, 1}, {x, cy + 1, w, 1, 1}}
	case a[0] != 0 && a[2] != 0:
		return []shapeRect{{cx - 1, y, 1, h, 1}, {cx + 1, y, 1, h, 1}}
	}
	sx, sy := 1.0, 1.0 // toward the arms: right, down
	if a[3] != 0 {
		sx = -1
	}
	if a[0] != 0 {
		sy = -1
	}
	var rs []shapeRect
	for _, off := range []float64{-1, 1} { // outer, inner
		px, py := cx+off*sx, cy+off*sy
		if sx > 0 {
			rs = append(rs, shapeRect{px, py, x + w - px, 1, 1})
		} else {
			rs = append(rs, shapeRect{x, py, px + 1 - x, 1, 1})
		}
		if sy > 0 {
			rs = append(rs, shapeRect{px, py, 1, y + h - py, 1})
		} else {
			rs = append(rs, shapeRect{px, y, 1, py + 1 - y, 1})
		}
	}
	return rs
}
