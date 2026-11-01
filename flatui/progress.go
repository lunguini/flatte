package flatui

import (
	"fmt"
	"math"
	"strings"

	"github.com/lunguini/flatte/flatui/layout"
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

// SetWidth sets the fill bar width in cells. Values outside the supported
// 0..65,536 cell range clamp to the nearest endpoint.
func (p *Progress) SetWidth(width int) {
	p.width = clampWidgetDimension(width)
}

// SetPercent sets the fill percentage. Finite values outside 0..100 clamp;
// NaN normalizes to zero and infinities clamp to the corresponding endpoint.
func (p *Progress) SetPercent(percent float64) {
	if math.IsNaN(percent) {
		percent = 0
	}
	p.percent = min(max(percent, 0), 100)
}

// Percent returns the clamped percentage.
func (p Progress) Percent() float64 { return p.percent }

// Width returns the bar width in cells.
func (p Progress) Width() int { return p.width }

// Layout returns the bar as a layout.Meter of the bar's width followed by
// the percentage label: View's text in the terminal, a native bar in a
// window. A zero-width bar is only the label, as in View.
func (p Progress) Layout() layout.Node {
	label := layout.Label{Text: fmt.Sprintf("%3d%%", int(math.Round(p.percent)))}
	if p.width <= 0 {
		return label
	}
	label.Text = "  " + label.Text
	return layout.Row{Children: []layout.Node{
		layout.Meter{NodeBase: layout.NodeBase{W: layout.Fixed(p.width)}, Value: p.percent / 100},
		label,
	}}
}

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

func (p Progress) ViewWithStyle(style ProgressStyle) string {
	label := fmt.Sprintf("%3d%%", int(math.Round(p.percent)))
	if p.width <= 0 {
		return style.Label.Render(label)
	}
	filled := int(math.Round(p.percent / 100 * float64(p.width)))
	filled = min(max(filled, 0), p.width)
	filledText := strings.Repeat("█", filled)
	emptyText := strings.Repeat("░", p.width-filled)
	if filledText != "" {
		filledText = style.Filled.Render(filledText)
	}
	if emptyText != "" {
		emptyText = style.Empty.Render(emptyText)
	}
	return filledText + emptyText + "  " + style.Label.Render(label)
}
