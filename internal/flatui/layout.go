package flatui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func Overlay(base string, layer string) string {
	baseRows := splitRows(base)
	layerRows := splitRows(layer)
	if len(baseRows) == 0 || len(layerRows) == 0 {
		return base
	}

	baseWidth := widestRow(baseRows)
	layerWidth := widestRow(layerRows)
	left := max(0, (baseWidth-layerWidth)/2)
	top := max(0, (len(baseRows)-len(layerRows))/2)

	for rowIndex, layerRow := range layerRows {
		target := top + rowIndex
		for target >= len(baseRows) {
			baseRows = append(baseRows, strings.Repeat(" ", baseWidth))
		}
		baseRows[target] = overlayRow(baseRows[target], padVisible(layerRow, layerWidth), left, layerWidth)
	}

	return strings.Join(baseRows, "\n")
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

func splitRows(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.Split(value, "\n")
}

func widestRow(rows []string) int {
	width := 0
	for _, row := range rows {
		if rowWidth := ansi.StringWidth(row); rowWidth > width {
			width = rowWidth
		}
	}
	return width
}

func overlayRow(base string, layer string, left int, width int) string {
	baseWidth := ansi.StringWidth(base)
	if baseWidth < left {
		base += strings.Repeat(" ", left-baseWidth)
		baseWidth = left
	}

	right := left + width
	prefix := ansi.Cut(base, 0, left)
	suffix := ""
	if right < baseWidth {
		suffix = ansi.Cut(base, right, baseWidth)
	}
	return prefix + layer + suffix
}

func padVisible(value string, width int) string {
	if short := width - ansi.StringWidth(value); short > 0 {
		return value + strings.Repeat(" ", short)
	}
	return value
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
