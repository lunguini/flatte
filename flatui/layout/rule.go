package layout

import "strings"

// Rule is a horizontal line across its width: ─ in a terminal, a hairline
// through the middle of its height in a window, a wash over whatever it is
// on. It is one line high and grows in width.
type Rule struct{ NodeBase }

func (r Rule) Size() (Size, Size) {
	w, h := r.W, r.H
	if w.Kind == SizeAuto {
		w = Grow(1)
	}
	if h.Kind == SizeAuto {
		h = Fixed(1)
	}
	return w, h
}

func (r Rule) Render(rc Rect) string {
	inner := rc.insetBy(getPadSides(r))
	if inner.W <= 0 || inner.H <= 0 {
		return fillBox("", rc, r.padSides(), r.box())
	}
	line := TextStyle{Color: ColorDisabled}.Render(strings.Repeat("─", inner.W))
	return fillBox(strings.Repeat("\n", inner.H/2)+line, rc, r.padSides(), r.box())
}
