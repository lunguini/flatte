package window

import "image/color"

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
	// Radius is a plain border's corner radius in points (a rounded border
	// is twice as round); negative draws square corners.
	Radius float64
}

// Dark is the default dark theme.
func Dark() Theme {
	return Theme{
		Background: rgb(0x1e, 0x1f, 0x22), Surface: rgb(0x29, 0x2a, 0x2e),
		Text: rgb(0xde, 0xe0, 0xe5), Subtle: rgb(0x8d, 0x91, 0x9b), Title: rgb(0xf2, 0xf3, 0xf5),
		Border: rgb(0x4c, 0x4e, 0x55), Field: rgb(0x17, 0x18, 0x1b),
		Accent: rgb(0x66, 0xa7, 0xff), Selection: color.NRGBA{0x66, 0xa7, 0xff, 0x38},
		Track:    rgb(0x3f, 0x41, 0x47),
		FontSize: 13, TitleSize: 15, LineSpacing: 1.25, Radius: 4,
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
		FontSize: 13, TitleSize: 15, LineSpacing: 1.25, Radius: 4,
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
	return Theme{
		Background: pick(t.Background, base.Background), Surface: pick(t.Surface, base.Surface),
		Text: pick(t.Text, base.Text), Subtle: pick(t.Subtle, base.Subtle), Title: pick(t.Title, base.Title),
		Border: pick(t.Border, base.Border), Field: pick(t.Field, base.Field),
		Accent: pick(t.Accent, base.Accent), Selection: pick(t.Selection, base.Selection),
		Track: pick(t.Track, base.Track),
		Font:  str(t.Font, base.Font), MonoFont: str(t.MonoFont, base.MonoFont),
		FontSize: num(t.FontSize, base.FontSize), TitleSize: num(t.TitleSize, base.TitleSize),
		LineSpacing: num(t.LineSpacing, base.LineSpacing), Radius: num(t.Radius, base.Radius),
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
