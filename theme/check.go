package theme

import (
	"fmt"
	"image/color"
)

// Problem is a color pair below its contrast floor.
type Problem struct {
	Appearance string // "light" or "dark"
	Fg, Bg     string // the palette fields
	Ratio      float64
	Need       float64
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s on %s is %.2f:1, needs %.1f:1", p.Appearance, p.Fg, p.Bg, p.Ratio, p.Need)
}

// Check reports every text, status and control color that falls below
// WCAG 2's floor on the surfaces it is meant for: 4.5:1 for text, 3:1 for
// control outlines, the focus ring, syntax comments and punctuation.
func (t Theme) Check() []Problem {
	var out []Problem
	for _, a := range []struct {
		name string
		p    Palette
	}{{"light", t.Light}, {"dark", t.Dark}} {
		p := a.p
		type c = color.NRGBA
		surfaces := map[string]c{"Chrome": p.Chrome, "Content": p.Content, "Raised": p.Raised, "Field": p.Field}
		check := func(fgName string, fg c, need float64, bgs map[string]c) {
			for bgName, bg := range bgs {
				if r := Contrast(fg, bg); r < need {
					out = append(out, Problem{a.name, fgName, bgName, r, need})
				}
			}
		}
		check("Text", p.Text, 4.5, merge(surfaces, map[string]c{"SelectionActive": p.SelectionActive, "SelectionInactive": p.SelectionInactive, "CurrentLine": p.CurrentLine, "Diff.Add": p.Diff.Add, "Diff.Del": p.Diff.Del, "Diff.AddWord": p.Diff.AddWord, "Diff.DelWord": p.Diff.DelWord, "Match": p.Match}))
		check("TextSecondary", p.TextSecondary, 4.5, merge(surfaces, map[string]c{"Diff.Fold": p.Diff.Fold, "SelectionActive": p.SelectionActive}))
		check("TextMuted", p.TextMuted, 4.5, map[string]c{"Chrome": p.Chrome, "Content": p.Content})
		check("Accent", p.Accent, 4.5, map[string]c{"Chrome": p.Chrome, "Content": p.Content, "Raised": p.Raised})
		check("OnAccent", p.OnAccent, 4.5, map[string]c{"Accent": p.Accent, "AccentHover": p.AccentHover})
		check("Focus", p.Focus, 3, surfaces)
		check("ControlBorder", p.ControlBorder, 3, map[string]c{"Field": p.Field, "Content": p.Content, "Chrome": p.Chrome})
		for _, s := range []struct {
			name     string
			fg, tint c
		}{{"Info", p.Info, p.InfoTint}, {"Success", p.Success, p.SuccessTint}, {"Warning", p.Warning, p.WarningTint}, {"Danger", p.Danger, p.DangerTint}} {
			check(s.name, s.fg, 4.5, map[string]c{"Chrome": p.Chrome, "Content": p.Content, s.name + "Tint": s.tint})
		}
		code := map[string]c{"Content": p.Content, "CurrentLine": p.CurrentLine, "Diff.Add": p.Diff.Add, "Diff.Del": p.Diff.Del}
		y := p.Syntax
		for _, s := range []struct {
			name string
			fg   c
			need float64
		}{
			{"Syntax.Keyword", y.Keyword, 4.5}, {"Syntax.Control", y.Control, 4.5},
			{"Syntax.Type", y.Type, 4.5}, {"Syntax.Func", y.Func, 4.5}, {"Syntax.Method", y.Method, 4.5},
			{"Syntax.Variable", y.Variable, 4.5}, {"Syntax.Parameter", y.Parameter, 4.5}, {"Syntax.Property", y.Property, 4.5},
			{"Syntax.Builtin", y.Builtin, 4.5}, {"Syntax.Constant", y.Constant, 4.5}, {"Syntax.Number", y.Number, 4.5},
			{"Syntax.String", y.String, 4.5}, {"Syntax.Escape", y.Escape, 4.5},
			{"Syntax.Comment", y.Comment, 3}, {"Syntax.DocComment", y.DocComment, 3},
			{"Syntax.Operator", y.Operator, 4.5}, {"Syntax.Punctuation", y.Punctuation, 3},
			{"Syntax.Tag", y.Tag, 4.5}, {"Syntax.Attribute", y.Attribute, 4.5},
			{"Syntax.Preprocessor", y.Preprocessor, 4.5}, {"Syntax.Annotation", y.Annotation, 4.5},
			{"Syntax.Heading", y.Heading, 4.5}, {"Syntax.Link", y.Link, 4.5},
		} {
			check(s.name, s.fg, s.need, code)
		}
	}
	return out
}

func merge[K comparable, V any](a, b map[K]V) map[K]V {
	out := make(map[K]V, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
