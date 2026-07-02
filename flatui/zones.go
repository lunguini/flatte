package flatui

import "github.com/lunguini/flatte/flatui/layout"

// Rect is a cell-space rectangle: the same type the layout engine solves, so
// hit regions and solved geometry share one representation. X/Y are zero-based
// absolute frame coordinates; W/H must be positive to contain points.
type Rect = layout.Rect

// ZoneMap stores app-measured hit regions. It does not scan rendered output or
// own input policy; apps register rectangles from their layout calculations.
type ZoneMap struct {
	order []string
	rects map[string]Rect
}

// Clear removes every registered zone.
func (z *ZoneMap) Clear() {
	z.order = z.order[:0]
	clear(z.rects)
}

// Set registers or replaces a zone rectangle. Replacing an existing zone moves
// it to the front of hit priority.
func (z *ZoneMap) Set(id string, rect Rect) {
	if id == "" {
		return
	}
	z.ensure()
	z.removeOrder(id)
	z.order = append(z.order, id)
	z.rects[id] = rect
}

// Rect returns the rectangle registered for id.
func (z ZoneMap) Rect(id string) (Rect, bool) {
	rect, ok := z.rects[id]
	return rect, ok
}

// In reports whether x,y falls inside id's registered rectangle.
func (z ZoneMap) In(id string, x, y int) bool {
	rect, ok := z.Rect(id)
	return ok && rect.Contains(x, y)
}

// At returns the topmost registered zone at x,y. Later Set calls take priority
// when rectangles overlap.
func (z ZoneMap) At(x, y int) (string, bool) {
	for i := len(z.order) - 1; i >= 0; i-- {
		id := z.order[i]
		if z.rects[id].Contains(x, y) {
			return id, true
		}
	}
	return "", false
}

// Local converts absolute x,y to coordinates relative to id's registered
// rectangle.
func (z ZoneMap) Local(id string, x, y int) (localX, localY int, ok bool) {
	rect, ok := z.Rect(id)
	if !ok || !rect.Contains(x, y) {
		return 0, 0, false
	}
	return x - rect.X, y - rect.Y, true
}

func (z *ZoneMap) ensure() {
	if z.rects == nil {
		z.rects = make(map[string]Rect)
	}
}

func (z *ZoneMap) removeOrder(id string) {
	for i, existing := range z.order {
		if existing == id {
			copy(z.order[i:], z.order[i+1:])
			z.order = z.order[:len(z.order)-1]
			return
		}
	}
}
