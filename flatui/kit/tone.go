package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Tone is a status color: badges and status text.
type Tone int

const (
	ToneNeutral Tone = iota
	ToneAccent
	ToneInfo
	ToneSuccess
	ToneWarning
	ToneDanger
)

// color is the tone's text color, fill its tinted background.
func (t Tone) color() layout.TextColor {
	switch t {
	case ToneAccent:
		return layout.ColorAccent
	case ToneInfo:
		return layout.ColorInfo
	case ToneSuccess:
		return layout.ColorSuccess
	case ToneWarning:
		return layout.ColorWarning
	case ToneDanger:
		return layout.ColorDanger
	}
	return layout.ColorSecondary
}

func (t Tone) fill() layout.Fill {
	switch t {
	case ToneAccent:
		return layout.FillSelectionInactive
	case ToneInfo:
		return layout.FillInfoTint
	case ToneSuccess:
		return layout.FillSuccessTint
	case ToneWarning:
		return layout.FillWarningTint
	case ToneDanger:
		return layout.FillDangerTint
	}
	return layout.FillPressed
}
