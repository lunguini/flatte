package layout

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Text is a leaf carrying a pre-rendered string. Auto-sizes to the
// string's dimensions unless explicit W/H is set on the embedded NodeBase.
type Text struct {
	NodeBase
	String string
}

func (t Text) Size() (Size, Size) {
	w, h := t.NodeBase.Size()
	if w.Kind == SizeAuto || h.Kind == SizeAuto {
		mw, mh := measureString(t.String)
		inset := 2 * t.innerInset()
		if w.Kind == SizeAuto {
			w = Size{Kind: SizeContent, Value: mw + inset}
		}
		if h.Kind == SizeAuto {
			h = Size{Kind: SizeContent, Value: mh + inset}
		}
	}
	return w, h
}

func (t Text) Render(r Rect) string {
	return fillRect(t.String, r, t.Pad, t.Bordered)
}

// Spacer fills remaining space. Grows on both axes.
type Spacer struct {
	NodeBase
}

func NewSpacer() Spacer {
	return Spacer{NodeBase: NodeBase{W: Grow(1), H: Grow(1)}}
}

func (s Spacer) Render(r Rect) string {
	return fillRect("", r, 0, false)
}

// --- helpers ---

func measureString(s string) (int, int) {
	lines := strings.Split(s, "\n")
	h := len(lines)
	w := 0
	for _, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			w = lw
		}
	}
	return w, h
}

func nonOverlayChildren(children []Node) []Node {
	var out []Node
	for _, c := range children {
		if o, ok := c.(interface{ IsOverlay() bool }); !ok || !o.IsOverlay() {
			out = append(out, c)
		}
	}
	return out
}
