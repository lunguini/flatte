package window

import (
	"image/color"
	"math"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// Theme is a window's look: the colors of everything the app does not color
// itself — the window, raised surfaces, text roles, borders, fields, and the
// accent used for focus, carets, and meters — and the type and corner shape
// the layout is measured and drawn in. Start from Dark or Light and change
// fields; zero fields take the base theme's value (Dark's, for a Theme built
// from scratch).
type Theme struct {
	Background, Surface color.Color
	Text, Subtle, Title color.Color
	Border, Field       color.Color
	// Accent marks focus: field outlines, carets, input-method text, and
	// meters without a Fill. Selection highlights selected rows.
	Accent, Selection color.Color
	// Track is a meter's unfilled bar.
	Track color.Color

	// Font is the UI font family and MonoFont the family of terminal cell
	// grids ("": the system UI font and the user's fixed-pitch font).
	Font, MonoFont string
	// FontSize is the body and cell-grid size and TitleSize the title size,
	// in points; LineSpacing is a line's height over its font's. Columns and
	// lines are measured in this type, so a larger size makes a larger
	// window for the same Options.Width and Height.
	FontSize, TitleSize, LineSpacing float64
	// ControlHeight is the minimum Auto height of regular controls in points.
	// Zero inherits the base theme (32pt); negative disables the minimum.
	// Explicit node heights and Bare/multiline fields are unaffected.
	ControlHeight float64
	// Radius is a plain border's corner radius in points (a rounded border
	// is twice as round); negative draws square corners.
	Radius float64
	// ANSI is the palette cell grids draw the 16 ANSI colors in (0–7
	// normal, 8–15 bright), as a terminal profile does: program output
	// asks for "blue", and the theme decides which blue reads on its
	// background. Nil entries take the base theme's. Colors 16–255 and
	// true colors are drawn as given.
	ANSI [16]color.Color
}

// cellColor is c as a cell grid draws it: an ANSI color 0–15 in t's
// palette, anything else unchanged.
func (t Theme) cellColor(c color.Color) color.Color { return ansiColor(&t.ANSI, c) }

func ansiColor(pal *[16]color.Color, c color.Color) color.Color {
	i := -1
	switch v := c.(type) {
	case ansi.BasicColor:
		i = int(v)
	case ansi.IndexedColor:
		i = int(v)
	}
	if i >= 0 && i < 16 && pal[i] != nil {
		return pal[i]
	}
	return c
}

// darkANSI reads on Dark's field: softened hues, a visible "black".
var darkANSI = [16]color.Color{
	rgb(0x54, 0x58, 0x62), rgb(0xe0, 0x6c, 0x75), rgb(0x98, 0xc3, 0x79), rgb(0xe5, 0xc0, 0x7b),
	rgb(0x61, 0xaf, 0xef), rgb(0xc6, 0x78, 0xdd), rgb(0x56, 0xb6, 0xc2), rgb(0xdc, 0xdf, 0xe4),
	rgb(0x7f, 0x84, 0x8e), rgb(0xff, 0x7b, 0x86), rgb(0xb5, 0xe8, 0x90), rgb(0xf0, 0xd5, 0x8a),
	rgb(0x7c, 0xc4, 0xff), rgb(0xd7, 0x9b, 0xf0), rgb(0x72, 0xd3, 0xde), rgb(0xff, 0xff, 0xff),
}

// lightANSI reads on Light's white field: deep hues, and "white" a gray.
var lightANSI = [16]color.Color{
	rgb(0x24, 0x29, 0x2f), rgb(0xcf, 0x22, 0x2e), rgb(0x11, 0x63, 0x29), rgb(0x9a, 0x67, 0x00),
	rgb(0x05, 0x50, 0xae), rgb(0x82, 0x50, 0xdf), rgb(0x1b, 0x7c, 0x83), rgb(0x6e, 0x77, 0x81),
	rgb(0x57, 0x60, 0x6a), rgb(0xa4, 0x0e, 0x26), rgb(0x1a, 0x7f, 0x37), rgb(0x7d, 0x4e, 0x00),
	rgb(0x09, 0x69, 0xda), rgb(0xa4, 0x75, 0xf9), rgb(0x31, 0x92, 0xaa), rgb(0x8c, 0x95, 0x9f),
}

// Dark is the default dark theme.
func Dark() Theme {
	return Theme{
		Background: rgb(0x1e, 0x1f, 0x22), Surface: rgb(0x29, 0x2a, 0x2e),
		Text: rgb(0xde, 0xe0, 0xe5), Subtle: rgb(0x8d, 0x91, 0x9b), Title: rgb(0xf2, 0xf3, 0xf5),
		Border: rgb(0x4c, 0x4e, 0x55), Field: rgb(0x17, 0x18, 0x1b),
		Accent: rgb(0x66, 0xa7, 0xff), Selection: color.NRGBA{0x66, 0xa7, 0xff, 0x38},
		Track:    rgb(0x3f, 0x41, 0x47),
		FontSize: 13, TitleSize: 15, LineSpacing: 1.25, Radius: 4, ControlHeight: 32,
		ANSI: darkANSI,
	}
}

// Light is the default light theme.
func Light() Theme {
	return Theme{
		Background: rgb(0xf6, 0xf6, 0xf7), Surface: rgb(0xff, 0xff, 0xff),
		Text: rgb(0x1d, 0x1d, 0x1f), Subtle: rgb(0x6e, 0x6e, 0x73), Title: rgb(0x0b, 0x0b, 0x0d),
		Border: rgb(0xc9, 0xc9, 0xce), Field: rgb(0xff, 0xff, 0xff),
		Accent: rgb(0x00, 0x6e, 0xe6), Selection: color.NRGBA{0x00, 0x6e, 0xe6, 0x2e},
		Track:    rgb(0xdd, 0xdd, 0xe1),
		FontSize: 13, TitleSize: 15, LineSpacing: 1.25, Radius: 4, ControlHeight: 32,
		ANSI: lightANSI,
	}
}

func rgb(r, g, b uint8) color.Color { return color.RGBA{r, g, b, 0xff} }

// over returns t with its zero fields taken from base.
func (t Theme) over(base Theme) Theme {
	pick := func(c, b color.Color) color.Color {
		if c == nil {
			return b
		}
		return c
	}
	num := func(v, b float64) float64 {
		if v == 0 {
			return b
		}
		return v
	}
	str := func(v, b string) string {
		if v == "" {
			return b
		}
		return v
	}
	var pal [16]color.Color
	for i := range pal {
		pal[i] = pick(t.ANSI[i], base.ANSI[i])
	}
	return Theme{
		ANSI:       pal,
		Background: pick(t.Background, base.Background), Surface: pick(t.Surface, base.Surface),
		Text: pick(t.Text, base.Text), Subtle: pick(t.Subtle, base.Subtle), Title: pick(t.Title, base.Title),
		Border: pick(t.Border, base.Border), Field: pick(t.Field, base.Field),
		Accent: pick(t.Accent, base.Accent), Selection: pick(t.Selection, base.Selection),
		Track: pick(t.Track, base.Track),
		Font:  str(t.Font, base.Font), MonoFont: str(t.MonoFont, base.MonoFont),
		FontSize: num(t.FontSize, base.FontSize), TitleSize: num(t.TitleSize, base.TitleSize),
		LineSpacing: num(t.LineSpacing, base.LineSpacing), Radius: num(t.Radius, base.Radius),
		ControlHeight: num(t.ControlHeight, base.ControlHeight),
	}
}

// themeFor is the theme a window draws with: opts' theme over Dark, or —
// with none — the system's appearance (dark reports it).
func themeFor(opts *Theme, dark bool) Theme {
	if opts != nil {
		return opts.over(Dark())
	}
	if dark {
		return Dark()
	}
	return Light()
}

// fillColor is t's color for a Box.Fill role; nil for FillNone and roles
// it does not know.
func (t Theme) fillColor(f layout.Fill) color.Color {
	switch f {
	case layout.FillSurface:
		return t.Surface
	case layout.FillField:
		return t.Field
	case layout.FillSeparator:
		return t.Border
	}
	return nil
}

// appearance is light or dark by the Background's relative luminance, so a
// custom theme reports what it looks like, not which base it started from.
func (t Theme) appearance() flatte.Appearance {
	bg := t.Background
	if bg == nil {
		bg = Dark().Background
	}
	r, g, b, _ := bg.RGBA()
	lin := func(c uint32) float64 {
		v := float64(c) / 0xffff
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	if 0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b) > 0.18 {
		return flatte.AppearanceLight
	}
	return flatte.AppearanceDark
}
