package window

import (
	"image/color"
	"math"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
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
	// MonoSize is the cell grids' (code's) size in points, apart from the
	// interface text: columns are its fixed-pitch advance and lines its
	// pitch. Zero follows FontSize.
	MonoSize float64
	// TerminalFont, TerminalSize and TerminalLine are the terminal face's
	// (layout.FaceTerminal) family, size and line height in points: its
	// cells are the family's fixed-pitch advance at the size by the line.
	// Empty or zero follow MonoFont, the code size, and the line the code
	// grid would have at the size.
	TerminalFont               string
	TerminalSize, TerminalLine float64
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

	// design is the design system this Theme was made from (fromDesign):
	// the palette layout's newer roles resolve in, and its type scale.
	design *ds.Theme
	dark   bool
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

// Dark is the default dark theme: theme.Default()'s dark palette.
func Dark() Theme { return fromDesign(&defaultDesign, true) }

// Light is the default light theme: theme.Default()'s light palette.
func Light() Theme { return fromDesign(&defaultDesign, false) }

var defaultDesign = ds.Default()

// fromDesign is the window Theme for design's palette in an appearance.
// The window ground is Chrome (an app that fills nothing reads as a
// sidebar would); Surface is Raised; Subtle is TextSecondary; Border is
// the control outline (separators use the Divider through FillSeparator).
func fromDesign(design *ds.Theme, dark bool) Theme {
	p := design.Palette(dark)
	var pal [16]color.Color
	for i, c := range p.ANSI {
		pal[i] = c
	}
	radius := design.Radius.Sm
	if radius <= 0 {
		radius = -1 // square
	}
	return Theme{
		Background: p.Chrome, Surface: p.Raised,
		Text: p.Text, Subtle: p.TextSecondary, Title: p.Text,
		Border: p.ControlBorder, Field: p.Field,
		Accent: p.Accent, Selection: p.SelectionActive, Track: p.SelectionInactive,
		Font: design.Font, MonoFont: design.MonoFont,
		FontSize: design.Type.Body.Size, TitleSize: design.Type.Title.Size, LineSpacing: 1.25,
		MonoSize:     design.Type.Code.Size,
		TerminalFont: design.TerminalFont, TerminalSize: design.Type.Terminal.Size, TerminalLine: design.Type.Terminal.Line,
		Radius: radius, ControlHeight: design.Size.Control,
		ANSI:   pal,
		design: design, dark: dark,
	}
}

// palette is the design palette t's roles resolve in.
func (t Theme) palette() *ds.Palette {
	if t.design == nil {
		return defaultDesign.Palette(t.appearance() == flatte.AppearanceDark)
	}
	return t.design.Palette(t.dark)
}

// typeScale is the design type scale t's text roles are set in.
func (t Theme) typeScale() ds.TypeScale {
	if t.design == nil {
		return defaultDesign.Type
	}
	return t.design.Type
}

// spacing is the layout step (one Pad or Gap), the default component
// insets and the token sizes in points, from the design's scales: step
// space-2, control and item insets space-1 by space-2, panels space-3.
func (t Theme) spacing() (step float64, in layout.Insets, sizes layout.Sizes) {
	d := t.design
	if d == nil {
		d = &defaultDesign
	}
	sp := d.Space
	step = sp[1]
	if step <= 0 {
		step = stepPoints
	}
	z := d.Size
	return step, layout.Insets{Vertical: sp[0], Horizontal: sp[1], Panel: sp[2]}, layout.Sizes{Row: z.Row, Control: z.Control, Bar: z.Bar, Header: z.Header}
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
	design, dark := t.design, t.dark
	if design == nil {
		design, dark = base.design, base.dark
	}
	return Theme{
		design: design, dark: dark,
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
		MonoSize:      t.monoOver(base),
		TerminalFont:  str(t.TerminalFont, base.TerminalFont),
		TerminalSize:  num(t.TerminalSize, base.TerminalSize), TerminalLine: num(t.TerminalLine, base.TerminalLine),
	}
}

// monoOver is t's MonoSize over base: its own, else — when t sets
// FontSize, which sized the grid before MonoSize — FontSize's (0), else
// base's.
func (t Theme) monoOver(base Theme) float64 {
	switch {
	case t.MonoSize > 0:
		return t.MonoSize
	case t.FontSize > 0:
		return 0
	}
	return base.MonoSize
}

// monoSize is the grid's size: MonoSize, else FontSize.
func (t Theme) monoSize() float64 {
	if t.MonoSize > 0 {
		return t.MonoSize
	}
	return t.FontSize
}

// terminalSize is the terminal face's size: TerminalSize, else the grid's.
func (t Theme) terminalSize() float64 {
	if t.TerminalSize > 0 && !math.IsInf(t.TerminalSize, 1) {
		return t.TerminalSize
	}
	return t.monoSize()
}

// terminalLine is the terminal face's line height when set, on whole
// points (cells stay on the pixel grid as code's pitch does); 0 for the
// grid's pitch at its size.
func (t Theme) terminalLine() float64 {
	if t.TerminalLine > 0 && !math.IsInf(t.TerminalLine, 1) {
		return max(math.Round(t.TerminalLine), 1)
	}
	return 0
}

// themeFor is the theme a window draws with: opts' theme over Dark, or —
// with none — design (the app's flatte.WithTheme, else theme.Default())
// in the system's appearance (dark reports it).
func themeFor(opts *Theme, dark bool, design *ds.Theme) Theme {
	if design == nil {
		design = &defaultDesign
	}
	if opts != nil {
		// The app's design in opts' appearance under it: Light() over an
		// app's own design is that design's light palette, not its dark.
		o, look := *opts, true
		if o.design != nil {
			look = o.dark
		}
		if o.design == &defaultDesign {
			o.design = nil // the app's design, for its sizes and roles
		}
		return o.over(fromDesign(design, look))
	}
	return fromDesign(design, dark)
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
		return t.palette().Divider
	}
	if c, ok := layout.FillColor(t.palette(), f); ok {
		return c
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
