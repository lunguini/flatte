package window

import (
	"sort"
	"strings"
)

// FontFamily is a font family installed on the system, by the name a Theme's
// Font or MonoFont takes.
type FontFamily struct {
	Name string
	// Monospace reports a fixed-pitch family: every glyph is as wide.
	Monospace bool
}

// FontFamilies lists the font families installed on this system, sorted
// by name ignoring case. It asks the system each time and can take a
// moment, so call it off the loop (in a flatte.Go) and keep the result.
// It is empty where the host cannot list fonts: terminals off macOS, a
// browser, and on a failure.
func FontFamilies() []FontFamily {
	fams := fontFamilies()
	sortFamilies(fams)
	return fams
}

// sortFamilies orders families by name ignoring case, then by name.
func sortFamilies(fams []FontFamily) {
	sort.SliceStable(fams, func(i, j int) bool {
		a, b := strings.ToLower(fams[i].Name), strings.ToLower(fams[j].Name)
		if a != b {
			return a < b
		}
		return fams[i].Name < fams[j].Name
	})
}
