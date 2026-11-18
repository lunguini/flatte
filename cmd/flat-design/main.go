// Command flat-design is Flatte's design system shown by Flatte itself: the
// palettes, the type scale and the spacing, size, radius and stroke
// scales of theme.Default(), in a terminal (themed cells), with -gui in a
// native window, and compiled to WebAssembly on a browser canvas.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"os"
	"runtime"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/theme"
	"github.com/lunguini/flatte/window"
)

// Pages, in the sidebar's order.
const (
	pageColors = iota
	pageType
	pageShape
	pageCount
)

var pageNames = [pageCount]string{"Colors", "Type", "Space & shape"}

// Layout IDs.
const (
	idSidebar  = "sidebar"
	pagePrefix = "page:"
)

// State is the page shown.
type State struct {
	page int
}

// NewState opens on the colors.
func NewState() *State { return &State{} }

// Handle moves between pages (↑/↓, 1–3, a click) and quits (q).
func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.KeyEvent:
		switch {
		case e.Key == flatte.KeyUp:
			s.page = (s.page + pageCount - 1) % pageCount
		case e.Key == flatte.KeyDown:
			s.page = (s.page + 1) % pageCount
		case e.Key == flatte.KeyCharacter && e.Rune >= '1' && e.Rune < '1'+pageCount:
			s.page = int(e.Rune - '1')
		case e.Key == flatte.KeyCharacter && e.Rune == 'q', e.Key == flatte.KeyCtrlC:
			fx.Quit()
		}
	case flatte.MouseEvent:
		if e.Action != flatte.MousePress || e.Button != flatte.MouseLeft {
			return
		}
		for i := range pageCount {
			if e.Target == pageID(i) {
				s.page = i
			}
		}
	}
}

func pageID(i int) string { return fmt.Sprintf("%s%d", pagePrefix, i) }

// View is the sidebar and the page, both from the theme in ctx (the
// default one when the host has none to give).
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	t := ctx.Theme
	if t == nil {
		d := theme.Default()
		t = &d
	}
	var page layout.Node
	switch s.page {
	case pageType:
		page = typePage(t, ctx.Graphical)
	case pageShape:
		page = shapePage(t, ctx.Graphical)
	default:
		page = colorsPage(t, ctx.Graphical)
	}
	return flatte.Frame{Title: "Flatte design system", Root: layout.Row{
		NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)},
		Children: []layout.Node{
			sidebar(s, ctx.Graphical),
			layout.Col{NodeBase: layout.NodeBase{W: layout.Points(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillSeparator}}},
			layout.Scroll{
				NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillContent}},
				Child:    page,
			},
		},
	}}
}

func sidebar(s *State, g bool) layout.Node {
	kids := []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{PadLeft: 1, PadTop: 1, PadBottom: 1}, Text: "Flatte", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{NodeBase: layout.NodeBase{PadLeft: 1}, Text: "Design system", Style: layout.TextStyle{Role: layout.TextCaption}},
	}
	for i, name := range pageNames {
		fill := layout.FillNone
		style := layout.TextStyle{}
		if i == s.page {
			fill, style.Role = layout.FillSelection, layout.TextStrong
		}
		h := layout.Fixed(1)
		if g {
			h = layout.Points(int(theme.Default().Size.Row))
		}
		kids = append(kids, layout.Label{
			NodeBase: layout.NodeBase{ID: pageID(i), H: h, PadLeft: 1, PadRight: 1, Box: layout.Box{Fill: fill, Round: true, Interactive: true}},
			Text:     name, Style: style,
		})
	}
	return layout.Col{NodeBase: layout.NodeBase{ID: idSidebar, W: layout.Fixed(22), H: layout.Grow(1), PadLeft: 1, PadRight: 1, Gap: 0, Box: layout.Box{Fill: layout.FillChrome}}, Children: kids}
}

// swatch is a named palette color, with what it is for (the design docs
// carry the usage; the page shows the name, value and contrast).
type swatch struct {
	name  string
	c     color.NRGBA
	about string
}

func swatches(p *theme.Palette) []swatch {
	return []swatch{
		{"Canvas", p.Canvas, "window ground"}, {"Chrome", p.Chrome, "headers, sidebars"}, {"Content", p.Content, "where work is read"},
		{"Field", p.Field, "inputs"}, {"Raised", p.Raised, "popovers"},
		{"Text", p.Text, "body"}, {"TextSecondary", p.TextSecondary, "paths, metadata"}, {"TextMuted", p.TextMuted, "captions, gutters"},
		{"Accent", p.Accent, "the action color"}, {"Focus", p.Focus, "focus ring"},
		{"SelectionActive", p.SelectionActive, "selected row, focused"}, {"SelectionInactive", p.SelectionInactive, "selected row"},
		{"Divider", p.Divider, "separators"}, {"ControlBorder", p.ControlBorder, "field outlines"},
		{"Info", p.Info, "working"}, {"Success", p.Success, "ready, added"}, {"Warning", p.Warning, "needs you"}, {"Danger", p.Danger, "error, removed"},
		{"Diff.Add", p.Diff.Add, "added line"}, {"Diff.Del", p.Diff.Del, "removed line"},
		{"Syntax.Keyword", p.Syntax.Keyword, "code"}, {"Syntax.String", p.Syntax.String, "code"}, {"Syntax.Func", p.Syntax.Func, "code"},
	}
}

// colorsPage shows both palettes side by side, each on its own content
// surface with its own text colors, whichever appearance the host is in.
func colorsPage(t *theme.Theme, g bool) layout.Node {
	col := func(name string, p *theme.Palette) layout.Node {
		ink := func(c color.NRGBA) layout.TextStyle { return layout.TextStyle{Fg: c} }
		rows := []layout.Node{layout.Label{NodeBase: layout.NodeBase{PadBottom: 1}, Text: name, Style: layout.TextStyle{Role: layout.TextCaption, Fg: p.TextMuted}}}
		for _, s := range swatches(p) {
			chip := layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(4), Box: layout.Box{Background: s.c, Round: true}}, Text: "    "}
			if g {
				chip.W = layout.Points(28)
			}
			ratio := ""
			if s.c.A == 0xff {
				ratio = fmt.Sprintf("%4.1f:1", theme.Contrast(s.c, p.Content))
			}
			rows = append(rows, layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1), Gap: 1}, Children: []layout.Node{
				chip,
				layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(18)}, Text: s.name, Style: ink(p.Text), Truncate: true},
				layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(8)}, Text: hex(s.c), Style: ink(p.TextSecondary)},
				layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: ratio, Style: ink(p.TextMuted)},
			}})
		}
		return layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), Pad: 1, Box: layout.Box{Background: p.Content, Round: true}}, Children: rows}
	}
	return page("Colors", "Each palette on its own content surface. The ratio is each color's contrast with that surface; text roles need 4.5:1.",
		layout.Row{NodeBase: layout.NodeBase{W: layout.Grow(1), Gap: 2}, Children: []layout.Node{col("Light", &t.Light), col("Dark", &t.Dark)}})
}

func hex(c color.NRGBA) string {
	if c.A == 0xff {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

func typePage(t *theme.Theme, _ bool) layout.Node {
	sample := func(role layout.TextRole, name string, spec theme.TextSpec, text string) layout.Node {
		return layout.Row{NodeBase: layout.NodeBase{Gap: 2, PadBottom: 1}, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(26)}, Text: fmt.Sprintf("%s  %g/%g · %d", name, spec.Size, spec.Line, spec.Weight), Style: layout.TextStyle{Role: layout.TextMeta}},
			layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: text, Style: layout.TextStyle{Role: role}, Truncate: true},
		}}
	}
	ts := t.Type
	return page("Type", "One scale for every app. A terminal keeps one size and shows the scale by weight, case and color.",
		layout.Col{Children: []layout.Node{
			sample(layout.TextPageTitle, "page-title", ts.PageTitle, "Settings"),
			sample(layout.TextTitle, "title", ts.Title, "No agent sessions"),
			sample(layout.TextStrong, "body-strong", theme.TextSpec{Size: ts.Body.Size, Line: ts.Body.Line, Weight: 600}, "changes.go"),
			sample(layout.TextBody, "body", ts.Body, "Run claude in a terminal to see its sessions here."),
			sample(layout.TextMeta, "meta", ts.Meta, "internal/app · 2 files changed"),
			sample(layout.TextCaption, "caption", ts.Caption, "Files"),
		}})
}

func shapePage(t *theme.Theme, g bool) layout.Node {
	bar := func(name string, pts float64, about string) layout.Node {
		w := layout.Fixed(max(int(pts/4), 1))
		if g {
			w = layout.Points(int(pts))
		}
		return layout.Row{NodeBase: layout.NodeBase{H: layout.Fixed(1), Gap: 2}, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(14)}, Text: name, Style: layout.TextStyle{Role: layout.TextStrong}},
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(6)}, Text: fmt.Sprintf("%gpt", pts), Style: layout.TextStyle{Role: layout.TextMeta}},
			layout.Col{NodeBase: layout.NodeBase{W: w, H: layout.Fixed(1), Box: layout.Box{Fill: layout.FillAccent}}},
			layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: about, Style: layout.TextStyle{Role: layout.TextMeta}, Truncate: true},
		}}
	}
	section := func(title string, rows ...layout.Node) layout.Node {
		return layout.Col{NodeBase: layout.NodeBase{PadBottom: 1}, Children: append([]layout.Node{
			layout.Label{NodeBase: layout.NodeBase{PadBottom: 1}, Text: title, Style: layout.TextStyle{Role: layout.TextCaption}},
		}, rows...)}
	}
	sp, sz, r, st := t.Space, t.Size, t.Radius, t.Stroke
	return page("Space & shape", "Every gap, inset, height and corner comes from these steps; a terminal reads them as whole cells.",
		layout.Col{Children: []layout.Node{
			section("Spacing",
				bar("space-1", sp[0], "icon to label"), bar("space-2", sp[1], "items in a row"), bar("space-3", sp[2], "pane inset"),
				bar("space-4", sp[3], "groups in a pane"), bar("space-5", sp[4], "sections"), bar("space-6", sp[5], "page margins")),
			section("Sizes",
				bar("row", sz.Row, "list, tree and tab rows"), bar("control", sz.Control, "buttons, fields"), bar("bar", sz.Bar, "pane headers, tab strips"),
				bar("header", sz.Header, "the window header"), bar("icon", sz.Icon, "icons in rows")),
			section("Radius and strokes",
				bar("radius-sm", r.Sm, "rows, tabs, badges"), bar("radius-md", r.Md, "controls"), bar("radius-lg", r.Lg, "popovers"),
				bar("divider", st.Divider, "separators"), bar("focus", st.Focus, "the focus ring")),
		}})
}

// page is a page's title, one line about it, and its body.
func page(title, about string, body layout.Node) layout.Node {
	return layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), PadLeft: 2, PadRight: 2, PadTop: 1}, Children: []layout.Node{
		layout.Label{Text: title, Style: layout.TextStyle{Role: layout.TextPageTitle}},
		layout.Label{NodeBase: layout.NodeBase{PadBottom: 1}, Text: about, Style: layout.TextStyle{Role: layout.TextMeta}, Wrap: true},
		body,
	}}
}

func main() {
	gui := runtime.GOOS == "js" // a browser page always draws on the canvas
	if window.Supported() && !gui {
		flag.BoolVar(&gui, "gui", false, "run in a native window instead of the terminal")
	}
	flag.Parse()
	app := flatte.App[State]{State: NewState(), Handle: Handle, View: View}
	opts := []flatte.Option{flatte.WithTheme(theme.Default()), flatte.WithMouse(flatte.MouseModeCellMotion), flatte.WithoutDefaultQuit()}
	if gui {
		opts = append(opts, window.Options{Title: "Flatte design system", Width: 120, Height: 40})
	}
	if err := flatte.Run(context.Background(), app, opts...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
