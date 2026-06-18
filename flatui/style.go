package flatui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// CardStyle customizes CardWithStyle. Zero fields use Card's defaults.
type CardStyle struct {
	Container        lipgloss.Style
	BorderForeground color.Color
}

// ProgressStyle customizes Progress.ViewWithStyle. Zero-value styles render
// the same text without additional styling.
type ProgressStyle struct {
	Filled lipgloss.Style
	Empty  lipgloss.Style
	Label  lipgloss.Style
}

// TableStyle customizes Table styled rendering. Zero-value styles render the
// same text without additional styling.
type TableStyle struct {
	Header lipgloss.Style
	Row    lipgloss.Style
	Active lipgloss.Style
}
