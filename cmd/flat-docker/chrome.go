package main

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

// chromeRowsBottom is the number of rows reserved at the bottom of the frame
// (separator + footer/help line), both anchored to the bottom.
const chromeRowsBottom = 2

// --- Node types for the frame chrome (embed layout.NodeBase) ---

type Header struct {
	layout.NodeBase
	State *State
}

func (h Header) Size() (layout.Size, layout.Size) {
	w, hh := h.NodeBase.Size()
	if w.Kind == layout.SizeAuto || hh.Kind == layout.SizeAuto {
		h.State.headerTabs.SetActive(int(h.State.screen))
		row := h.row()
		rw, rh := row.Size()
		if w.Kind == layout.SizeAuto {
			w = rw
		}
		if hh.Kind == layout.SizeAuto {
			hh = rh
			if h.State.headerBorder.Top {
				hh.Value++
			}
			if h.State.headerBorder.Bottom {
				hh.Value++
			}
		}
	}
	return w, hh
}

func (h Header) Render(r layout.Rect) string {
	h.State.headerTabs.SetActive(int(h.State.screen))

	row := h.row()

	contentHeight := r.H
	if h.State.headerBorder.Top {
		contentHeight--
	}
	if h.State.headerBorder.Bottom {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	content, _ := layout.SolveAndCompose(row, r.W-headerBorderHorizontalWidth(h.State.headerBorder), contentHeight)
	return renderBorderedHeader(content, r.W, h.State.headerBorder)
}

func (h Header) row() layout.Row {
	title := lipgloss.NewStyle().Bold(true).Background(pal.bg).Foreground(pal.accent).Padding(0, 0).
		Render("flat-docker")

	return layout.Row{
		Children: []layout.Node{
			layout.Text{String: title},
			layout.NewSpacer().WithBackground(pal.bg),
			h.State.headerTabs.Layout(),
		},
	}
}

func renderBorderedHeader(content string, width int, border flatui.TabBarBorder) string {
	if !headerBorderEnabled(border) {
		return content
	}
	innerWidth := width - headerBorderHorizontalWidth(border)
	if innerWidth < 0 {
		innerWidth = 0
	}
	style := lipgloss.NewStyle().
		Width(innerWidth).
		BorderStyle(headerBorderStyle(border)).
		BorderTop(border.Top).
		BorderRight(border.Right).
		BorderBottom(border.Bottom).
		BorderLeft(border.Left)
	if border.Color != nil {
		style = style.BorderForeground(border.Color)
	}
	return style.Render(content)
}

func headerBorderEnabled(border flatui.TabBarBorder) bool {
	return border.Top || border.Right || border.Bottom || border.Left
}

func headerBorderStyle(border flatui.TabBarBorder) lipgloss.Border {
	if border.Style == (lipgloss.Border{}) {
		return lipgloss.InnerHalfBlockBorder()
	}
	return border.Style
}

func headerBorderHorizontalWidth(border flatui.TabBarBorder) int {
	w := 0
	if border.Left {
		w++
	}
	if border.Right {
		w++
	}
	return w
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
