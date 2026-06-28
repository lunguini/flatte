package flatui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

const (
	fallbackCardWidth     = 72
	cardBorderColumns     = 2
	cardHorizontalPadding = 4
)

func Card(lines []string, maxWidth int) string {
	return CardWithStyle(lines, maxWidth, CardStyle{})
}

func CardWithStyle(lines []string, maxWidth int, style CardStyle) string {
	width := FrameWidth(maxWidth, lines)
	container := style.Container.
		Width(width).
		Padding(0, 2).
		Border(lipgloss.NormalBorder())
	if style.BorderForeground != nil {
		container = container.BorderForeground(style.BorderForeground)
	} else {
		container = container.BorderForeground(lipgloss.Color("240"))
	}
	return container.Render(strings.Join(lines, "\n"))
}

func Title(text string) string {
	return TitleWithStyle(text, lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("235")).
		Padding(0, 1))
}

func TitleWithStyle(text string, style lipgloss.Style) string {
	return style.Render(text)
}

func Subtle(text string) string {
	return SubtleWithStyle(text, lipgloss.NewStyle().Foreground(lipgloss.Color("244")))
}

func SubtleWithStyle(text string, style lipgloss.Style) string {
	return style.Render(text)
}

// Overlay draws layer centered over base as a cell-buffer composite. It
// delegates to the layout engine's compositor so there is a single
// ANSI-aware overlay implementation (see flatui/layout/overlay.go).
func Overlay(base string, layer string) string {
	return layout.Overlay(base, layer)
}

// CardOrigin is the cell offset of a Card's first content cell relative
// to the card's top-left corner: one border column plus two padding
// columns across, one border row down.
func CardOrigin() (x, y int) {
	return 1 + cardHorizontalPadding/2, 1
}

// OverlayOrigin returns where Overlay places layer's top-left cell inside
// the composed frame. Delegates to the layout engine so it stays in lockstep
// with Overlay's centering.
func OverlayOrigin(base, layer string) (x, y int) {
	return layout.OverlayOrigin(base, layer)
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
