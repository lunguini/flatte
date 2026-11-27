package window

import (
	"math"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// Control sizes in points.
const (
	checkSize   = 14.0
	switchW     = 28.0
	switchH     = 16.0
	knobSize    = 14.0
	sliderTrack = 4.0
)

var white = rgba{1, 1, 1, 1}

// control draws a ControlOp: hover and press are the host's (the painter's
// hovered and pressed IDs), drawn on enabled controls only.
func (p *painter) control(op layout.ControlOp) {
	hot := op.ID != "" && op.ID == p.hovered && !op.Disabled
	down := hot && op.ID == p.pressed
	switch op.Kind {
	case layout.ControlButton:
		p.button(op, p.levelOf(p.hovers, op.ID, hot, op.Disabled), p.levelOf(p.presses, op.ID, down, op.Disabled))
	case layout.ControlSlider:
		p.slider(op, hot, down)
	default:
		p.toggle(op, hot, down)
	}
}

// levelOf is id's fading level in levels — or, when the host keeps none
// (a snapshot), on as given — for an enabled control.
func (p *painter) levelOf(levels map[string]float64, id string, on, disabled bool) float64 {
	switch {
	case disabled || id == "":
		return 0
	case levels != nil:
		return levels[id]
	case on:
		return 1
	}
	return 0
}

// shadeBy is a control fill under hover and press levels (0..1).
func (p *painter) shadeBy(c rgba, hot, down float64) rgba {
	return mix(mix(c, white, 0.1*hot*(1-down)), p.th.bg, 0.25*down) // a press replaces the hover's lift
}

// shade is a control fill under hover and press.
func (p *painter) shade(c rgba, hot, down bool) rgba {
	switch {
	case down:
		return mix(c, p.th.bg, 0.25)
	case hot:
		return mix(c, white, 0.1)
	}
	return c
}

// dim fades a disabled control's color into the background.
func (p *painter) dim(c rgba, disabled bool) rgba {
	if disabled {
		return mix(c, p.th.bg, 0.55)
	}
	return c
}

// ring strokes a focus ring around r: the theme's focus color and width,
// a point clear of the control.
func (p *painter) ring(r CGRect, rad float64) {
	p.stroke(p.th.lk.focus)
	w := p.th.lk.focusWidth
	cgContextSetLineWidth(p.ctx, w)
	o := w / 2
	p.roundedPath(cgRect(r.Origin.X-o-1, r.Origin.Y-o-1, r.Size.Width+2*o+2, r.Size.Height+2*o+2), rad+o+1, func() { cgContextStrokePath(p.ctx) })
}

// controlBezel shares the visual inset of regular native buttons and
// one-line fields; their full layout rectangles remain the hit targets.
func controlBezel(r layout.PointRect) CGRect {
	return cgRect(r.X+0.5, r.Y+1.5, max(r.W-1, 0), max(r.H-3, 0))
}

// button draws the design's button recipe: Default is primary (accent
// fill, AccentHover under the pointer), Plain is quiet (no bezel until
// hover), anything else secondary (outlined); hover and press are the
// theme's washes, focus a ring.
func (p *painter) button(op layout.ControlOp, hot, down float64) {
	r := op.Rect
	lk := p.th.lk
	bez := controlBezel(r)
	rad := min(lk.radius, bez.Size.Height/2)
	icon := op.Icon != layout.IconNone
	if icon && math.Abs(r.W-r.H) < 1 {
		// A square icon button: an inset square, a circle when Default.
		bez = cgRect(r.X+1, r.Y+1, max(r.W-2, 0), max(r.H-2, 0))
		rad = min(lk.radius, bez.Size.Height/2)
		if op.Default {
			rad = bez.Size.Height / 2
		}
	}
	text := p.roleColor(op.Style)
	path := func(f func()) { p.roundedPath(bez, rad, f) }
	if op.Default {
		fill := mix(lk.accent, lk.accentHot, hot)
		text = lk.onAccent
		if op.Style.Bg != nil { // the app's own accent for this button
			fill = p.shadeBy(rgbaOf(op.Style.Bg), hot, 0)
		}
		if op.Style.Fg != nil {
			text = rgbaOf(op.Style.Fg)
		}
		fill = lk.wash(down, down).over(fill)
		p.fill(p.dim(fill, op.Disabled))
		path(func() { cgContextFillPath(p.ctx) })
	} else {
		if w := lk.wash(hot, down); w.A > 0 {
			p.fill(w)
			path(func() { cgContextFillPath(p.ctx) })
		}
		if !op.Plain {
			p.stroke(p.dim(lk.border, op.Disabled))
			cgContextSetLineWidth(p.ctx, lk.control)
			path(func() { cgContextStrokePath(p.ctx) })
		}
	}
	if op.Plain && !op.Disabled && op.Style.Fg == nil {
		text = mix(text, p.th.title, hot) // a hovered toolbar button brightens
	}
	if icon {
		// The icon fills half the button's shorter side, centered.
		size := min(r.W, r.H) * 0.5
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		g, ok := symbol(op.Icon, cx, cy, size)
		if !ok { // a line-glyph shape (chevrons, dots): centered the same
			g, ok = iconShape(op.Icon, cx-size/2, cy-size/2, size, size, size)
		}
		if ok {
			p.shape(g, p.buttonText(text, op))
		}
	} else {
		f := p.th.styleFont(op.Style)
		w := f.width(op.Label)
		content := op.Content
		if content == (layout.PointRect{}) {
			content = r
		}
		p.line(op.Label, f, content.X+(content.W-w)/2, content.Y, content.H, p.buttonText(text, op), nil, op.Style.Underline, op.Style.Strikethrough)
	}
	if op.Focused && !op.Disabled {
		p.ring(bez, rad)
	}
}

// buttonText is a button's label color: dimmed on a disabled primary,
// the theme's disabled text on other disabled buttons.
func (p *painter) buttonText(c rgba, op layout.ControlOp) rgba {
	switch {
	case !op.Disabled:
		return c
	case op.Default:
		return p.dim(c, true)
	}
	return p.th.lk.disabled
}

// indicatorCols is a toggle indicator's terminal width, which its native
// indicator is centered in and its label follows.
func indicatorCols(k layout.ControlKind) int {
	if k == layout.ControlSwitch {
		return ansi.StringWidth("[ ●]")
	}
	return ansi.StringWidth("[x]")
}

func (p *painter) toggle(op layout.ControlOp, hot, down bool) {
	r := op.Content
	if r == (layout.PointRect{}) {
		r = op.Rect
	}
	cols := indicatorCols(op.Kind)
	areaW := float64(cols) * p.m.Column
	cy := r.Y + r.H/2
	var ind CGRect
	var rad float64
	switch op.Kind {
	case layout.ControlSwitch:
		ind = cgRect(r.X+(areaW-switchW)/2, cy-switchH/2, switchW, switchH)
		rad = switchH / 2
		lk := p.th.lk
		track, thumb := lk.field, lk.knob
		if op.Checked {
			track, thumb = lk.accent, lk.onAccent
		}
		p.fill(p.dim(lk.wash(b2f(hot), b2f(down)).over(track), op.Disabled))
		p.roundedPath(ind, rad, func() { cgContextFillPath(p.ctx) })
		if !op.Checked {
			p.stroke(p.dim(lk.border, op.Disabled))
			cgContextSetLineWidth(p.ctx, lk.control)
			p.roundedPath(cgRect(ind.Origin.X+0.5, ind.Origin.Y+0.5, switchW-1, switchH-1), rad-0.5, func() { cgContextStrokePath(p.ctx) })
		}
		knob := switchH - 6
		kx := ind.Origin.X + 3
		if op.Checked {
			kx = ind.Origin.X + switchW - 3 - knob
		}
		p.fill(p.dim(thumb, op.Disabled))
		cgContextAddEllipseInRect(p.ctx, cgRect(kx, cy-knob/2, knob, knob))
		cgContextFillPath(p.ctx)
	case layout.ControlRadio:
		ind = cgRect(r.X+(areaW-checkSize)/2, cy-checkSize/2, checkSize, checkSize)
		rad = checkSize / 2
		p.indicatorBox(ind, rad, op, hot, down)
		if op.Checked {
			dot := 5.0
			p.fill(p.dim(p.th.lk.onAccent, op.Disabled))
			cgContextAddEllipseInRect(p.ctx, cgRect(ind.Origin.X+(checkSize-dot)/2, cy-dot/2, dot, dot))
			cgContextFillPath(p.ctx)
		}
	default:
		ind = cgRect(r.X+(areaW-checkSize)/2, cy-checkSize/2, checkSize, checkSize)
		rad = p.th.lk.check
		p.indicatorBox(ind, rad, op, hot, down)
		if op.Checked {
			x, y := ind.Origin.X, ind.Origin.Y
			p.stroke(p.dim(p.th.lk.onAccent, op.Disabled))
			cgContextSetLineWidth(p.ctx, 1.8)
			cgContextMoveToPoint(p.ctx, x+3.5, y+7.2)
			cgContextAddLineToPoint(p.ctx, x+6, y+9.8)
			cgContextAddLineToPoint(p.ctx, x+10.5, y+4.3)
			cgContextStrokePath(p.ctx)
		}
	}
	if op.Label != "" {
		f := p.th.styleFont(op.Style)
		x := r.X + float64(cols+1)*p.m.Column
		p.line(op.Label, f, x, r.Y, r.H, p.dim(p.roleColor(op.Style), op.Disabled), nil, op.Style.Underline, op.Style.Strikethrough)
	}
	if op.Focused && !op.Disabled {
		p.ring(ind, rad)
	}
}

// indicatorBox fills and strokes a checkbox or radio indicator: the accent
// when checked, a field with the control border when not, under the
// theme's hover and press washes.
func (p *painter) indicatorBox(r CGRect, rad float64, op layout.ControlOp, hot, down bool) {
	lk := p.th.lk
	fill := lk.field
	if op.Checked {
		fill = lk.accent
	}
	p.fill(p.dim(lk.wash(b2f(hot), b2f(down)).over(fill), op.Disabled))
	path := func(f func()) {
		if rad >= r.Size.Width/2 {
			cgContextAddEllipseInRect(p.ctx, r)
			f()
			return
		}
		p.roundedPath(r, rad, f)
	}
	path(func() { cgContextFillPath(p.ctx) })
	if !op.Checked {
		border := lk.border
		if hot {
			border = mix(border, lk.accent, 0.6)
		}
		p.stroke(p.dim(border, op.Disabled))
		cgContextSetLineWidth(p.ctx, lk.control)
		path(func() { cgContextStrokePath(p.ctx) })
	}
}

func (p *painter) slider(op layout.ControlOp, hot, down bool) {
	r := op.Rect
	cy := r.Y + r.H/2
	x0, x1 := r.X+knobSize/2, r.X+r.W-knobSize/2
	kx := x0 + op.Value*max(x1-x0, 0)
	fill := p.th.caret
	if op.Fill != nil {
		fill = rgbaOf(op.Fill)
	}
	p.fill(p.dim(p.th.track, op.Disabled))
	p.roundedPath(cgRect(x0, cy-sliderTrack/2, max(x1-x0, 0), sliderTrack), sliderTrack/2, func() { cgContextFillPath(p.ctx) })
	p.fill(p.dim(fill, op.Disabled))
	p.roundedPath(cgRect(x0, cy-sliderTrack/2, max(kx-x0, 0), sliderTrack), sliderTrack/2, func() { cgContextFillPath(p.ctx) })
	knob := cgRect(kx-knobSize/2, cy-knobSize/2, knobSize, knobSize)
	p.fill(p.dim(p.shade(mix(p.th.title, p.th.bg, 0.05), hot, down), op.Disabled))
	cgContextAddEllipseInRect(p.ctx, knob)
	cgContextFillPath(p.ctx)
	p.stroke(p.dim(p.th.border, op.Disabled))
	cgContextSetLineWidth(p.ctx, strokeWidth)
	cgContextAddEllipseInRect(p.ctx, knob)
	cgContextStrokePath(p.ctx)
	if op.Focused && !op.Disabled {
		p.ring(knob, knobSize/2)
	}
}
