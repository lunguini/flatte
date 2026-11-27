package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Action is a small icon button inside a row, a tab or a pane header — a
// tab's close, a row's reveal: in a window a compact square with the hover
// wash under its icon, in a terminal its glyph. Its node ID is ID.
type Action struct {
	ID      string
	Icon    layout.IconName
	Glyph   string // the terminal spelling ("" : the icon's glyph)
	Tooltip string
	Color   layout.TextColor // of the icon (the default: secondary)
}

func (a Action) Layout() layout.Node {
	c := a.Color
	if c == layout.ColorDefault {
		c = layout.ColorSecondary
	}
	g := a.Glyph
	if g == "" {
		g = glyph(a.Icon)
	}
	return layout.Row{
		NodeBase: layout.NodeBase{ID: a.ID, W: layout.Points(actionPoints), NoPadding: true, Tooltip: a.Tooltip, Box: layout.Box{Round: true, Interactive: a.ID != ""}},
		Align:    layout.AlignCenter,
		Children: []layout.Node{
			&layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}},
			layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(actionIconPoints)}, Name: a.Icon, Text: g, Style: layout.TextStyle{Color: c}},
			&layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}},
		},
	}
}

// An Action's box and icon widths in a window (two cells each in a
// terminal: the glyph and a space).
const (
	actionPoints     = 16
	actionIconPoints = 10
)

// ToggleButton is an on/off button with a short label or an icon — a find
// bar's match case: in a window a control-sized square, selected while On,
// with the hover wash under it; in a terminal its label, strong while On.
// Focused draws the focus look. Its node ID is ID: clicks arrive on it,
// and flipping On (on a click, or Space or Enter while focused) is the
// app's.
type ToggleButton struct {
	ID      string
	Label   string          // the text, and an icon's terminal spelling
	Icon    layout.IconName // drawn instead of Label in a window
	Tooltip string
	On      bool
	Focused bool
}

func (t ToggleButton) Layout() layout.Node {
	style := layout.TextStyle{Color: layout.ColorSecondary}
	if t.On {
		style = layout.TextStyle{Role: layout.TextStrong}
	}
	return layout.Button{
		NodeBase: layout.NodeBase{ID: t.ID, W: layout.ControlSize(), H: layout.ControlSize(), Tooltip: t.Tooltip,
			Box: layout.Box{Selected: t.On, Round: true, Interactive: t.ID != ""}},
		Label: t.Label, Icon: t.Icon, Style: style, Plain: true, Focused: t.Focused,
	}
}
