package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// EmptyState fills a pane with nothing to show: an icon, a title, a hint
// and actions (buttons), centered.
type EmptyState struct {
	ID      string
	Icon    layout.IconName
	Title   string
	Hint    string
	Actions []layout.Node
	// Inline sets it at the top left with the hint wrapping, for a narrow
	// sidebar, instead of centered in the pane. It adds no inset: put it
	// in the pane's body, which has layout.NodeBase.PanePadding.
	Inline bool
}

func (e EmptyState) Layout() layout.Node {
	kids := []layout.Node{}
	if e.Icon != layout.IconNone {
		kids = append(kids, icon(e.Icon, layout.ColorMuted))
	}
	if e.Title != "" {
		role := layout.TextTitle
		if e.Inline { // a sidebar's title is strong body text, not a page title
			role = layout.TextStrong
		}
		kids = append(kids, layout.Label{Text: e.Title, Style: layout.TextStyle{Role: role}})
	}
	if e.Hint != "" {
		kids = append(kids, layout.Label{Text: e.Hint, Style: layout.TextStyle{Role: layout.TextMeta}, Wrap: e.Inline})
	}
	if len(e.Actions) > 0 {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: e.Actions})
	}
	if e.Inline {
		return layout.Col{NodeBase: layout.NodeBase{ID: e.ID, W: layout.Grow(1), Gap: 1}, Align: layout.AlignStart, Children: kids}
	}
	return layout.Col{
		NodeBase: layout.NodeBase{ID: e.ID, W: layout.Grow(1), H: layout.Grow(1)},
		Align:    layout.AlignCenter,
		Children: []layout.Node{
			&layout.Spacer{NodeBase: layout.NodeBase{H: layout.Grow(1)}},
			layout.Col{NodeBase: layout.NodeBase{Gap: 1}, Align: layout.AlignCenter, Children: kids},
			&layout.Spacer{NodeBase: layout.NodeBase{H: layout.Grow(1)}},
		},
	}
}
