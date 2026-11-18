package flatest_test

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/theme"
)

type themed struct{ ctx flatte.RenderContext }

func themedApp(s *themed) flatte.App[themed] {
	return flatte.App[themed]{State: s, View: func(s *themed, ctx flatte.RenderContext) flatte.Frame {
		s.ctx = ctx
		return flatte.Frame{Content: "x"}
	}}
}

func TestViewsSeeTheTheme(t *testing.T) {
	// A terminal without a theme sees none.
	s := &themed{}
	d := flatest.Start(themedApp(s), 10)
	d.Frame()
	if s.ctx.Theme != nil || s.ctx.Palette != nil || s.ctx.Appearance != flatte.AppearanceUnknown {
		t.Fatalf("unthemed terminal: %+v", s.ctx)
	}
	d.Close()

	// A graphical host defaults to theme.Default().
	s = &themed{}
	d = flatest.StartWith(themedApp(s), flatest.Options{Graphical: true})
	d.Frame()
	if s.ctx.Theme == nil || s.ctx.Palette == nil || s.ctx.Palette.Canvas != theme.Default().Dark.Canvas {
		t.Fatalf("graphical default: %+v", s.ctx.Palette)
	}
	d.Close()

	// An app's theme reaches View, its dark palette in a terminal.
	teal := theme.Pair{Light: theme.Hex("#0e7468"), Dark: theme.Hex("#4fd1c0")}
	mine := theme.Default().With(theme.Seeds{Accent: &teal, Density: theme.Compact})
	s = &themed{}
	d = flatest.StartWith(themedApp(s), flatest.Options{Theme: &mine})
	d.Frame()
	if s.ctx.Palette == nil || s.ctx.Palette.Accent != teal.Dark || s.ctx.Appearance != flatte.AppearanceDark || s.ctx.Theme.Size.Row != mine.Size.Row {
		t.Fatalf("app theme: %+v", s.ctx)
	}
	d.Close()
}
