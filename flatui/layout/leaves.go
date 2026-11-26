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
//
// Carets are further carets for multi-cursor editing: a GUI draws each as
// the same blinking bar. A terminal has one hardware cursor (Caret) and
// ignores them; draw their cells yourself there (RenderContext.Graphical
// tells the hosts apart).
//
// OffsetY is smooth scrolling: a GUI draws the text OffsetY lines higher
// (0 ≤ OffsetY < 1; outside that, 0), clipped to the node, with one more
// line of String showing below — the fraction of a line a scroll is part
// way through. Give String one line more than the node holds so the bottom
// stays filled. Carets move with the text. A terminal draws whole lines and
// ignores it.
//
// Face is the type a GUI sets the grid in (FaceCode by default): its
// columns and lines are that face's cells (Metrics.Cell), so an app sizes
// the content from the node's point rect over the face's cell
// (LayoutEvent.Faces). A terminal ignores it.
type Text struct {
	NodeBase
	String  string
	Caret   *Caret
	Carets  []Caret
	OffsetY float64
	Face    Face
}

// shift is the fraction of a line a GUI draws t higher.
func (t Text) shift() float64 {
	if t.OffsetY > 0 && t.OffsetY < 1 {
		return t.OffsetY
	}
	return 0
}

// Caret is a cell in a Text's content: column X of line Y, from 0.
type Caret struct{ X, Y int }

// caret places t's caret in r, t's rect; ok is false without a caret or
// when it falls outside the content.
func (t Text) caret(r Rect) (x, y int, ok bool) {
	if t.Caret == nil {
		return 0, 0, false
	}
	return t.place(*t.Caret, r)
}

// place puts content cell c in r, t's rect; ok is false outside the content.
func (t Text) place(c Caret, r Rect) (x, y int, ok bool) {
	in := r.insetBy(t.insetSides())
	if c.X < 0 || c.Y < 0 || c.X >= in.W || c.Y >= in.H {
		return 0, 0, false
	}
	return satAdd(in.X, c.X), satAdd(in.Y, c.Y), true
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
