package layout

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TextRole is what a piece of text is for. Backends map roles to their own
// look — the terminal to colors and emphasis, a GUI to font size and weight —
// so neither bakes the other's look into the app. The set is deliberately
// small; unknown roles read as TextBody.
type TextRole int

const (
	TextBody TextRole = iota
	TextTitle
	TextSubtle
)

// TextStyle is a role plus optional overrides applied on top of it. Nil
// colors and false flags leave the role's look unchanged.
type TextStyle struct {
	Role   TextRole
	Fg, Bg color.Color

	Bold, Italic, Underline, Strikethrough bool
}

// Render returns text styled for a terminal. It is the single definition of
// each role's terminal look.
func (s TextStyle) Render(text string) string {
	return s.lipgloss().Render(text)
}

func (s TextStyle) lipgloss() lipgloss.Style {
	st := lipgloss.NewStyle()
	switch s.Role {
	case TextTitle:
		st = st.Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("235")).Padding(0, 1)
	case TextSubtle:
		st = st.Foreground(lipgloss.Color("244"))
	}
	if s.Fg != nil {
		st = st.Foreground(s.Fg)
	}
	if s.Bg != nil {
		st = st.Background(s.Bg)
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Strikethrough {
		st = st.Strikethrough(true)
	}
	return st
}

// Label is a leaf of plain text with a semantic style. Text holds no escape
// sequences (use Text for pre-styled strings). Like Text, it auto-sizes to
// its content on Auto axes, and lines break only at "\n" — unless Wrap is
// set: then it also wraps by words at its width (a word wider than the
// width breaks), in cells in the terminal and in the font in a GUI. A
// wrapping Label fills the width on an Auto width, and its Auto height is
// its wrapped line count at the width it gets.
type Label struct {
	NodeBase
	Text  string
	Style TextStyle
	Wrap  bool
}

func (l Label) Size() (Size, Size) {
	w, h := Text{NodeBase: l.NodeBase, String: l.Style.Render(l.Text)}.Size()
	if l.Wrap && l.W.Kind == SizeAuto {
		w = Grow(1)
	}
	return w, h
}

func (l Label) Render(r Rect) string {
	text := l.Text
	if l.Wrap {
		text = wrapCells(text, r.insetBy(l.insetSides()).W)
	}
	return fillBox(l.Style.Render(text), r, l.padSides(), l.box())
}

// wrapHeight is a wrapping Label's Auto height in cells at a rect width
// (its unwrapped height when the width is unknown, width < 0).
func (l Label) wrapHeight(width int) Size {
	_, h := l.Size()
	if h.Kind != SizeContent || width < 0 {
		return h
	}
	inset := l.insetSides()
	lines := strings.Count(wrapCells(l.Text, satSub(width, inset.horiz())), "\n") + 1
	return Size{Kind: SizeContent, Value: satAdd(lines, inset.vert())}
}

// wrapCells wraps text by words at width cells, breaking words wider than
// the width; width <= 0 leaves it unwrapped. "\n" still breaks lines.
func wrapCells(text string, width int) string {
	if width <= 0 {
		return text
	}
	return ansi.Wrap(text, width, "")
}
