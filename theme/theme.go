// Package theme is Flatte's design system as data: the colors of a light
// and a dark palette, and the type, spacing, size, radius and stroke
// scales every component shares. Renderers resolve layout's roles through
// it — a window in points, a terminal in cells — so an app that names
// roles (layout.Fill, layout.TextStyle.Color) looks finished without
// choosing a color or a size.
//
// Most apps use Default. An app with an identity turns the seeds — accent,
// density, roundness — with With; any single token can then be set on the
// Theme value directly. Check reports text and controls that no longer
// reach their contrast floor after such changes.
package theme

import "image/color"

// Theme is a light and a dark palette and the scales both share.
type Theme struct {
	Light, Dark Palette

	// Type is the type scale.
	Type TypeScale
	// Space is the spacing scale, Space[0] … Space[5] in points (space-1 …
	// space-6 in the design docs): 4, 8, 12, 16, 24, 32 by default.
	Space [6]float64
	// Size holds the fixed heights every component shares.
	Size Sizes
	// Radius is the corner scale.
	Radius Radii
	// Stroke is the line widths.
	Stroke Strokes
	// Fonts are the UI and mono families ("": the system's).
	Font, MonoFont string
}

// Palette is one appearance's colors. Surfaces hold content; Text* read on
// every surface unless noted; washes (Hover, Pressed) are translucent and
// go over whatever surface is below.
type Palette struct {
	// Canvas is the window ground behind the panes; it holds no text.
	Canvas color.NRGBA
	// Chrome is headers, sidebars, pane headers and the status bar — the
	// translucency material in a window.
	Chrome color.NRGBA
	// Content is where work is read: editors, terminals, diffs, lists,
	// pages.
	Content color.NRGBA
	// Field is text fields and segmented-control tracks.
	Field color.NRGBA
	// Raised is popovers, menus and alerts (always with Shadow).
	Raised color.NRGBA

	Text          color.NRGBA // body text, names, values
	TextSecondary color.NRGBA // paths, descriptions, metadata, inactive tabs
	TextMuted     color.NRGBA // gutters, captions, placeholders: on Chrome and Content only
	TextDisabled  color.NRGBA // disabled controls (no contrast floor)
	OnAccent      color.NRGBA // text and icons on an Accent fill

	Accent      color.NRGBA // the action color: primary buttons, active-tab edge, links
	AccentHover color.NRGBA // a primary button under the pointer
	Focus       color.NRGBA // the keyboard focus ring
	Hover       color.NRGBA // translucent wash under the pointer
	Pressed     color.NRGBA // translucent wash while pressed, or toggled on

	SelectionActive   color.NRGBA // the selected row in the focused pane
	SelectionInactive color.NRGBA // the selected row in a pane without focus
	TextSelection     color.NRGBA // selected text
	Match             color.NRGBA // a find or quick-open match, behind text
	CurrentLine       color.NRGBA // the caret's line in an editor

	Divider       color.NRGBA // pane separators and header rules
	ControlBorder color.NRGBA // field and toggle outlines

	Info, InfoTint       color.NRGBA // working, informational
	Success, SuccessTint color.NRGBA // ready, passed, added
	Warning, WarningTint color.NRGBA // needs the user
	Danger, DangerTint   color.NRGBA // errors, removed, destructive

	Diff   DiffColors
	Syntax SyntaxColors

	// Shadow is the one elevation (popovers): its color; the geometry is
	// the renderer's (a terminal draws a border instead).
	Shadow color.NRGBA

	// ANSI is the 16-color palette embedded terminals draw program output
	// in (0–7 normal, 8–15 bright).
	ANSI [16]color.NRGBA
}

// DiffColors are a diff's line and word backgrounds.
type DiffColors struct {
	Add, AddWord, Del, DelWord color.NRGBA
	Fold, Blank                color.NRGBA
}

// SyntaxColors are code highlighting colors, checked against Content,
// CurrentLine and both diff line fills.
type SyntaxColors struct {
	Keyword, String, Comment, Number, Type, Func color.NRGBA
}

// TypeScale is the text styles' size and line height in points and weight
// (400 regular, 600 semibold).
type TypeScale struct {
	PageTitle, Title, Body, Meta, Caption, Code TextSpec
}

// TextSpec is one type style.
type TextSpec struct {
	Size, Line float64
	Weight     int
	// Tracking is letter spacing in ems (captions).
	Tracking float64
}

// Sizes are the heights components share, in points.
type Sizes struct {
	Row     float64 // list, tree, quick-open and tab rows
	Control float64 // buttons, fields, segmented controls
	Bar     float64 // pane headers, tab strips, toolbars
	Header  float64 // the window header strip
	Icon    float64 // icons in rows, tabs and buttons
	Popover float64 // the widest a centered popover gets
}

// Radii are corner radii in points.
type Radii struct {
	Sm float64 // rows, tabs, badges
	Md float64 // buttons, fields, toggles, segmented controls
	Lg float64 // popovers, alerts
}

// Strokes are line widths in points.
type Strokes struct {
	Divider, Control, Focus float64
}

// Palette is the palette for an appearance.
func (t *Theme) Palette(dark bool) *Palette {
	if dark {
		return &t.Dark
	}
	return &t.Light
}

// Default is Flatte's theme: neutral surfaces with a slight blue bias, a
// blue accent, regular density and subtly rounded corners.
func Default() Theme {
	return Theme{
		Light: light(),
		Dark:  dark(),
		Type: TypeScale{
			PageTitle: TextSpec{Size: 20, Line: 26, Weight: 600},
			Title:     TextSpec{Size: 15, Line: 22, Weight: 600},
			Body:      TextSpec{Size: 13, Line: 19, Weight: 400},
			Meta:      TextSpec{Size: 12, Line: 16, Weight: 400},
			Caption:   TextSpec{Size: 11, Line: 16, Weight: 600, Tracking: 0.04},
			Code:      TextSpec{Size: 13, Line: 19, Weight: 400},
		},
		Space:  regularSpace,
		Size:   regularSizes,
		Radius: subtleRadii,
		Stroke: Strokes{Divider: 1, Control: 1, Focus: 2},
	}
}

func light() Palette {
	return Palette{
		Canvas: Hex("#e9ecf0"), Chrome: Hex("#f4f5f7"), Content: Hex("#ffffff"), Field: Hex("#ffffff"), Raised: Hex("#ffffff"),
		Text: Hex("#1b1f26"), TextSecondary: Hex("#4f5765"), TextMuted: Hex("#636a76"), TextDisabled: Hex("#a1a7b1"), OnAccent: Hex("#ffffff"),
		Accent: Hex("#1f5fd1"), AccentHover: Hex("#1a52b5"), Focus: Hex("#1f5fd1"),
		Hover: color.NRGBA{27, 31, 38, 13}, Pressed: color.NRGBA{27, 31, 38, 26},
		SelectionActive: Hex("#d6e3fb"), SelectionInactive: Hex("#e3e6eb"), TextSelection: Hex("#b9d0f7"),
		Match: Hex("#f8e39b"), CurrentLine: Hex("#f3f6fb"),
		Divider: Hex("#dfe2e7"), ControlBorder: Hex("#838a97"),
		Info: Hex("#1f5fd1"), InfoTint: Hex("#e3ecfc"),
		Success: Hex("#17732f"), SuccessTint: Hex("#dcf2e2"),
		Warning: Hex("#975e00"), WarningTint: Hex("#fbeed3"),
		Danger: Hex("#c4302b"), DangerTint: Hex("#fce4e2"),
		Diff:   DiffColors{Add: Hex("#e3f5e6"), AddWord: Hex("#a9dfb3"), Del: Hex("#fde8e7"), DelWord: Hex("#f6b9b5"), Fold: Hex("#eef1f5"), Blank: Hex("#f5f6f8")},
		Syntax: SyntaxColors{Keyword: Hex("#7c3aed"), String: Hex("#1f7a3d"), Comment: Hex("#646b77"), Number: Hex("#a35200"), Type: Hex("#0b6e99"), Func: Hex("#1f5fd1")},
		Shadow: color.NRGBA{16, 24, 40, 41},
		ANSI: [16]color.NRGBA{
			Hex("#24292f"), Hex("#cf222e"), Hex("#116329"), Hex("#9a6700"), Hex("#0550ae"), Hex("#8250df"), Hex("#1b7c83"), Hex("#6e7781"),
			Hex("#57606a"), Hex("#a40e26"), Hex("#1a7f37"), Hex("#7d4e00"), Hex("#0969da"), Hex("#a475f9"), Hex("#3192aa"), Hex("#8c959f"),
		},
	}
}

func dark() Palette {
	return Palette{
		Canvas: Hex("#121418"), Chrome: Hex("#1a1d22"), Content: Hex("#1f2228"), Field: Hex("#262a31"), Raised: Hex("#2a2e36"),
		Text: Hex("#e6e8ec"), TextSecondary: Hex("#a7adb7"), TextMuted: Hex("#8f96a1"), TextDisabled: Hex("#5d636d"), OnAccent: Hex("#0b1220"),
		Accent: Hex("#6ea2ff"), AccentHover: Hex("#8bb5ff"), Focus: Hex("#8bb5ff"),
		Hover: color.NRGBA{255, 255, 255, 15}, Pressed: color.NRGBA{255, 255, 255, 26},
		SelectionActive: Hex("#24406b"), SelectionInactive: Hex("#2c3139"), TextSelection: Hex("#2f5390"),
		Match: Hex("#5a4812"), CurrentLine: Hex("#252a32"),
		Divider: Hex("#2c3037"), ControlBorder: Hex("#6b7380"),
		Info: Hex("#6ea2ff"), InfoTint: Hex("#1c2b44"),
		Success: Hex("#4cc06a"), SuccessTint: Hex("#17321f"),
		Warning: Hex("#e3a83b"), WarningTint: Hex("#382b11"),
		Danger: Hex("#ff7b72"), DangerTint: Hex("#3d1f1f"),
		Diff:   DiffColors{Add: Hex("#1d3326"), AddWord: Hex("#2f6440"), Del: Hex("#3a2326"), DelWord: Hex("#6e2f37"), Fold: Hex("#262a31"), Blank: Hex("#1b1e23")},
		Syntax: SyntaxColors{Keyword: Hex("#c792ea"), String: Hex("#98c379"), Comment: Hex("#8f96a1"), Number: Hex("#e5a66b"), Type: Hex("#6cc5d9"), Func: Hex("#82aaff")},
		Shadow: color.NRGBA{0, 0, 0, 128},
		ANSI: [16]color.NRGBA{
			Hex("#545862"), Hex("#e06c75"), Hex("#98c379"), Hex("#e5c07b"), Hex("#61afef"), Hex("#c678dd"), Hex("#56b6c2"), Hex("#dcdfe4"),
			Hex("#7f848e"), Hex("#ff7b86"), Hex("#b5e890"), Hex("#f0d58a"), Hex("#7cc4ff"), Hex("#d79bf0"), Hex("#72d3de"), Hex("#ffffff"),
		},
	}
}
