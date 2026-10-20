package window

import (
	"math"
	"testing"
)

// cell is the rect the glyph tests draw into: fractional, as real columns are.
const (
	gx, gy, gw, gh = 10.4, 20, 7.8, 20
)

func near1(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// spans reports whether the rects together cover [lo, hi] along one axis
// without a gap.
func spans(rs []shapeRect, horizontal bool, lo, hi float64) bool {
	at := lo
	for grew := true; grew && at < hi-1e-9; {
		grew = false
		for _, r := range rs {
			a, b := r.Y, r.Y+r.H
			if horizontal {
				a, b = r.X, r.X+r.W
			}
			if a <= at+1e-9 && b > at+1e-9 {
				at, grew = b, true
			}
		}
	}
	return at >= hi-1e-9
}

func TestCellGlyphLinesReachTheCellEdges(t *testing.T) {
	// A line drawn as a glyph stops short of the cell when rows are taller
	// than the font; drawn as a shape it must reach the edges so neighbors
	// join.
	h, ok := cellGlyph('─', gx, gy, gw, gh)
	if !ok || !spans(h.rects, true, gx, gx+gw) {
		t.Fatalf("─ does not span the cell width: %+v", h)
	}
	v, ok := cellGlyph('│', gx, gy, gw, gh)
	if !ok || !spans(v.rects, false, gy, gy+gh) {
		t.Fatalf("│ does not span the cell height: %+v", v)
	}
	// Lines sit on whole points so they are crisp at any scale, and one
	// column/row draws them at the same place in every cell.
	if r := v.rects[0]; r.X != math.Floor(r.X) || r.W != 1 {
		t.Errorf("│ at x=%v w=%v, want a whole-point 1pt line", r.X, r.W)
	}
	if r := h.rects[0]; r.Y != math.Floor(r.Y) || r.H != 1 {
		t.Errorf("─ at y=%v h=%v, want a whole-point 1pt line", r.Y, r.H)
	}
	if hv, _ := cellGlyph('━', gx, gy, gw, gh); hv.rects[0].H != 2 {
		t.Errorf("━ is %vpt, want heavy 2pt", hv.rects[0].H)
	}
}

func TestCellGlyphJunctionsAndCorners(t *testing.T) {
	cross, _ := cellGlyph('┼', gx, gy, gw, gh)
	if !spans(cross.rects, true, gx, gx+gw) || !spans(cross.rects, false, gy, gy+gh) {
		t.Errorf("┼ does not reach all four edges: %+v", cross.rects)
	}
	// ┌ has arms right and down only.
	corner, _ := cellGlyph('┌', gx, gy, gw, gh)
	for _, r := range corner.rects {
		if r.X < math.Floor(gx+gw/2)-1e-9 || r.Y < math.Floor(gy+gh/2)-1e-9 {
			t.Errorf("┌ draws up or left of its center: %+v", r)
		}
	}
	if !spans(corner.rects, true, math.Floor(gx+gw/2), gx+gw) || !spans(corner.rects, false, math.Floor(gy+gh/2), gy+gh) {
		t.Errorf("┌ arms do not reach right and bottom: %+v", corner.rects)
	}
	// Rounded corners are an arc from one edge to the other.
	round, ok := cellGlyph('╭', gx, gy, gw, gh)
	if !ok || len(round.arcs) != 1 || len(round.rects) != 0 {
		t.Fatalf("╭ = %+v", round)
	}
	a := round.arcs[0]
	if !near1(a.from.X, gx+gw) || !near1(a.to.Y, gy+gh) || !(a.r > 0) {
		t.Errorf("╭ arc %+v does not run from the right edge to the bottom edge", a)
	}
	// Double lines are two parallel lines; corners join outer to outer.
	dbl, _ := cellGlyph('═', gx, gy, gw, gh)
	if len(dbl.rects) != 2 || dbl.rects[0].Y == dbl.rects[1].Y {
		t.Errorf("═ = %+v, want two parallel lines", dbl.rects)
	}
	if dc, ok := cellGlyph('╔', gx, gy, gw, gh); !ok || len(dc.rects) != 4 {
		t.Errorf("╔ = %+v, want two L-shapes", dc)
	}
}

func TestCellGlyphBlocks(t *testing.T) {
	for _, c := range []struct {
		r          rune
		x, y, w, h float64 // fractions of the cell
		alpha      float64
	}{
		{'█', 0, 0, 1, 1, 1},
		{'▀', 0, 0, 1, 0.5, 1},
		{'▄', 0, 0.5, 1, 0.5, 1},
		{'▌', 0, 0, 0.5, 1, 1},
		{'▐', 0.5, 0, 0.5, 1, 1},
		{'▁', 0, 7.0 / 8, 1, 1.0 / 8, 1},
		{'▇', 0, 1.0 / 8, 1, 7.0 / 8, 1},
		{'▏', 0, 0, 1.0 / 8, 1, 1},
		{'░', 0, 0, 1, 1, 0.25},
		{'▓', 0, 0, 1, 1, 0.75},
	} {
		g, ok := cellGlyph(c.r, gx, gy, gw, gh)
		if !ok || len(g.rects) != 1 {
			t.Errorf("%c = %+v", c.r, g)
			continue
		}
		r := g.rects[0]
		want := shapeRect{gx + c.x*gw, gy + c.y*gh, c.w * gw, c.h * gh, c.alpha}
		if !near1(r.X, want.X) || !near1(r.Y, want.Y) || !near1(r.W, want.W) || !near1(r.H, want.H) || r.Alpha != want.Alpha {
			t.Errorf("%c = %+v, want %+v", c.r, r, want)
		}
	}
	// ▚ is two quadrants: top-left and bottom-right.
	if g, _ := cellGlyph('▚', gx, gy, gw, gh); len(g.rects) != 2 {
		t.Errorf("▚ = %+v", g.rects)
	}
}

func TestCellGlyphPowerline(t *testing.T) {
	// U+E0BC fills the upper-left triangle, U+E0BA the lower-right one.
	ul, ok := cellGlyph('', gx, gy, gw, gh)
	if !ok || len(ul.polys) != 1 || len(ul.polys[0].pts) != 3 || !ul.polys[0].fill {
		t.Fatalf("U+E0BC = %+v", ul)
	}
	for _, p := range ul.polys[0].pts {
		if (p.X-gx)/gw+(p.Y-gy)/gh > 1+1e-9 {
			t.Errorf("U+E0BC point %+v is below the diagonal", p)
		}
	}
	lr, _ := cellGlyph('', gx, gy, gw, gh)
	for _, p := range lr.polys[0].pts {
		if (p.X-gx)/gw+(p.Y-gy)/gh < 1-1e-9 {
			t.Errorf("U+E0BA point %+v is above the diagonal", p)
		}
	}
	if arrow, ok := cellGlyph('', gx, gy, gw, gh); !ok || arrow.polys[0].fill {
		t.Errorf("U+E0B1 = %+v, want a stroked chevron", arrow)
	}
}

func TestCellGlyphLeavesTextAlone(t *testing.T) {
	for _, r := range []rune{'a', ' ', '●', '▸', '┍', '╠'} {
		if _, ok := cellGlyph(r, gx, gy, gw, gh); ok {
			t.Errorf("%q is drawn as a shape; it should be a font glyph", r)
		}
	}
}
