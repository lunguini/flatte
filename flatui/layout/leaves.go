package layout

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Text is a leaf carrying a pre-rendered string. Auto-sizes to the
// string's dimensions unless explicit W/H is set on the embedded NodeBase.
//
// Caret, when set, is a text caret at a cell of the content (inside the
// node's padding), for apps that draw editable text as a cell grid — a code
// editor or a terminal emulator. A GUI draws it as its blinking caret bar at
// the cell's left edge; a terminal places the hardware cursor there
// (Frame.CursorStyle shapes it). A caret outside the content, or clipped
// away, shows nowhere. Like a focused Input's, the last visible caret in
// paint order wins.
type Text struct {
	NodeBase
	String string
	Caret  *Caret
}

// Caret is a cell in a Text's content: column X of line Y, from 0.
type Caret struct{ X, Y int }

// caret places t's caret in r, t's rect; ok is false without a caret or
// when it falls outside the content.
func (t Text) caret(r Rect) (x, y int, ok bool) {
	if t.Caret == nil {
		return 0, 0, false
	}
	in := r.insetBy(t.insetSides())
	if t.Caret.X < 0 || t.Caret.Y < 0 || t.Caret.X >= in.W || t.Caret.Y >= in.H {
		return 0, 0, false
	}
	return satAdd(in.X, t.Caret.X), satAdd(in.Y, t.Caret.Y), true
}

func (t Text) Size() (Size, Size) {
	w, h := t.NodeBase.Size()
	if w.Kind == SizeAuto || h.Kind == SizeAuto {
		mw, mh := measureString(t.String)
		inset := t.insetSides()
		if w.Kind == SizeAuto {
			w = Size{Kind: SizeContent, Value: satAdd(mw, inset.horiz())}
		}
		if h.Kind == SizeAuto {
			h = Size{Kind: SizeContent, Value: satAdd(mh, inset.vert())}
		}
	}
	return w, h
}

func (t Text) Render(r Rect) string {
	return fillBox(t.String, r, t.padSides(), t.box())
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

// Render fills r with the spacer's background: the WithBackground color, or
// else Box.Background. A spacer has no border or padding of its own.
func (s *Spacer) Render(r Rect) string {
	bg := s.bg
	if bg == nil {
		bg = s.Box.Background
	}
	return fillBox("", r, sides{}, Box{Background: bg})
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
