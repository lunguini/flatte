// Package brand holds the shared Flatte brand palette so the sample apps and
// the landing showcase draw from one source of truth instead of scattering
// ANSI color codes. Colors are the published Flatte brand hex values.
package brand

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Flatte brand colors.
var (
	// Teal is the primary accent (titles, frames, focus).
	Teal color.Color = lipgloss.Color("#30C2B8")
	// Pink is the secondary accent (highlights, selection, alerts).
	Pink color.Color = lipgloss.Color("#EF227D")
	// Magenta is a deep accent between pink and purple.
	Magenta color.Color = lipgloss.Color("#852467")
	// Purple is the darkest brand tone, for panels and backgrounds.
	Purple color.Color = lipgloss.Color("#3C0956")
	// Blue is a muted, cool tone for secondary text and calm chrome.
	Blue color.Color = lipgloss.Color("#49819A")
)
