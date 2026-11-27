package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// PaneHeader is the bar on top of a pane or sidebar: a caption title,
// optional meta text beside it, and trailing actions (icon buttons,
// usually) at the right, on the chrome surface at the bar height.
type PaneHeader struct {
	ID      string
	Title   string
	Meta    string
	Actions []layout.Node
	// Active brightens the caption while its pane has keyboard focus.
	Active bool
	// Body follows the title and takes the room before the actions: a
	// panel's tab strip (kit.Tabs), a filter field.
	Body layout.Node
	// Bare leaves the header unfilled, on the pane's own surface.
	Bare bool
}

func (h PaneHeader) Layout() layout.Node {
	caption := layout.TextStyle{Role: layout.TextCaption}
	if h.Active {
		caption.Color = layout.ColorPrimary
	}
	kids := []layout.Node{layout.Label{Text: h.Title, Style: caption, Truncate: true}}
	if h.Meta != "" {
		kids = append(kids, layout.Label{Text: h.Meta, Style: layout.TextStyle{Role: layout.TextMeta}, Truncate: true})
	}
	if h.Body != nil {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), NoPadding: true}, Align: layout.AlignStretch, Children: []layout.Node{h.Body}})
	} else {
		kids = append(kids, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	}
	kids = append(kids, h.Actions...)
	box := layout.Box{Fill: layout.FillChrome, Round: true, Flat: layout.SideBottom}
	if h.Bare {
		box.Fill = layout.FillNone
	}
	// The title starts where the pane's contents do: past the body's inset
	// (PanePadding, a step) and a row's own, in line with a Divider's title.
	return layout.Row{
		NodeBase: layout.NodeBase{ID: h.ID, H: layout.BarSize(), Gap: 1, PadLeft: 2, PadRight: 1, Box: box},
		Align:    layout.AlignCenter, Children: kids,
	}
}
