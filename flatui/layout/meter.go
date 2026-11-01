package layout

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Meter is a horizontal gauge filled to Value (0..1, clamped; NaN is empty).
// The terminal draws round(Value×width) cells of █ in Fill and the rest as
// ░ in Track; a GUI draws a native bar. Nil colors are the backend's
// default. On Auto axes it fills the available width and is one line tall.
type Meter struct {
	NodeBase
	Value       float64
	Fill, Track color.Color
}

func (m Meter) Size() (Size, Size) {
	w, h := m.NodeBase.Size()
	if w.Kind == SizeAuto {
		w = Grow(1)
	}
	if h.Kind == SizeAuto {
		h = Size{Kind: SizeContent, Value: satAdd(1, m.insetSides().vert())}
	}
	return w, h
}

func (m Meter) Render(r Rect) string {
	inner := r.insetBy(m.insetSides())
	if inner.W <= 0 {
		return fillBox("", r, m.padSides(), m.box())
	}
	filled := int(math.Round(m.value() * float64(inner.W)))
	bar := tinted(strings.Repeat("█", filled), m.Fill) + tinted(strings.Repeat("░", inner.W-filled), m.Track)
	return fillBox(bar, r, m.padSides(), m.box())
}

// value is Value clamped to 0..1, with NaN as 0.
func (m Meter) value() float64 {
	if math.IsNaN(m.Value) {
		return 0
	}
	return min(max(m.Value, 0), 1)
}

// tinted styles s in fg, leaving it plain when fg is nil or s is empty.
func tinted(s string, fg color.Color) string {
	if s == "" || fg == nil {
		return s
	}
	return lipgloss.NewStyle().Foreground(fg).Render(s)
}
