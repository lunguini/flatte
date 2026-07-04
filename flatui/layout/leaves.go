package layout

import (
	"image/color"
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
		inset := t.insetSides()
		if w.Kind == SizeAuto {
			w = Size{Kind: SizeContent, Value: mw + inset.horiz()}
		}
		if h.Kind == SizeAuto {
			h = Size{Kind: SizeContent, Value: mh + inset.vert()}
		}
	}
	return w, h
}

func (t Text) Render(r Rect) string {
	return fillRect(t.String, r, t.padSides(), t.Bordered)
}

// Spacer fills remaining space. Grows on both axes.
type Spacer struct {
	NodeBase
	bg color.Color
}

func NewSpacer() *Spacer {
	return &Spacer{
		NodeBase: NodeBase{
			W: Grow(1),
			H: Grow(1),
		},
	}
}

func (s *Spacer) WithBackground(c color.Color) *Spacer {
	s.bg = c
	return s
}

func (s *Spacer) Render(r Rect) string {
	return fillRectWithBg("", r, sides{}, s.bg, false)
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
