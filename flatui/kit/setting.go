package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Setting is a settings row: a name with an optional description under
// it, and the control (a toggle, a segmented choice, a field) at the right.
type Setting struct {
	ID          string
	Name        string
	Description string
	Control     layout.Node
	// Stacked puts the control under the name and description instead of
	// beside them, for a column too narrow to share.
	Stacked bool
}

func (s Setting) Layout() layout.Node {
	text := []layout.Node{layout.Label{Text: s.Name, Style: layout.TextStyle{Role: layout.TextStrong}, Wrap: true}}
	if s.Description != "" {
		text = append(text, layout.Label{Text: s.Description, Style: layout.TextStyle{Role: layout.TextMeta}, Wrap: true})
	}
	if s.Stacked {
		if s.Control != nil {
			text = append(text, layout.Row{NodeBase: layout.NodeBase{PadTop: 1, NoPadding: true}, Children: []layout.Node{s.Control}})
		}
		return layout.Col{NodeBase: layout.NodeBase{ID: s.ID, W: layout.Grow(1), PadLeft: 1, PadRight: 1}, Children: text}
	}
	kids := []layout.Node{layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Children: text}}
	if s.Control != nil {
		kids = append(kids, s.Control)
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: s.ID, Gap: 2, PadLeft: 1, PadRight: 1}, Align: layout.AlignCenter, Children: kids}
}
