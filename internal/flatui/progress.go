package flatui

import (
	"fmt"
	"math"
	"strings"
)

// Progress is a horizontal percentage indicator. The app owns the percentage
// and width; the widget only clamps values and renders a deterministic bar.
type Progress struct {
	width   int
	percent float64
}

// NewProgress returns a progress bar with the given fill width in cells.
func NewProgress(width int) Progress {
	var p Progress
	p.SetWidth(width)
	return p
}

// SetWidth sets the fill bar width in cells. Negative widths clamp to zero.
func (p *Progress) SetWidth(width int) {
	p.width = max(width, 0)
}

// SetPercent sets the fill percentage. Values outside 0..100 clamp.
func (p *Progress) SetPercent(percent float64) {
	p.percent = min(max(percent, 0), 100)
}

// Percent returns the clamped percentage.
func (p Progress) Percent() float64 { return p.percent }

// Width returns the bar width in cells.
func (p Progress) Width() int { return p.width }

// View renders the progress bar and a rounded percentage label. A zero-width
// bar still returns the label so compact layouts can show progress.
func (p Progress) View() string {
	label := fmt.Sprintf("%3d%%", int(math.Round(p.percent)))
	if p.width <= 0 {
		return label
	}
	filled := int(math.Round(p.percent / 100 * float64(p.width)))
	filled = min(max(filled, 0), p.width)
	return strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled) + "  " + label
}
