package layout

import (
	"image/color"

	"charm.land/lipgloss/v2"
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
// sequences (use Text for pre-styled strings); lines break only at "\n".
// Like Text, it auto-sizes to its content on Auto axes.
type Label struct {
	NodeBase
	Text  string
	Style TextStyle
}

func (l Label) Size() (Size, Size) {
	return Text{NodeBase: l.NodeBase, String: l.Style.Render(l.Text)}.Size()
}

func (l Label) Render(r Rect) string {
	return fillBox(l.Style.Render(l.Text), r, l.padSides(), l.box())
}
