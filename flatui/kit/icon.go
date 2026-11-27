package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// icon is a component's icon: the shape in a GUI, 16pt wide, and its
// glyph in a terminal.
func icon(name layout.IconName, c layout.TextColor) layout.Node {
	return layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(16)}, Name: name, Text: glyph(name), Style: layout.TextStyle{Color: c}}
}

// glyph is an icon's terminal spelling.
func glyph(name layout.IconName) string {
	switch name {
	case layout.IconFile:
		return "▤"
	case layout.IconFolder:
		return "▸"
	case layout.IconDot:
		return "●"
	case layout.IconCircle:
		return "○"
	case layout.IconCheck:
		return "✓"
	case layout.IconClose:
		return "×"
	case layout.IconPlus:
		return "+"
	case layout.IconMinus:
		return "−"
	case layout.IconSearch:
		return "⌕"
	case layout.IconWarning:
		return "!"
	case layout.IconError:
		return "✕"
	case layout.IconInfo:
		return "i"
	case layout.IconGear:
		return "⚙"
	case layout.IconBranch:
		return "⎇"
	case layout.IconTerminal:
		return ">_"
	case layout.IconChevronRight, layout.IconDisclosureClosed:
		return "›"
	case layout.IconChevronDown, layout.IconDisclosureOpen:
		return "⌄"
	}
	return "•"
}
