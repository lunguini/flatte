package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Popover is a raised panel over the frame: a quick-open list, a menu of
// choices. Anchor places it under the node with that ID (empty: centered);
// Width is its width (zero: its content's).
type Popover struct {
	ID       string
	Anchor   string
	Width    layout.Size
	Children []layout.Node
}

func (p Popover) Layout() layout.Node {
	return layout.Col{
		// The border band is the inset: no padding on top of it.
		NodeBase: layout.NodeBase{ID: p.ID, Overlay: true, Anchor: p.Anchor, W: p.Width, NoPadding: true,
			Box: layout.Box{Border: layout.BorderRounded, Fill: layout.FillRaised}},
		Children: p.Children,
	}
}
