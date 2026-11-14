package layout

import (
	"image"
	"image/color"
	"math"
	"sync"
)

// A terminal that shows images draws an icon button with room for more
// than one line as its icon's shape, as a GUI does, instead of the
// one-cell glyph. The shapes are the GUI's, in a unit box centered on the
// origin (x right, y down).

type iconPoint struct{ x, y float64 }

// iconStroke is a filled closed polygon, or an open polyline width wide with
// round caps and joins.
type iconStroke struct {
	pts   []iconPoint
	fill  bool
	width float64
}

// iconCircle is a filled disc, or a ring width wide, centered on c.
type iconCircle struct {
	c     iconPoint
	r     float64
	fill  bool
	width float64
}

type iconGeometry struct {
	strokes []iconStroke
	circles []iconCircle
}

// iconStrokeWidth is the unit box's line width, as a GUI draws a large icon.
const iconStrokeWidth = 1.0 / 9

var iconGeometries sync.Map // IconName → iconGeometry, or nil when the name has no shape

// vectorIconGeometry is name's shape, or false for names drawn as text.
func vectorIconGeometry(name IconName) (iconGeometry, bool) {
	if g, ok := iconGeometries.Load(name); ok {
		geo, has := g.(*iconGeometry)
		if !has || geo == nil {
			return iconGeometry{}, false
		}
		return *geo, true
	}
	geo, ok := buildIconGeometry(name)
	if !ok {
		iconGeometries.Store(name, (*iconGeometry)(nil))
		return iconGeometry{}, false
	}
	iconGeometries.Store(name, &geo)
	return geo, true
}

func buildIconGeometry(name IconName) (iconGeometry, bool) {
	at := func(a, b float64) iconPoint { return iconPoint{a, b} }
	sw := iconStrokeWidth
	line := func(pts ...iconPoint) iconStroke { return iconStroke{pts: pts, width: sw} }
	ring := func(c iconPoint, r float64) iconCircle { return iconCircle{c: c, r: r - sw/2, width: sw} }
	dot := func(c iconPoint, r float64) iconCircle { return iconCircle{c: c, r: r, fill: true} }
	box := func(a, b iconPoint) iconStroke {
		return iconStroke{pts: []iconPoint{a, {b.x, a.y}, b, {a.x, b.y}}, fill: true}
	}
	cross := func(d float64) []iconStroke {
		return []iconStroke{line(at(-d, -d), at(d, d)), line(at(d, -d), at(-d, d))}
	}
	turn := func(pts []iconPoint, turns int) []iconPoint {
		out := make([]iconPoint, len(pts))
		for i, p := range pts {
			for range turns % 4 {
				p = iconPoint{-p.y, p.x}
			}
			out[i] = p
		}
		return out
	}
	arrow := []iconPoint{at(-0.4, 0), at(0.4, 0)}
	head := []iconPoint{at(0.08, -0.32), at(0.4, 0), at(0.08, 0.32)}
	switch name {
	case IconCheck:
		return iconGeometry{strokes: []iconStroke{line(at(-0.4, 0.02), at(-0.12, 0.32), at(0.42, -0.32))}}, true
	case IconClose:
		return iconGeometry{strokes: cross(0.32)}, true
	case IconPlus:
		return iconGeometry{strokes: []iconStroke{line(at(-0.38, 0), at(0.38, 0)), line(at(0, -0.38), at(0, 0.38))}}, true
	case IconMinus:
		return iconGeometry{strokes: []iconStroke{line(at(-0.38, 0), at(0.38, 0))}}, true
	case IconPlay:
		return iconGeometry{strokes: []iconStroke{{pts: []iconPoint{at(-0.32, -0.4), at(0.4, 0), at(-0.32, 0.4)}, fill: true}}}, true
	case IconPause:
		return iconGeometry{strokes: []iconStroke{box(at(-0.32, -0.38), at(-0.08, 0.38)), box(at(0.08, -0.38), at(0.32, 0.38))}}, true
	case IconNext, IconPrevious:
		g := iconGeometry{strokes: []iconStroke{
			{pts: []iconPoint{at(-0.36, -0.36), at(0.18, 0), at(-0.36, 0.36)}, fill: true},
			box(at(0.22, -0.36), at(0.36, 0.36)),
		}}
		if name == IconPrevious {
			for i, s := range g.strokes {
				pts := make([]iconPoint, len(s.pts))
				for j, p := range s.pts {
					pts[j] = iconPoint{-p.x, p.y}
				}
				g.strokes[i].pts = pts
			}
		}
		return g, true
	case IconShuffle:
		return iconGeometry{strokes: []iconStroke{
			line(at(-0.44, 0.28), at(-0.16, 0.28), at(0.16, -0.28), at(0.4, -0.28)),
			line(at(-0.44, -0.28), at(-0.16, -0.28), at(0.16, 0.28), at(0.4, 0.28)),
			line(at(0.26, -0.42), at(0.42, -0.28), at(0.26, -0.14)),
			line(at(0.26, 0.14), at(0.42, 0.28), at(0.26, 0.42)),
		}}, true
	case IconRepeat, IconRepeatOne:
		g := iconGeometry{strokes: []iconStroke{
			line(at(-0.42, 0.06), at(-0.42, -0.26), at(0.36, -0.26)),
			line(at(0.22, -0.4), at(0.38, -0.26), at(0.22, -0.12)),
			line(at(0.42, -0.06), at(0.42, 0.26), at(-0.36, 0.26)),
			line(at(-0.22, 0.12), at(-0.38, 0.26), at(-0.22, 0.4)),
		}}
		if name == IconRepeatOne {
			g.strokes = append(g.strokes, line(at(-0.12, -0.08), at(0.04, -0.21), at(0.04, 0.21)))
		}
		return g, true
	case IconSpeaker:
		g := iconGeometry{strokes: []iconStroke{{pts: []iconPoint{at(-0.44, -0.14), at(-0.24, -0.14), at(-0.02, -0.36), at(-0.02, 0.36), at(-0.24, 0.14), at(-0.44, 0.14)}, fill: true}}}
		for _, r := range []float64{0.2, 0.36} {
			var pts []iconPoint
			for k := -3; k <= 3; k++ {
				a := float64(k) * math.Pi / 9 // ±60°
				pts = append(pts, at(0.02+r*math.Cos(a), r*math.Sin(a)))
			}
			g.strokes = append(g.strokes, line(pts...))
		}
		return g, true
	case IconHome:
		return iconGeometry{strokes: []iconStroke{line(at(-0.32, 0.4), at(-0.32, -0.06), at(0, -0.36), at(0.32, -0.06), at(0.32, 0.4), at(-0.32, 0.4))}}, true
	case IconHeart, IconHeartOutline:
		var pts []iconPoint
		for k := range 48 {
			t := float64(k) / 48 * 2 * math.Pi
			x := 16 * math.Pow(math.Sin(t), 3)
			y := 13*math.Cos(t) - 5*math.Cos(2*t) - 2*math.Cos(3*t) - math.Cos(4*t)
			pts = append(pts, at(x/34, -(y+2.5)/34))
		}
		if name == IconHeart {
			return iconGeometry{strokes: []iconStroke{{pts: pts, fill: true}}}, true
		}
		return iconGeometry{strokes: []iconStroke{line(append(pts, pts[0])...)}}, true
	case IconClock:
		return iconGeometry{
			circles: []iconCircle{ring(at(0, 0), 0.46)},
			strokes: []iconStroke{line(at(0, -0.26), at(0, 0), at(0.18, 0.12))},
		}, true
	case IconEqualizer:
		return iconGeometry{strokes: []iconStroke{
			box(at(-0.36, 0.06), at(-0.16, 0.4)),
			box(at(-0.1, -0.4), at(0.1, 0.4)),
			box(at(0.16, -0.16), at(0.36, 0.4)),
		}}, true
	case IconLibrary:
		return iconGeometry{strokes: []iconStroke{
			line(at(-0.3, -0.38), at(-0.3, 0.38)),
			line(at(-0.08, -0.38), at(-0.08, 0.38)),
			line(at(0.1, -0.34), at(0.34, 0.36)),
		}}, true
	case IconSearch:
		return iconGeometry{
			circles: []iconCircle{ring(at(-0.08, -0.08), 0.32)},
			strokes: []iconStroke{{pts: []iconPoint{at(0.16, 0.16), at(0.42, 0.42)}, width: sw * 1.4}},
		}, true
	case IconGear:
		g := iconGeometry{circles: []iconCircle{{c: at(0, 0), r: 0.24, width: 0.16}}}
		for k := range 8 {
			a := float64(k) * math.Pi / 4
			c, s := math.Cos(a), math.Sin(a)
			// Square-ended teeth: a quad 0.17 wide along the spoke.
			n := iconPoint{-s * 0.085, c * 0.085}
			in, out := at(0.28*c, 0.28*s), at(0.47*c, 0.47*s)
			g.strokes = append(g.strokes, iconStroke{fill: true, pts: []iconPoint{
				{in.x + n.x, in.y + n.y}, {out.x + n.x, out.y + n.y}, {out.x - n.x, out.y - n.y}, {in.x - n.x, in.y - n.y},
			}})
		}
		return g, true
	case IconInfo:
		return iconGeometry{
			circles: []iconCircle{ring(at(0, 0), 0.5), dot(at(0, -0.2), 0.07)},
			strokes: []iconStroke{line(at(0, -0.04), at(0, 0.26))},
		}, true
	case IconWarning:
		return iconGeometry{
			strokes: []iconStroke{line(at(0, -0.44), at(0.48, 0.4), at(-0.48, 0.4), at(0, -0.44)), line(at(0, -0.14), at(0, 0.1))},
			circles: []iconCircle{dot(at(0, 0.24), 0.07)},
		}, true
	case IconError:
		return iconGeometry{circles: []iconCircle{ring(at(0, 0), 0.5)}, strokes: cross(0.17)}, true
	case IconArrowRight, IconArrowDown, IconArrowLeft, IconArrowUp:
		turns := map[IconName]int{IconArrowDown: 1, IconArrowLeft: 2, IconArrowUp: 3}[name]
		return iconGeometry{strokes: []iconStroke{line(turn(arrow, turns)...), line(turn(head, turns)...)}}, true
	case IconEllipsis:
		return iconGeometry{circles: []iconCircle{dot(at(-0.32, 0), 0.07), dot(at(0, 0), 0.07), dot(at(0.32, 0), 0.07)}}, true
	case IconFolder:
		return iconGeometry{strokes: []iconStroke{line(at(-0.45, -0.3), at(-0.14, -0.3), at(-0.04, -0.2), at(0.45, -0.2), at(0.45, 0.34), at(-0.45, 0.34), at(-0.45, -0.3))}}, true
	case IconFile:
		return iconGeometry{strokes: []iconStroke{
			line(at(-0.32, -0.45), at(0.1, -0.45), at(0.32, -0.23), at(0.32, 0.45), at(-0.32, 0.45), at(-0.32, -0.45)),
			line(at(0.1, -0.45), at(0.1, -0.23), at(0.32, -0.23)),
		}}, true
	}
	return iconGeometry{}, false
}

// covers reports whether the unit-box point p is inside the shape.
func (g iconGeometry) covers(p iconPoint) bool {
	for _, s := range g.strokes {
		if s.fill {
			if insidePolygon(s.pts, p) {
				return true
			}
			continue
		}
		half := max(s.width, iconStrokeWidth) / 2
		for i := 1; i < len(s.pts); i++ {
			if segmentDistance(p, s.pts[i-1], s.pts[i]) <= half {
				return true
			}
		}
	}
	for _, c := range g.circles {
		d := math.Hypot(p.x-c.c.x, p.y-c.c.y)
		if c.fill && d <= c.r || !c.fill && math.Abs(d-c.r) <= max(c.width, iconStrokeWidth)/2 {
			return true
		}
	}
	return false
}

// coverage is the share of the unit-box square [x0,x1)×[y0,y1) inside the
// shape, sampled n×n.
func (g iconGeometry) coverage(x0, y0, x1, y1 float64, n int) float64 {
	hit := 0
	for j := range n {
		for i := range n {
			p := iconPoint{x0 + (float64(i)+0.5)*(x1-x0)/float64(n), y0 + (float64(j)+0.5)*(y1-y0)/float64(n)}
			if g.covers(p) {
				hit++
			}
		}
	}
	return float64(hit) / float64(n*n)
}

func insidePolygon(pts []iconPoint, p iconPoint) bool {
	in := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a.y > p.y) != (b.y > p.y) && p.x < (b.x-a.x)*(p.y-a.y)/(b.y-a.y)+a.x {
			in = !in
		}
	}
	return in
}

func segmentDistance(p, a, b iconPoint) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = min(max(((p.x-a.x)*dx+(p.y-a.y)*dy)/l, 0), 1)
	}
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

// vectorIconScale is the share of its square an icon's shape spans: a
// margin all round, as a GUI draws a button's icon well inside its bezel.
const vectorIconScale = 0.6

// vectorIconSide is a vectorIcon's side in pixels: more than a terminal
// prepares an icon button's picture at, so the picture is never upscaled.
const vectorIconSide = 512

// vectorIcon is an icon's shape as a square image in one color: what a
// terminal that shows images draws for an icon button. Each pixel is
// antialiased, so a terminal can scale it down smoothly.
type vectorIcon struct {
	Name  IconName
	Color color.RGBA
	// Invert fills the square with Color and leaves the shape dark: a
	// focused button, as the terminal reverses a focused label.
	Invert bool
}

func (v vectorIcon) ColorModel() color.Model { return color.NRGBAModel }

func (v vectorIcon) Bounds() image.Rectangle {
	return image.Rect(0, 0, vectorIconSide, vectorIconSide)
}

func (v vectorIcon) At(x, y int) color.Color {
	g, ok := vectorIconGeometry(v.Name)
	if !ok || x < 0 || y < 0 || x >= vectorIconSide || y >= vectorIconSide {
		return color.NRGBA{}
	}
	const px = 1.0 / vectorIconSide / vectorIconScale
	u, w := (float64(x)/vectorIconSide-0.5)/vectorIconScale, (float64(y)/vectorIconSide-0.5)/vectorIconScale
	a := g.coverage(u, w, u+px, w+px, 2)
	if v.Invert {
		return blendInverted(v.Color, a)
	}
	return color.NRGBA{R: v.Color.R, G: v.Color.G, B: v.Color.B, A: uint8(math.Round(a * 255))}
}

// iconInk is the shape's color in an inverted icon.
var iconInk = color.RGBA{R: 0x1c, G: 0x1c, B: 0x1c, A: 0xff}

// blendInverted is c with the share a of iconInk over it.
func blendInverted(c color.RGBA, a float64) color.NRGBA {
	mix := func(from, to uint8) uint8 { return uint8(math.Round(float64(from)*(1-a) + float64(to)*a)) }
	return color.NRGBA{R: mix(c.R, iconInk.R), G: mix(c.G, iconInk.G), B: mix(c.B, iconInk.B), A: 0xff}
}
