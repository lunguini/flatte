package kit

import (
	"strconv"

	"github.com/lunguini/flatte/flatui/layout"
)

// Segmented shows a few options side by side while they fit; when narrow it
// shows a popup button with the chosen option. Option i's node ID is ID +
// ":" + its index (OptionID). The app owns Open and Highlighted; Tooltip is
// optional.
type Segmented struct {
	ID          string
	Options     []string
	Selected    int
	Focused     bool
	Tooltip     string // help text for the choice
	Open        bool   // show the popup menu in the narrow layout
	Highlighted int    // option to highlight while Open; an invalid value uses Selected
}

// OptionID is the node ID of a Segmented's option i.
func OptionID(id string, i int) string { return id + ":" + strconv.Itoa(i) }

func (s Segmented) Layout() layout.Node {
	kids := make([]layout.Node, len(s.Options))
	for i, o := range s.Options {
		box := layout.Box{Round: true, Interactive: i != s.Selected}
		style := layout.TextStyle{Color: layout.ColorSecondary}
		if i == s.Selected {
			box.Fill = layout.FillRaised
			box.Selected = s.Focused
			style = layout.TextStyle{Role: layout.TextStrong}
		}
		// The option fills the track's height and centers its label in it
		// (a Label draws its text at the top of its rect).
		kids[i] = layout.Row{
			NodeBase: layout.NodeBase{ID: OptionID(s.ID, i), PadLeft: 1, PadRight: 1, NoPadding: true, Tooltip: s.Tooltip, Box: box},
			Align:    layout.AlignCenter,
			Children: []layout.Node{layout.Label{Text: o, Style: style}},
		}
	}
	wide := layout.Row{
		NodeBase: layout.NodeBase{ID: s.ID, H: layout.ControlSize(), NoPadding: true, Tooltip: s.Tooltip, Box: layout.Box{Fill: layout.FillPressed, Round: true}},
		Align:    layout.AlignStretch, Children: kids,
	}
	return layout.Fit{Preferred: wide, Fallback: s.popup()}
}

func (s Segmented) popup() layout.Node {
	label := "Select"
	if s.Selected >= 0 && s.Selected < len(s.Options) {
		label = s.Options[s.Selected]
	}
	button := layout.Button{NodeBase: layout.NodeBase{ID: s.ID, Tooltip: s.Tooltip}, Label: label + " ▾", Focused: s.Focused}
	if !s.Open || len(s.Options) == 0 {
		return button
	}
	selected := s.Selected
	highlighted := s.Highlighted
	if highlighted < 0 || highlighted >= len(s.Options) {
		highlighted = selected
	}
	rows := make([]layout.Node, len(s.Options))
	for i, option := range s.Options {
		style := layout.TextStyle{Color: layout.ColorSecondary}
		if i == selected {
			style.Role = layout.TextStrong
		}
		rows[i] = layout.Row{
			NodeBase: layout.NodeBase{ID: OptionID(s.ID, i), ItemPadding: true, Box: layout.Box{Interactive: true, Selected: i == highlighted}},
			Align:    layout.AlignCenter,
			Children: []layout.Node{layout.Label{Text: option, Style: style}},
		}
	}
	menu := layout.Col{
		NodeBase: layout.NodeBase{ID: s.ID + ":menu", Overlay: true, Anchor: s.ID, Bordered: true, NoPadding: true},
		Children: rows,
	}
	return layout.Row{Children: []layout.Node{button, menu}}
}
