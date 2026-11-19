package theme

import "image/color"

// Pair is one value per appearance.
type Pair struct{ Light, Dark color.NRGBA }

// Density scales the shared sizes and the middle spacing steps.
type Density int

const (
	Regular Density = iota
	Compact
	Comfortable
)

// Roundness sets the corner radii.
type Roundness int

const (
	Subtle Roundness = iota
	Square
	Round
)

// Seeds are the few choices an app makes for an identity; zero values keep
// the theme's own (Regular, Subtle, its accent).
type Seeds struct {
	// Accent sets Accent and Focus per appearance, and derives
	// AccentHover, SelectionActive, TextSelection, Info and Syntax.Func
	// from it.
	Accent    *Pair
	Density   Density
	Roundness Roundness
}

var (
	regularSpace     = [6]float64{4, 8, 12, 16, 24, 32}
	compactSpace     = [6]float64{4, 6, 10, 12, 20, 28}
	comfortableSpace = [6]float64{4, 10, 14, 20, 28, 36}

	regularSizes     = Sizes{Row: 24, Control: 28, Bar: 36, Header: 44, Icon: 16, Popover: 560}
	compactSizes     = Sizes{Row: 22, Control: 24, Bar: 30, Header: 38, Icon: 14, Popover: 520}
	comfortableSizes = Sizes{Row: 28, Control: 32, Bar: 40, Header: 48, Icon: 16, Popover: 600}

	subtleRadii = Radii{Sm: 4, Md: 6, Lg: 10}
	squareRadii = Radii{}
	roundRadii  = Radii{Sm: 8, Md: 10, Lg: 16}
)

// With is t with the seeds applied.
func (t Theme) With(s Seeds) Theme {
	switch s.Density {
	case Compact:
		t.Space, t.Size = compactSpace, compactSizes
	case Comfortable:
		t.Space, t.Size = comfortableSpace, comfortableSizes
	}
	switch s.Roundness {
	case Square:
		t.Radius = squareRadii
	case Round:
		t.Radius = roundRadii
	}
	if s.Accent != nil {
		t.Light = withAccent(t.Light, s.Accent.Light, false)
		t.Dark = withAccent(t.Dark, s.Accent.Dark, true)
	}
	return t
}

// withAccent derives p's accent family from a: hover a step toward
// contrast, selection a tint of it over the content surface.
func withAccent(p Palette, a color.NRGBA, dark bool) Palette {
	p.Accent, p.Focus, p.Info, p.Syntax.Func = a, a, a, a
	if dark {
		p.AccentHover = shift(a, 0.06)
		p.SelectionActive = Mix(p.Content, a, 0.2)
		p.TextSelection = Mix(p.Content, a, 0.42)
		p.InfoTint = Mix(p.Content, a, 0.16)
		p.OnAccent = Hex("#0b1220")
	} else {
		p.AccentHover = shift(a, -0.06)
		p.SelectionActive = Mix(p.Content, a, 0.16)
		p.TextSelection = Mix(p.Content, a, 0.3)
		p.InfoTint = Mix(p.Content, a, 0.1)
		p.OnAccent = Hex("#ffffff")
	}
	if Contrast(p.OnAccent, a) < 4.5 { // a light accent in light mode, or a dark one in dark
		p.OnAccent = Hex("#ffffff")
		if Contrast(p.OnAccent, a) < Contrast(Hex("#0b1220"), a) {
			p.OnAccent = Hex("#0b1220")
		}
	}
	return p
}
