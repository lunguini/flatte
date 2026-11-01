package window

import (
	"math"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

// The icon tests draw into a 2-column, 20pt line at a fractional position.
const ix, iy, iw, ih, icol = 30.5, 40.0, 15.6, 20.0, 7.8

func inside(p point) bool { return p.X >= ix && p.X <= ix+iw && p.Y >= iy && p.Y <= iy+ih }

func TestIconShapesStayInTheirColumns(t *testing.T) {
	for _, name := range []layout.IconName{layout.IconChevronRight, layout.IconDisclosureClosed, layout.IconDisclosureOpen, layout.IconDot, layout.IconCircle} {
		s, ok := iconShape(name, ix, iy, iw, ih, icol)
		if !ok {
			t.Fatalf("icon %d has no shape", name)
		}
		for _, p := range s.polys {
			for _, pt := range p.pts {
				if !inside(pt) {
					t.Errorf("icon %d point %+v outside its rect", name, pt)
				}
			}
		}
		for _, c := range s.circles {
			if !inside(point{c.c.X - c.r, c.c.Y - c.r}) || !inside(point{c.c.X + c.r, c.c.Y + c.r}) {
				t.Errorf("icon %d circle %+v outside its rect", name, c)
			}
		}
	}
	// A blank icon is a shape with nothing in it: it draws nothing.
	if blank, ok := iconShape(layout.IconBlank, ix, iy, iw, ih, icol); !ok || len(blank.rects)+len(blank.arcs)+len(blank.polys)+len(blank.circles) != 0 {
		t.Errorf("IconBlank = %+v, %v; want an empty shape", blank, ok)
	}
	if _, ok := iconShape(layout.IconNone, ix, iy, iw, ih, icol); ok {
		t.Error("IconNone has a shape; it should draw its text")
	}
	if _, ok := iconShape(layout.IconName(99), ix, iy, iw, ih, icol); ok {
		t.Error("an unknown icon has a shape; it should draw its text")
	}
}

func TestIconShapes(t *testing.T) {
	// Icons sit where the terminal spells them: centered on the first
	// column (the marker), vertically centered in the line.
	cx, cy := ix+icol/2, iy+ih/2
	dot, _ := iconShape(layout.IconDot, ix, iy, iw, ih, icol)
	if len(dot.circles) != 1 || !dot.circles[0].fill || dot.circles[0].c != (point{cx, cy}) {
		t.Fatalf("dot = %+v, want a filled circle at %v,%v", dot, cx, cy)
	}
	ring, _ := iconShape(layout.IconCircle, ix, iy, iw, ih, icol)
	if len(ring.circles) != 1 || ring.circles[0].fill {
		t.Fatalf("circle = %+v, want an outlined circle", ring)
	}
	// ▸ points right: its tip is its rightmost point, on the center line.
	closed, _ := iconShape(layout.IconDisclosureClosed, ix, iy, iw, ih, icol)
	tip := closed.polys[0].pts[1]
	for _, p := range closed.polys[0].pts {
		if p.X > tip.X {
			t.Fatalf("▸ %+v: tip %+v is not rightmost", closed.polys[0].pts, tip)
		}
	}
	if !closed.polys[0].fill || math.Abs(tip.Y-cy) > 1e-9 {
		t.Fatalf("▸ = %+v", closed.polys[0])
	}
	// ▾ points down.
	open, _ := iconShape(layout.IconDisclosureOpen, ix, iy, iw, ih, icol)
	low := open.polys[0].pts[2]
	for _, p := range open.polys[0].pts {
		if p.Y > low.Y {
			t.Fatalf("▾ %+v: tip %+v is not lowest", open.polys[0].pts, low)
		}
	}
	// The chevron is a stroked, open shape pointing right.
	chev, _ := iconShape(layout.IconChevronRight, ix, iy, iw, ih, icol)
	if p := chev.polys[0]; p.fill || len(p.pts) != 3 || !(p.pts[1].X > p.pts[0].X) || !(p.width > 1) {
		t.Fatalf("chevron = %+v", p)
	}
}
