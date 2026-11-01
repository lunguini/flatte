package dockerapp

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

// chromeRowsBottom is the number of rows reserved at the bottom of the frame
// (separator + footer/help line), both anchored to the bottom.
const chromeRowsBottom = 2

// --- Frame chrome as plain layout subtrees ---

// headerNode builds the header as a real subtree of the frame tree: a content
// row (title, spacer, tab strip) optionally sandwiched between top/bottom
// border rules. Because it lives directly in the frame tree, the single frame
// solve records each tab's rect into s.rects for hit-testing — no nested solve
// that discards geometry, no manual coordinate math.
func headerNode(s *State) layout.Node {
	content := layout.Row{
		Children: []layout.Node{
			layout.Label{Text: "flat-docker", Style: layout.TextStyle{Bold: true, Bg: pal.bg, Fg: pal.accent}},
			layout.NewSpacer().WithBackground(pal.bg),
			s.headerTabs.Layout(),
		},
	}

	b := s.headerBorder
	if !b.Top && !b.Bottom {
		return content
	}

	style := headerBorderStyle(b)
	children := make([]layout.Node, 0, 3)
	if b.Top {
		children = append(children, headerRule{glyph: style.Top, color: b.Color})
	}
	children = append(children, content)
	if b.Bottom {
		children = append(children, headerRule{glyph: style.Bottom, color: b.Color})
	}
	return layout.Col{Children: children}
}

// headerHeight is the header subtree's row count: one content row plus each
// enabled horizontal border rule. resize uses it to place the body.
func headerHeight(s *State) int {
	h := 1
	if s.headerBorder.Top {
		h++
	}
	if s.headerBorder.Bottom {
		h++
	}
	return h
}

// headerRule is a full-width horizontal border line reproducing what a
// Lip Gloss top/bottom border draws — a leaf so the solver owns its rect.
type headerRule struct {
	layout.NodeBase
	glyph string
	color color.Color
}

func (h headerRule) Size() (layout.Size, layout.Size) {
	return layout.Auto(), layout.Fixed(1)
}

func (h headerRule) Render(r layout.Rect) string {
	if r.W <= 0 {
		return ""
	}
	style := lipgloss.NewStyle()
	if h.color != nil {
		style = style.Foreground(h.color)
	}
	return style.Render(strings.Repeat(h.glyph, r.W))
}

func headerBorderStyle(border flatui.TabBarBorder) lipgloss.Border {
	if border.Style == (lipgloss.Border{}) {
		return lipgloss.InnerHalfBlockBorder()
	}
	return border.Style
}

// separatorNode is the full-width accent rule above the footer.
func separatorNode() layout.Node {
	return layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1), Box: layout.Box{Background: pal.accent}}}
}

// footerNode is the key-hint line for whatever has the keyboard.
func footerNode(s *State) layout.Node {
	var hints string
	if s.commandModal != nil {
		hints = s.commandModal.keyHints()
	} else {
		switch s.screen {
		case screenContainers:
			hints = s.containers.keyHints()
		case screenImages:
			hints = s.images.keyHints()
		case screenVolumes:
			hints = s.volumes.keyHints()
		}
	}
	return layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: " " + hints + " ", Style: layout.TextStyle{Fg: pal.muted}}
}
