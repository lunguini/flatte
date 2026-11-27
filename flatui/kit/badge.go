package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Badge is a short status label on its tone's tint: a count, a state.
type Badge struct {
	Text string
	Tone Tone
}

func (b Badge) Layout() layout.Node {
	return layout.Label{
		NodeBase: layout.NodeBase{PadLeft: 1, PadRight: 1, NoPadding: true, Box: layout.Box{Fill: b.Tone.fill(), Round: true}},
		Text:     b.Text, Style: layout.TextStyle{Role: layout.TextMeta, Color: b.Tone.color()},
	}
}
