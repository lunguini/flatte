package flatui

import (
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	fallbackCardWidth     = 72
	cardBorderColumns     = 2
	cardHorizontalPadding = 4
)

func Card(lines []string, maxWidth int) string {
	width := FrameWidth(maxWidth, lines)
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Render(strings.Join(lines, "\n"))
}

func Title(text string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Render(text)
}

func Subtle(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(text)
}

// Overlay draws layer centered over base as a cell-buffer composite: both
// frames are parsed into cells, the layer rectangle covers the base
// (including padding short layer rows), and the result is serialized back
// to a styled string.
func Overlay(base string, layer string) string {
	baseStyled := uv.NewStyledString(base)
	layerStyled := uv.NewStyledString(layer)
	baseBounds := baseStyled.Bounds()
	layerBounds := layerStyled.Bounds()
	if baseBounds.Empty() || layerBounds.Empty() {
		return base
	}

	width := max(baseBounds.Dx(), layerBounds.Dx())
	height := max(baseBounds.Dy(), layerBounds.Dy())
	canvas := uv.NewScreenBuffer(width, height)
	baseStyled.Draw(canvas, canvas.Bounds())

	left, top := OverlayOrigin(base, layer)
	layerArea := uv.Rect(left, top, layerBounds.Dx(), layerBounds.Dy())
	canvas.FillArea(&uv.EmptyCell, layerArea) // the layer rectangle covers the base
	layerStyled.Draw(canvas, layerArea)

	return trimTrailingSpace(canvas.Render())
}

// CardOrigin is the cell offset of a Card's first content cell relative
// to the card's top-left corner: one border column plus two padding
// columns across, one border row down.
func CardOrigin() (x, y int) {
	return 1 + cardHorizontalPadding/2, 1
}

// OverlayOrigin returns where Overlay places layer's top-left cell inside
// the composed frame. Same centering math as Overlay — Overlay calls
// this, so they cannot drift apart.
func OverlayOrigin(base, layer string) (x, y int) {
	baseBounds := uv.NewStyledString(base).Bounds()
	layerBounds := uv.NewStyledString(layer).Bounds()
	if baseBounds.Empty() || layerBounds.Empty() {
		return 0, 0
	}
	width := max(baseBounds.Dx(), layerBounds.Dx())
	height := max(baseBounds.Dy(), layerBounds.Dy())
	return max(0, (width-layerBounds.Dx())/2), max(0, (height-layerBounds.Dy())/2)
}

func trimTrailingSpace(frame string) string {
	rows := strings.Split(frame, "\n")
	for i, row := range rows {
		rows[i] = strings.TrimRight(row, " ")
	}
	return strings.Join(rows, "\n")
}

func FrameWidth(maxWidth int, lines []string) int {
	if maxWidth <= 0 {
		maxWidth = fallbackCardWidth
	}

	content := 1
	for _, line := range lines {
		for _, segment := range strings.Split(line, "\n") {
			if width := lipgloss.Width(segment); width > content {
				content = width
			}
		}
	}

	width := content + cardHorizontalPadding + cardBorderColumns
	if width > maxWidth {
		return maxWidth
	}
	return width
}

func ContentWidth(totalWidth int) int {
	if totalWidth <= 0 {
		totalWidth = fallbackCardWidth
	}
	width := totalWidth - cardBorderColumns
	if width < 1 {
		return 1
	}
	return width
}

// cardBorderRows is the card's top + bottom border (it has no vertical padding).
const cardBorderRows = 2

// CardBodyWidth returns the usable content width for a body placed inside a
// Card of the given total width — the total minus the card's border and
// horizontal padding. Size a body widget with this (in Handle, on ResizeEvent)
// so it tracks the card chrome instead of a hardcoded constant.
func CardBodyWidth(totalWidth int) int {
	return max(totalWidth-cardBorderColumns-cardHorizontalPadding, 0)
}

// CardBodyHeight returns the rows available for a body inside a Card of the
// given total height, after the card's top and bottom border and the
// pinnedRows of non-body content (title, footer, etc.) the app stacks around
// the body. The app still declares its own pinnedRows — that count is app
// layout, not card chrome — but the card's border math lives here.
func CardBodyHeight(totalHeight, pinnedRows int) int {
	return max(totalHeight-cardBorderRows-pinnedRows, 0)
}
