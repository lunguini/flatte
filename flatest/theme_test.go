package flatest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
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

// A frame's Theme restyles the app live: the frame that sets it is viewed
// in it, the terminal's cells paint in it, and clearing it goes back to
// the app's own.
func TestFrameThemeSwitchesLive(t *testing.T) {
	teal := theme.Pair{Light: theme.Hex("#0e7468"), Dark: theme.Hex("#4fd1c0")}
	compact := theme.Default().With(theme.Seeds{Accent: &teal, Density: theme.Compact})
	type st struct {
		pick  *theme.Theme
		seen  []flatte.RenderContext
		views int
	}
	s := &st{}
	app := flatte.App[st]{State: s, View: func(s *st, ctx flatte.RenderContext) flatte.Frame {
		s.views++
		s.seen = append(s.seen, ctx)
		return flatte.Frame{Theme: s.pick, Root: layout.Label{Text: "x", Style: layout.TextStyle{Color: layout.ColorAccent}}}
	}}
	base := theme.Default()
	d := flatest.StartWith(app, flatest.Options{Width: 4, Height: 1, Theme: &base})
	defer d.Close()
	d.Frame()
	if got := s.seen[len(s.seen)-1].Palette.Accent; got != base.Dark.Accent {
		t.Fatalf("before: accent %v", got)
	}
	v0 := s.views
	s.pick = &compact
	f := d.Frame()
	last := s.seen[len(s.seen)-1]
	if last.Palette.Accent != teal.Dark || last.Theme.Space != compact.Space {
		t.Fatalf("the switching frame was not viewed in its theme: %+v", last.Palette.Accent)
	}
	if s.views != v0+2 {
		t.Fatalf("views %d, want one extra for the switch", s.views-v0)
	}
	r, g, _, _ := teal.Dark.RGBA()
	if want := fmt.Sprintf("38;2;%d;%d", r>>8, g>>8); !strings.Contains(f.Content, want) {
		t.Fatalf("cells not painted in the frame's theme: %q", f.Content)
	}
	// Unchanged frames view once.
	v1 := s.views
	d.Frame()
	if s.views != v1+1 {
		t.Fatalf("steady frame viewed %d times", s.views-v1)
	}
	// Mutating the theme the state holds is a change too.
	s.pick = &theme.Theme{}
	*s.pick = compact
	s.pick.Radius.Sm = 0
	d.Frame()
	if s.seen[len(s.seen)-1].Theme.Radius.Sm != 0 {
		t.Fatal("an edited theme was not adopted")
	}
	s.pick = nil
	d.Frame()
	if got := s.seen[len(s.seen)-1].Palette.Accent; got != base.Dark.Accent {
		t.Fatalf("after clearing: accent %v", got)
	}
}
