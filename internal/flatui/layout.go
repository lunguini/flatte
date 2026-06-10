package flatui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
		Width(ContentWidth(width)).
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

	left := max(0, (width-layerBounds.Dx())/2)
	top := max(0, (height-layerBounds.Dy())/2)
	layerArea := uv.Rect(left, top, layerBounds.Dx(), layerBounds.Dy())
	canvas.FillArea(&uv.EmptyCell, layerArea) // the layer rectangle covers the base
	layerStyled.Draw(canvas, layerArea)

	return trimTrailingSpace(canvas.Render())
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
