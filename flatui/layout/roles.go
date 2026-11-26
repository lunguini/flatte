package layout

import (
	"image/color"
	"strings"
	"sync/atomic"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/theme"
)

// TextColor is a text color by its role in the theme (TextStyle.Color).
type TextColor int

const (
	// ColorDefault is the text role's own color.
	ColorDefault TextColor = iota
	ColorPrimary
	ColorSecondary
	ColorMuted
	ColorDisabled
	ColorAccent
	// ColorOnAccent is text on a FillAccent box.
	ColorOnAccent
	ColorInfo
	ColorSuccess
	ColorWarning
	ColorDanger
)

// The fills below follow FillSeparator; see theme.Palette for each one's
// purpose.
const (
	// FillCanvas is the window ground behind panes.
	FillCanvas Fill = iota + FillSeparator + 1
	// FillChrome is headers, sidebars, pane headers and status bars.
	FillChrome
	// FillContent is where work is read: editors, terminals, lists, pages.
	FillContent
	// FillRaised is popovers and menus.
	FillRaised
	// FillAccent is a primary button or an accent badge (text on it:
	// ColorOnAccent).
	FillAccent
	// FillSelection is the selected row in the focused pane.
	FillSelection
	// FillSelectionInactive is the selected row in a pane without focus.
	FillSelectionInactive
	// FillPressed is a pressed or toggled-on control's wash.
	FillPressed
	FillInfoTint
	FillSuccessTint
	FillWarningTint
	FillDangerTint
	FillCurrentLine
	FillMatch
	FillDiffAdd
	FillDiffAddWord
	FillDiffDel
	FillDiffDelWord
	FillDiffFold
	FillDiffBlank
	// FillGlass is white glass for a control that sits on a translucent
	// window's chrome (a side bar's buttons, a title bar field), so it
	// reads the same whatever shows through: white at GlassLight on a
	// light palette, a faint white wash at GlassDark on a dark one.
	FillGlass
	// FillBorder is a control's border color: a neutral mark that does not
	// read as focus (an unfocused pane's active tab edge).
	FillBorder
	// FillShade is a wash of black for a pane's body, so the pane and the
	// header on it stand out from the window ground: it darkens what is
	// under it, a translucent window's blur too, by ShadeLight or ShadeDark.
	FillShade
)

// FillGlass's white opacities: 80% on a light palette, 8% on a dark one.
const (
	GlassLight = 0xcc
	GlassDark  = 0x14
)

// FillShade's black opacities: 5% on a light palette, 50% on a dark one.
const (
	ShadeLight = 0x0d
	ShadeDark  = 0x80
)

// FillColor is f's color in p; ok is false for FillNone and unknown fills.
// FillSurface is Raised, FillField is Field, FillSeparator is Divider.
func FillColor(p *theme.Palette, f Fill) (c color.NRGBA, ok bool) {
	switch f {
	case FillSurface, FillRaised:
		return p.Raised, true
	case FillField:
		return p.Field, true
	case FillSeparator:
		return p.Divider, true
	case FillCanvas:
		return p.Canvas, true
	case FillChrome:
		return p.Chrome, true
	case FillContent:
		return p.Content, true
	case FillAccent:
		return p.Accent, true
	case FillSelection:
		return p.SelectionActive, true
	case FillSelectionInactive:
		return p.SelectionInactive, true
	case FillPressed:
		return p.Pressed, true
	case FillInfoTint:
		return p.InfoTint, true
	case FillSuccessTint:
		return p.SuccessTint, true
	case FillWarningTint:
		return p.WarningTint, true
	case FillDangerTint:
		return p.DangerTint, true
	case FillCurrentLine:
		return p.CurrentLine, true
	case FillMatch:
		return p.Match, true
	case FillDiffAdd:
		return p.Diff.Add, true
	case FillDiffAddWord:
		return p.Diff.AddWord, true
	case FillDiffDel:
		return p.Diff.Del, true
	case FillDiffDelWord:
		return p.Diff.DelWord, true
	case FillDiffFold:
		return p.Diff.Fold, true
	case FillDiffBlank:
		return p.Diff.Blank, true
	case FillBorder:
		return p.ControlBorder, true
	case FillShade:
		if isDark(p) {
			return color.NRGBA{A: ShadeDark}, true
		}
		return color.NRGBA{A: ShadeLight}, true
	case FillGlass:
		if isDark(p) {
			return color.NRGBA{0xff, 0xff, 0xff, GlassDark}, true
		}
		return color.NRGBA{0xff, 0xff, 0xff, GlassLight}, true
	}
	return color.NRGBA{}, false
}

// isDark reports a dark palette: white reads better on its Content than
// black.
func isDark(p *theme.Palette) bool {
	return theme.Contrast(color.NRGBA{0xff, 0xff, 0xff, 0xff}, p.Content) > theme.Contrast(color.NRGBA{0, 0, 0, 0xff}, p.Content)
}

// TextColorOf is the color s's text draws in from p: Fg when set, else
// its Color role, else its text role's (body and strong text primary,
// meta and subtle secondary, captions muted).
func TextColorOf(p *theme.Palette, s TextStyle) color.Color {
	if s.Fg != nil {
		return s.Fg
	}
	switch s.Color {
	case ColorPrimary:
		return p.Text
	case ColorSecondary:
		return p.TextSecondary
	case ColorMuted:
		return p.TextMuted
	case ColorDisabled:
		return p.TextDisabled
	case ColorAccent:
		return p.Accent
	case ColorOnAccent:
		return p.OnAccent
	case ColorInfo:
		return p.Info
	case ColorSuccess:
		return p.Success
	case ColorWarning:
		return p.Warning
	case ColorDanger:
		return p.Danger
	}
	switch s.Role {
	case TextSubtle, TextMeta:
		return p.TextSecondary
	case TextCaption:
		return p.TextMuted
	}
	return p.Text
}

// cellPalette is the palette terminal cells resolve roles from; nil keeps
// the terminal's own colors (the look before themes).
var cellPalette atomic.Pointer[theme.Palette]

// UseCellTheme makes terminal cells resolve text and fill roles from p in
// true color (nil: back to the terminal's own palette). Hosts call it with
// the app's theme (flatte.WithTheme); a window resolves roles itself.
func UseCellTheme(p *theme.Palette) { cellPalette.Store(p) }

// themed is s's terminal style with p's colors: weights by role (captions
// upper case, titles and strong text bold), colors by role.
func (s TextStyle) themed(st lipgloss.Style, p *theme.Palette) lipgloss.Style {
	st = st.Foreground(TextColorOf(p, s))
	switch s.Role {
	case TextTitle, TextPageTitle, TextStrong, TextCaption:
		st = st.Bold(true)
	}
	if s.Role == TextCaption {
		st = st.Transform(strings.ToUpper)
	}
	if s.Bg != nil {
		st = st.Background(s.Bg)
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Strikethrough {
		st = st.Strikethrough(true)
	}
	return st
}

// cellBox is b as terminal cells paint it: with a theme, a Fill (or a
// Selected box's selection) becomes its background, translucent fills
// composed over the content surface.
func cellBox(b Box) Box {
	p := cellPalette.Load()
	if p == nil || b.hasBackground() {
		return b
	}
	f := b.Fill
	if f == FillNone && b.Selected {
		f = FillSelection
	}
	if c, ok := FillColor(p, f); ok {
		if c.A < 0xff {
			under := p.Content
			if f == FillShade { // it lies on the window's ground
				under = p.Chrome
			}
			c = theme.Over(c, under)
		}
		b.Background = c
	}
	return b
}
