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
	// TerminalFont is embedded terminals' family (layout.FaceTerminal;
	// "": MonoFont).
	TerminalFont string
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

// SyntaxColors give code tokens their own colors. Every field is checked
// against Content, CurrentLine and both diff line fills.
type SyntaxColors struct {
	Keyword      color.NRGBA // language keywords
	String       color.NRGBA // strings and characters
	Comment      color.NRGBA // comments
	Number       color.NRGBA // numeric literals
	Type         color.NRGBA // types and constructors
	Func         color.NRGBA // function declarations
	Control      color.NRGBA // control-flow keywords
	Method       color.NRGBA // methods and calls
	Variable     color.NRGBA // local and global variables
	Parameter    color.NRGBA // function parameters
	Property     color.NRGBA // fields and properties
	Builtin      color.NRGBA // built-in names
	Constant     color.NRGBA // named and boolean constants
	Escape       color.NRGBA // escapes and regular expressions
	DocComment   color.NRGBA // documentation comments
	Operator     color.NRGBA // operators
	Punctuation  color.NRGBA // brackets and punctuation
	Tag          color.NRGBA // markup tags
	Attribute    color.NRGBA // markup attributes
	Preprocessor color.NRGBA // preprocessor directives
	Annotation   color.NRGBA // annotations and decorators
	Heading      color.NRGBA // markup headings
	Link         color.NRGBA // links
}

// TypeScale is the text styles' size and line height in points and weight
// (400 regular, 600 semibold).
type TypeScale struct {
	PageTitle, Title, Body, Meta, Caption, Code TextSpec
	// Terminal is embedded terminals' type (layout.FaceTerminal): a zero
	// Size is Code's size, and a zero Line the renderer's line for the
	// size, as code's.
	Terminal TextSpec
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

// Default is Flatte's theme: warm stone-grey surfaces, a vivid
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

type paletteColorPair struct {
	light color.NRGBA
	dark  color.NRGBA
}

func paletteHexPair(light, dark string) paletteColorPair {
	return paletteColorPair{light: Hex(light), dark: Hex(dark)}
}

func (p paletteColorPair) forAppearance(dark bool) color.NRGBA {
	if dark {
		return p.dark
	}
	return p.light
}

type paletteTable struct {
	Canvas, Chrome, Content, Field, Raised                                         paletteColorPair
	Text, TextSecondary, TextMuted, TextDisabled, OnAccent                         paletteColorPair
	Accent, AccentHover, Focus                                                     paletteColorPair
	Hover, Pressed                                                                 paletteColorPair
	SelectionActive, SelectionInactive, TextSelection                              paletteColorPair
	Match, CurrentLine                                                             paletteColorPair
	Divider, ControlBorder                                                         paletteColorPair
	Info, InfoTint, Success, SuccessTint, Warning, WarningTint, Danger, DangerTint paletteColorPair
	Diff                                                                           struct {
		Add, AddWord, Del, DelWord, Fold, Blank paletteColorPair
	}
	Syntax struct {
		Keyword, Control, Type, Func, Method       paletteColorPair
		Variable, Parameter, Property, Builtin     paletteColorPair
		Constant, Number, String, Escape           paletteColorPair
		Comment, DocComment, Operator, Punctuation paletteColorPair
		Tag, Attribute, Preprocessor, Annotation   paletteColorPair
		Heading, Link                              paletteColorPair
	}
	Shadow paletteColorPair
	ANSI   [16]paletteColorPair
}

var defaultPalette = paletteTable{
	Canvas: paletteHexPair("#ebe9e5", "#141311"), Chrome: paletteHexPair("#f6f5f2", "#1c1b18"), Content: paletteHexPair("#fefdfb", "#21201d"), Field: paletteHexPair("#ffffff", "#2a2825"), Raised: paletteHexPair("#ffffff", "#2f2d29"),
	Text: paletteHexPair("#1f1d1a", "#ece9e3"), TextSecondary: paletteHexPair("#57534c", "#b1aba1"), TextMuted: paletteHexPair("#6b665e", "#999288"), TextDisabled: paletteHexPair("#aaa49b", "#645f57"), OnAccent: paletteHexPair("#ffffff", "#0b1220"),
	Accent: paletteHexPair("#1f5fd1", "#6ea2ff"), AccentHover: paletteHexPair("#1a52b5", "#8bb5ff"), Focus: paletteHexPair("#1f5fd1", "#8bb5ff"),
	Hover: paletteColorPair{light: color.NRGBA{40, 32, 24, 13}, dark: color.NRGBA{255, 246, 232, 15}}, Pressed: paletteColorPair{light: color.NRGBA{40, 32, 24, 26}, dark: color.NRGBA{255, 246, 232, 26}},
	SelectionActive: paletteHexPair("#d6e3fb", "#24406b"), SelectionInactive: paletteHexPair("#dcd8d0", "#34312c"), TextSelection: paletteHexPair("#b9d0f7", "#2f5390"),
	Match: paletteHexPair("#f8e39b", "#5a4812"), CurrentLine: paletteHexPair("#f7f4ee", "#292723"),
	Divider: paletteHexPair("#e3dfd8", "#322f2a"), ControlBorder: paletteHexPair("#8a8378", "#7a7469"),
	Info: paletteHexPair("#1f5fd1", "#6ea2ff"), InfoTint: paletteHexPair("#e3ecfc", "#1c2b44"),
	Success: paletteHexPair("#17732f", "#4cc06a"), SuccessTint: paletteHexPair("#dcf2e2", "#17321f"),
	Warning: paletteHexPair("#975e00", "#e3a83b"), WarningTint: paletteHexPair("#fbeed3", "#382b11"),
	Danger: paletteHexPair("#c4302b", "#ff7b72"), DangerTint: paletteHexPair("#fce4e2", "#3d1f1f"),
	Diff: struct {
		Add, AddWord, Del, DelWord, Fold, Blank paletteColorPair
	}{Add: paletteHexPair("#e3f5e6", "#1d3326"), AddWord: paletteHexPair("#a9dfb3", "#2f6440"), Del: paletteHexPair("#fde8e7", "#3a2326"), DelWord: paletteHexPair("#f6b9b5", "#6e2f37"), Fold: paletteHexPair("#f0ede7", "#2a2825"), Blank: paletteHexPair("#f6f4f0", "#1d1c19")},
	Syntax: struct {
		Keyword, Control, Type, Func, Method       paletteColorPair
		Variable, Parameter, Property, Builtin     paletteColorPair
		Constant, Number, String, Escape           paletteColorPair
		Comment, DocComment, Operator, Punctuation paletteColorPair
		Tag, Attribute, Preprocessor, Annotation   paletteColorPair
		Heading, Link                              paletteColorPair
	}{
		Keyword: paletteHexPair("#6f42c1", "#c792ea"), Control: paletteHexPair("#a626a4", "#ff7ab2"), Type: paletteHexPair("#005a7a", "#6cc5d9"), Func: paletteHexPair("#1f5fd1", "#82aaff"), Method: paletteHexPair("#006b66", "#56b6c2"),
		Variable: paletteHexPair("#3e566b", "#b1c7d3"), Parameter: paletteHexPair("#775a11", "#e3b341"), Property: paletteHexPair("#865b00", "#e5c07b"), Builtin: paletteHexPair("#743f8c", "#d29ad6"),
		Constant: paletteHexPair("#b03a2e", "#ff7b86"), Number: paletteHexPair("#985500", "#e5a66b"), String: paletteHexPair("#1b6e37", "#98c379"), Escape: paletteHexPair("#b42336", "#ff8a65"),
		Comment: paletteHexPair("#6b665e", "#a09a90"), DocComment: paletteHexPair("#426552", "#7cbf9e"), Operator: paletteHexPair("#50545a", "#c0b6a5"), Punctuation: paletteHexPair("#8a8378", "#938d82"),
		Tag: paletteHexPair("#8a2847", "#ff8f8f"), Attribute: paletteHexPair("#714f13", "#e2a4d6"), Preprocessor: paletteHexPair("#585c18", "#c7c963"), Annotation: paletteHexPair("#585b9e", "#d2c7ff"),
		Heading: paletteHexPair("#00615c", "#58c6bd"), Link: paletteHexPair("#1750a4", "#7eb8e8"),
	},
	Shadow: paletteColorPair{light: color.NRGBA{40, 28, 16, 41}, dark: color.NRGBA{0, 0, 0, 128}},
	ANSI: [16]paletteColorPair{
		paletteHexPair("#24292f", "#545862"), paletteHexPair("#cf222e", "#e06c75"), paletteHexPair("#116329", "#98c379"), paletteHexPair("#9a6700", "#e5c07b"),
		paletteHexPair("#0550ae", "#61afef"), paletteHexPair("#8250df", "#c678dd"), paletteHexPair("#1b7c83", "#56b6c2"), paletteHexPair("#6e7781", "#dcdfe4"),
		paletteHexPair("#57606a", "#7f848e"), paletteHexPair("#a40e26", "#ff7b86"), paletteHexPair("#1a7f37", "#b5e890"), paletteHexPair("#7d4e00", "#f0d58a"),
		paletteHexPair("#0969da", "#7cc4ff"), paletteHexPair("#a475f9", "#d79bf0"), paletteHexPair("#3192aa", "#72d3de"), paletteHexPair("#858e98", "#ffffff"),
	},
}

func (t paletteTable) build(dark bool) Palette {
	return Palette{
		Canvas: t.Canvas.forAppearance(dark), Chrome: t.Chrome.forAppearance(dark), Content: t.Content.forAppearance(dark), Field: t.Field.forAppearance(dark), Raised: t.Raised.forAppearance(dark),
		Text: t.Text.forAppearance(dark), TextSecondary: t.TextSecondary.forAppearance(dark), TextMuted: t.TextMuted.forAppearance(dark), TextDisabled: t.TextDisabled.forAppearance(dark), OnAccent: t.OnAccent.forAppearance(dark),
		Accent: t.Accent.forAppearance(dark), AccentHover: t.AccentHover.forAppearance(dark), Focus: t.Focus.forAppearance(dark),
		Hover: t.Hover.forAppearance(dark), Pressed: t.Pressed.forAppearance(dark),
		SelectionActive: t.SelectionActive.forAppearance(dark), SelectionInactive: t.SelectionInactive.forAppearance(dark), TextSelection: t.TextSelection.forAppearance(dark),
		Match: t.Match.forAppearance(dark), CurrentLine: t.CurrentLine.forAppearance(dark),
		Divider: t.Divider.forAppearance(dark), ControlBorder: t.ControlBorder.forAppearance(dark),
		Info: t.Info.forAppearance(dark), InfoTint: t.InfoTint.forAppearance(dark),
		Success: t.Success.forAppearance(dark), SuccessTint: t.SuccessTint.forAppearance(dark),
		Warning: t.Warning.forAppearance(dark), WarningTint: t.WarningTint.forAppearance(dark),
		Danger: t.Danger.forAppearance(dark), DangerTint: t.DangerTint.forAppearance(dark),
		Diff: DiffColors{Add: t.Diff.Add.forAppearance(dark), AddWord: t.Diff.AddWord.forAppearance(dark), Del: t.Diff.Del.forAppearance(dark), DelWord: t.Diff.DelWord.forAppearance(dark), Fold: t.Diff.Fold.forAppearance(dark), Blank: t.Diff.Blank.forAppearance(dark)},
		Syntax: SyntaxColors{
			Keyword: t.Syntax.Keyword.forAppearance(dark), Control: t.Syntax.Control.forAppearance(dark), Type: t.Syntax.Type.forAppearance(dark), Func: t.Syntax.Func.forAppearance(dark), Method: t.Syntax.Method.forAppearance(dark),
			Variable: t.Syntax.Variable.forAppearance(dark), Parameter: t.Syntax.Parameter.forAppearance(dark), Property: t.Syntax.Property.forAppearance(dark), Builtin: t.Syntax.Builtin.forAppearance(dark),
			Constant: t.Syntax.Constant.forAppearance(dark), Number: t.Syntax.Number.forAppearance(dark), String: t.Syntax.String.forAppearance(dark), Escape: t.Syntax.Escape.forAppearance(dark),
			Comment: t.Syntax.Comment.forAppearance(dark), DocComment: t.Syntax.DocComment.forAppearance(dark), Operator: t.Syntax.Operator.forAppearance(dark), Punctuation: t.Syntax.Punctuation.forAppearance(dark),
			Tag: t.Syntax.Tag.forAppearance(dark), Attribute: t.Syntax.Attribute.forAppearance(dark), Preprocessor: t.Syntax.Preprocessor.forAppearance(dark), Annotation: t.Syntax.Annotation.forAppearance(dark),
			Heading: t.Syntax.Heading.forAppearance(dark), Link: t.Syntax.Link.forAppearance(dark),
		},
		Shadow: t.Shadow.forAppearance(dark),
		ANSI: [16]color.NRGBA{
			t.ANSI[0].forAppearance(dark), t.ANSI[1].forAppearance(dark), t.ANSI[2].forAppearance(dark), t.ANSI[3].forAppearance(dark),
			t.ANSI[4].forAppearance(dark), t.ANSI[5].forAppearance(dark), t.ANSI[6].forAppearance(dark), t.ANSI[7].forAppearance(dark),
			t.ANSI[8].forAppearance(dark), t.ANSI[9].forAppearance(dark), t.ANSI[10].forAppearance(dark), t.ANSI[11].forAppearance(dark),
			t.ANSI[12].forAppearance(dark), t.ANSI[13].forAppearance(dark), t.ANSI[14].forAppearance(dark), t.ANSI[15].forAppearance(dark),
		},
	}
}

func light() Palette { return defaultPalette.build(false) }

func dark() Palette { return defaultPalette.build(true) }
