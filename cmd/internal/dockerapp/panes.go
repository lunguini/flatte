package dockerapp

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

const (
	dividerWidth     = 1
	minListWidth     = 20
	minDetailWidth   = 20
	minActivityWidth = 12
)

// splitDrag tracks a 2-pane divider drag (images/volumes screens).
type splitDrag struct {
	startX     int
	startWidth int
}

// solveScreenRects solves a screen's body subtree and returns its geometry in
// absolute frame coordinates by offsetting the origin-based solve to the body
// rect. Used by the resize and drag paths, which derive geometry without going
// through the full frame render in View.
func solveScreenRects(body layout.Node, bodyRect layout.Rect) map[string]layout.Rect {
	rects := layout.Solve(body, bodyRect.W, bodyRect.H)
	if bodyRect.X == 0 && bodyRect.Y == 0 {
		return rects
	}
	for id, r := range rects {
		r.X += bodyRect.X
		r.Y += bodyRect.Y
		rects[id] = r
	}
	return rects
}

func scrollbarLines(offset, visible, total, height int) string {
	if total <= visible || height <= 0 || visible <= 0 {
		return strings.Repeat(" ", height)
	}
	maxOffset := total - visible
	thumbSize := max(1, height*visible/total)
	if thumbSize > height {
		thumbSize = height
	}
	thumbPos := int(float64(offset) / float64(maxOffset) * float64(height-thumbSize))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos+thumbSize > height {
		thumbPos = height - thumbSize
	}
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			rows[i] = "█"
		} else {
			rows[i] = "░"
		}
	}
	return strings.Join(rows, "\n")
}

func withScrollbar(content, bar string) string {
	if bar == "" {
		return content
	}
	contentLines := strings.Split(content, "\n")
	barLines := strings.Split(bar, "\n")
	n := max(len(contentLines), len(barLines))
	out := make([]string, n)
	for i := 0; i < n; i++ {
		var line, barChar string
		if i < len(contentLines) {
			line = contentLines[i]
		}
		if i < len(barLines) {
			barChar = barLines[i]
		} else {
			barChar = " "
		}
		out[i] = line + barChar
	}
	return strings.Join(out, "\n")
}

// renderDragDivider produces a 1-col-wide, height-tall divider with a
// background color matching the pane borders, and a single │ in the
// vertical center as a visible grip indicator. The bg fills the column
// (spaces are invisible against it); the │ stands out as a contrast.
func renderDragDivider(height int, dragging bool) string {
	bg := pal.panel
	gripFg := pal.bg // dark against gray — clearly visible
	if dragging {
		bg = pal.accent
		gripFg = pal.dark // dark against blue
	}
	midY := height / 2
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if i == midY {
			rows[i] = "│"
		} else {
			rows[i] = " "
		}
	}
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(dividerWidth).
		Height(height).
		MaxHeight(height).
		Foreground(gripFg).
		Background(bg).
		Render(content)
}
