package window

import (
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

var symbols = []layout.IconName{
	layout.IconCheck, layout.IconClose, layout.IconPlus, layout.IconMinus, layout.IconSearch,
	layout.IconGear, layout.IconInfo, layout.IconWarning, layout.IconError,
	layout.IconArrowUp, layout.IconArrowDown, layout.IconArrowLeft, layout.IconArrowRight,
	layout.IconChevronLeft, layout.IconChevronUp, layout.IconChevronDown,
	layout.IconEllipsis, layout.IconFolder, layout.IconFile,
}

// extent is the box g's points and circles span.
func extent(g cellShape) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = 1e9, 1e9, -1e9, -1e9
	add := func(x, y float64) {
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	for _, p := range g.polys {
		for _, pt := range p.pts {
			add(pt.X, pt.Y)
		}
	}
	for _, c := range g.circles {
		add(c.c.X-c.r, c.c.Y-c.r)
		add(c.c.X+c.r, c.c.Y+c.r)
	}
	return
}

func TestIconSymbolsFitTheirBox(t *testing.T) {
	const x, y, w, h, col = 100.0, 50.0, 16.0, 17.0, 8.0
	for _, name := range symbols {
		g, ok := iconShape(name, x, y, w, h, col)
		if !ok || len(g.polys)+len(g.circles) == 0 {
			t.Errorf("icon %d: no shape", name)
			continue
		}
		x0, y0, x1, y1 := extent(g)
		if x0 < x || y0 < y || x1 > x+w || y1 > y+h {
			t.Errorf("icon %d spans (%v,%v)-(%v,%v), outside the %vx%v box at (%v,%v)", name, x0, y0, x1, y1, w, h, x, y)
		}
		if x1-x0 < 3 || y1-y0 < 0.1 && name != layout.IconMinus && name != layout.IconEllipsis {
			t.Errorf("icon %d is tiny: %vx%v", name, x1-x0, y1-y0)
		}
	}
	// Narrow boxes shrink the symbol to fit.
	g, _ := iconShape(layout.IconGear, x, y, 6, h, col)
	if x0, _, x1, _ := extent(g); x0 < x || x1 > x+6 {
		t.Errorf("gear in 6pt spans %v..%v", x0, x1)
	}
	if _, ok := iconShape(layout.IconName(999), x, y, w, h, col); ok {
		t.Error("an unknown name has a shape")
	}
}

func TestIconDirections(t *testing.T) {
	const x, y, w, h, col = 0.0, 0.0, 16.0, 16.0, 8.0
	// tip is where an arrow's head (or a chevron) points: its middle point.
	tip := func(name layout.IconName) point {
		g, _ := iconShape(name, x, y, w, h, col)
		p := g.polys[len(g.polys)-1]
		return p.pts[1]
	}
	for _, c := range []struct {
		name   layout.IconName
		dx, dy int
	}{
		{layout.IconArrowRight, 1, 0}, {layout.IconArrowLeft, -1, 0}, {layout.IconArrowUp, 0, -1}, {layout.IconArrowDown, 0, 1},
	} {
		p := tip(c.name)
		if sign(p.X-w/2) != c.dx || sign(p.Y-h/2) != c.dy {
			t.Errorf("arrow %d tip at %+v, want direction (%d,%d)", c.name, p, c.dx, c.dy)
		}
	}
	for _, c := range []struct {
		name   layout.IconName
		dx, dy int
	}{
		{layout.IconChevronRight, 1, 0}, {layout.IconChevronLeft, -1, 0}, {layout.IconChevronUp, 0, -1}, {layout.IconChevronDown, 0, 1},
	} {
		p := tip(c.name)
		if sign(p.X-col/2) != c.dx || sign(p.Y-h/2) != c.dy {
			t.Errorf("chevron %d tip at %+v, want direction (%d,%d)", c.name, p, c.dx, c.dy)
		}
	}
}

func sign(v float64) int {
	switch {
	case v > 0.01:
		return 1
	case v < -0.01:
		return -1
	}
	return 0
}
