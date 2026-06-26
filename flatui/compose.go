package flatui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// ComposeHeader places left content at the left edge and right content at
// the right edge of a width-bounded row, on a shared background. The gap
// between them absorbs the remaining space. If left + right don't fit,
// left is truncated (with an ellipsis) to make room for right.
//
// This is the composition primitive for "title left, tabs right" or any
// two-element header layout. It does NOT aggregate the two elements into
// one component — the caller owns which is which and which side each goes on.
func ComposeHeader(left, right string, width int, bg color.Color) string {
	rightW := lipgloss.Width(right)
	if rightW >= width {
		return lipgloss.NewStyle().Width(width).MaxWidth(width).Background(bg).Render(right)
	}
	maxLeftW := width - rightW - 1
	leftW := lipgloss.Width(left)
	if leftW > maxLeftW {
		if maxLeftW > 1 {
			left = truncateAnsi(left, maxLeftW-1) + "…"
		} else {
			left = ""
		}
		leftW = lipgloss.Width(left)
	}
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	spacer := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Background(bg).Render(left + spacer + right)
}

func truncateAnsi(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	stripped := make([]rune, 0, width)
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' || (r >= '@' && r <= '~') {
				inEscape = false
			}
			continue
		}
		stripped = append(stripped, r)
		if len(stripped) >= width {
			break
		}
	}
	return string(stripped)
}
