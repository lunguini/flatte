package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Section is a group under a caption: settings, a sidebar's group.
type Section struct {
	ID       string
	Title    string
	Children []layout.Node
	// Card sets the rows on a rounded raised surface with a hairline
	// between them, as a settings form groups its rows (a GUI; a terminal
	// keeps the blank line between them).
	Card bool
}

func (s Section) Layout() layout.Node {
	kids := make([]layout.Node, 0, len(s.Children)+1)
	if s.Title != "" {
		// At the bar height, low in it: nearer its rows than what is above.
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{H: layout.BarSize(), PadLeft: 1}, Align: layout.AlignEnd, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: s.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true},
		}})
	}
	if !s.Card {
		kids = append(kids, s.Children...)
		return layout.Col{NodeBase: layout.NodeBase{ID: s.ID, Gap: 1}, Children: kids}
	}
	space := func(points, cells int) layout.Node {
		return layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Points(points).Cells(cells), NoPadding: true}}
	}
	rows := []layout.Node{space(cardInset, 0)}
	for i, c := range s.Children {
		if i > 0 {
			rows = append(rows, space(cardInset, 0),
				// The canvas, not the divider: the divider is near the
				// raised fill in a dark palette.
				layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Points(1).Cells(0), NoPadding: true, Box: layout.Box{Fill: layout.FillCanvas}}},
				space(cardInset, 1))
		}
		rows = append(rows, c)
	}
	rows = append(rows, space(cardInset, 0))
	kids = append(kids, layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), NoPadding: true, Box: layout.Box{Fill: layout.FillRaised, Round: true}}, Children: rows})
	return layout.Col{NodeBase: layout.NodeBase{ID: s.ID, Gap: 1}, Children: kids}
}

// cardInset is the room above and below each row of a Card section, in
// points.
const cardInset = 10
