package main

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
	title := lipgloss.NewStyle().Bold(true).Background(pal.bg).Foreground(pal.accent).
		Render("flat-docker")

	content := layout.Row{
		Children: []layout.Node{
			layout.Text{String: title},
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

type Separator struct{ layout.NodeBase }

func (s Separator) Size() (layout.Size, layout.Size) {
	w, h := s.NodeBase.Size()
	if h.Kind == layout.SizeAuto {
		h = layout.Fixed(1)
	}
	return w, h
}

func (s Separator) Render(r layout.Rect) string {
	return lipgloss.NewStyle().Width(r.W).Background(pal.accent).
		Render(strings.Repeat(" ", r.W))
}

type Footer struct {
	layout.NodeBase
	State *State
}

func (f Footer) Size() (layout.Size, layout.Size) {
	w, h := f.NodeBase.Size()
	if h.Kind == layout.SizeAuto {
		h = layout.Fixed(1)
	}
	return w, h
}

func (f Footer) Render(r layout.Rect) string {
	var hints string
	if f.State.commandModal != nil {
		hints = f.State.commandModal.keyHints()
	} else {
		switch f.State.screen {
		case screenContainers:
			hints = f.State.containers.keyHints()
		case screenImages:
			hints = f.State.images.keyHints()
		case screenVolumes:
			hints = f.State.volumes.keyHints()
		}
	}
	return lipgloss.NewStyle().Width(r.W).Foreground(pal.muted).
		Render(" " + hints + " ")
}
