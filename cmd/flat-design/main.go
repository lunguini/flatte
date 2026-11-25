// Command flat-design is Flatte's design system shown by Flatte itself: the
// palettes, the type scale, the spacing, size, radius and stroke scales,
// the flatui/kit components and an app shell, restyled live by density,
// shape and accent dials, in a terminal (themed cells), with -gui in a
// native window, and compiled to WebAssembly on a browser canvas.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/kit"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/theme"
	"github.com/lunguini/flatte/window"
)

// Pages, in the sidebar's order.
const (
	pageColors = iota
	pageType
	pageShape
	pageComponents
	pageShell
	pageCount
)

var pageNames = [pageCount]string{"Colors", "Type", "Space & shape", "Components", "App shell"}
var pageIcons = [pageCount]layout.IconName{layout.IconCircle, layout.IconInfo, layout.IconPanelBottom, layout.IconGear, layout.IconSidebarLeft}

// Layout IDs.
const (
	idSidebar  = "sidebar"
	pagePrefix = "page:"
	idDensity  = "density"
	idShape    = "shape"
	idAccent   = "accent"
	rowPrefix  = "row:"
	tabPrefix  = "tab:"
	idChoice   = "choice"
	idWrap     = "wrap"
	idPopover  = "popover"
	idOpen     = "open"
	idMode     = "mode"
	idQuery    = "query"
	idFont     = "font"
	idRadio    = "radio:"
	idVolume   = "volume"
)

// controls are the focusable demo controls, in Tab order.
var controls = []string{"primary", "secondary", "quiet", "danger", "icon", idQuery, "check:format", "check:trim",
	idRadio + "0", idRadio + "1", idRadio + "2", "switch:autosave", idVolume, idChoice, idWrap, idFont}

// accents are the showcase's accent seeds (all pass theme.Check).
var accents = []struct {
	name string
	pair theme.Pair
}{
	{"Blue", theme.Pair{}}, // the theme's own
	{"Teal", theme.Pair{Light: theme.Hex("#0e7468"), Dark: theme.Hex("#4fd1c0")}},
	{"Violet", theme.Pair{Light: theme.Hex("#6d3fd8"), Dark: theme.Hex("#b69bff")}},
	{"Graphite", theme.Pair{Light: theme.Hex("#3d4450"), Dark: theme.Hex("#c3c9d3")}},
}

var (
	densityNames = []string{"Compact", "Regular", "Comfortable"}
	densities    = []theme.Density{theme.Compact, theme.Regular, theme.Comfortable}
	shapeNames   = []string{"Square", "Subtle", "Round"}
	shapes       = []theme.Roundness{theme.Square, theme.Subtle, theme.Round}
	modeNames    = []string{"System", "Light", "Dark"}
)

// State is the page shown, the dials, and the component demos' state.
type State struct {
	page                   int
	density, shape, accent int // indexes into densities, shapes, accents
	mode                   int // 0 follows the system, 1 light, 2 dark
	theme                  theme.Theme

	row, tab, choice int
	wrap, popover    bool

	// focus is the focused demo control's ID ("": none); the rest is the
	// Controls group's state.
	focus        string
	query        string
	format, trim bool
	radio        int
	autosave     bool
	volume       float64
}

// NewState opens on the colors in the default theme.
func NewState() *State {
	s := &State{density: 1, shape: 1, tab: 0, choice: 1, wrap: true, format: true, radio: 2, autosave: true, volume: 0.6}
	s.restyle()
	return s
}

// restyle derives the theme from the dials; View returns it, so every
// host restyles live.
func (s *State) restyle() {
	seeds := theme.Seeds{Density: densities[s.density], Roundness: shapes[s.shape]}
	if s.accent > 0 {
		pair := accents[s.accent].pair
		seeds.Accent = &pair
	}
	s.theme = theme.Default().With(seeds)
	switch s.mode { // one palette for both appearances pins the look
	case 1:
		s.theme.Dark = s.theme.Light
	case 2:
		s.theme.Light = s.theme.Dark
	}
}

// Handle moves between pages (↑/↓, 1–5, a click), turns the dials (d, r,
// a, or their segments) and quits (q).
func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.KeyEvent:
		if controlKey(s, e) {
			return
		}
		switch {
		case e.Key == flatte.KeyUp:
			s.page = (s.page + pageCount - 1) % pageCount
		case e.Key == flatte.KeyDown:
			s.page = (s.page + 1) % pageCount
		case e.Key == flatte.KeyCharacter && e.Rune >= '1' && e.Rune < '1'+pageCount:
			s.page = int(e.Rune - '1')
		case e.Key == flatte.KeyCharacter && e.Rune == 'd':
			s.density = (s.density + 1) % len(densities)
			s.restyle()
		case e.Key == flatte.KeyCharacter && e.Rune == 'r':
			s.shape = (s.shape + 1) % len(shapes)
			s.restyle()
		case e.Key == flatte.KeyCharacter && e.Rune == 'a':
			s.accent = (s.accent + 1) % len(accents)
			s.restyle()
		case e.Key == flatte.KeyCharacter && e.Rune == 'm':
			s.mode = (s.mode + 1) % len(modeNames)
			s.restyle()
		case e.Key == flatte.KeyEscape:
			s.popover, s.focus = false, ""
		case e.Key == flatte.KeyCharacter && e.Rune == 'q', e.Key == flatte.KeyCtrlC:
			fx.Quit()
		}
	case flatte.MouseEvent:
		if e.Action != flatte.MousePress || e.Button != flatte.MouseLeft {
			return
		}
		if e.Target == idVolume {
			s.volume = layout.Slider{}.ValueAt(e.X-e.TargetRect.X, e.TargetRect.W)
		}
		click(s, e.Target)
	}
}

// controlKey moves focus through the demo controls (Tab, Shift-Tab), types
// into the focused field, and works the focused control (Space, Enter,
// ←/→); it reports whether it used the key.
func controlKey(s *State, e flatte.KeyEvent) bool {
	if e.Key == flatte.KeyTab {
		i := slices.Index(controls, s.focus)
		switch {
		case e.Mod&flatte.ModShift != 0 && i <= 0:
			i = len(controls) - 1
		case e.Mod&flatte.ModShift != 0:
			i--
		default:
			i = (i + 1) % len(controls)
		}
		s.focus = controls[i]
		return true
	}
	if s.focus == "" {
		return false
	}
	switch {
	case s.focus == idQuery && e.Key == flatte.KeyCharacter && e.Mod&(flatte.ModCtrl|flatte.ModAlt) == 0:
		s.query += string(e.Rune)
	case s.focus == idQuery && e.Key == flatte.KeyBackspace:
		if r := []rune(s.query); len(r) > 0 {
			s.query = string(r[:len(r)-1])
		}
	case e.Key == flatte.KeyEnter, e.Key == flatte.KeyCharacter && e.Rune == ' ':
		click(s, s.focus)
	case e.Key == flatte.KeyLeft || e.Key == flatte.KeyRight:
		step := 1
		if e.Key == flatte.KeyLeft {
			step = -1
		}
		switch {
		case s.focus == idVolume:
			s.volume = min(max(s.volume+0.05*float64(step), 0), 1)
		case s.focus == idChoice:
			s.choice = min(max(s.choice+step, 0), 2)
		case strings.HasPrefix(s.focus, idRadio):
			s.radio = min(max(s.radio+step, 0), 2)
			s.focus = idRadio + strconv.Itoa(s.radio)
		default:
			return false
		}
	default:
		return false
	}
	return true
}

func click(s *State, target string) {
	for i := range pageCount {
		if target == pageID(i) {
			s.page = i
		}
	}
	dial := func(id string, n int, set *int) {
		for i := range n {
			if target == kit.OptionID(id, i) {
				*set = i
				s.restyle()
			}
		}
	}
	dial(idDensity, len(densities), &s.density)
	dial(idShape, len(shapes), &s.shape)
	dial(idAccent, len(accents), &s.accent)
	dial(idMode, len(modeNames), &s.mode)
	for i := range 3 {
		if target == kit.OptionID(idChoice, i) {
			s.choice, s.focus = i, idChoice
		}
	}
	if slices.Contains(controls, target) {
		s.focus = target
	}
	switch {
	case strings.HasPrefix(target, rowPrefix):
		s.row, _ = strconv.Atoi(target[len(rowPrefix):])
	case strings.HasPrefix(target, tabPrefix):
		s.tab, _ = strconv.Atoi(target[len(tabPrefix):])
	case target == idWrap:
		s.wrap = !s.wrap
	case target == "check:format":
		s.format = !s.format
	case target == "check:trim":
		s.trim = !s.trim
	case strings.HasPrefix(target, idRadio):
		s.radio, _ = strconv.Atoi(target[len(idRadio):])
	case target == "switch:autosave":
		s.autosave = !s.autosave
	case target == idOpen:
		s.popover = !s.popover
	}
}

func pageID(i int) string { return fmt.Sprintf("%s%d", pagePrefix, i) }

// View is the sidebar, the dials and the page, in the theme the dials
// make (Frame.Theme: the host restyles to it).
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	t := &s.theme
	var page layout.Node
	switch s.page {
	case pageType:
		page = typePage(t, ctx.Graphical)
	case pageShape:
		page = shapePage(t, ctx.Graphical)
	case pageComponents:
		page = componentsPage(s)
	case pageShell:
		page = shellPage(s)
	default:
		page = colorsPage(t, ctx.Graphical)
	}
	root := layout.Row{
		NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)},
		Children: []layout.Node{
			sidebar(s),
			divider(true),
			layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)}, Children: []layout.Node{
				dials(s),
				divider(false),
				layout.Scroll{
					NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillContent}},
					Child:    page,
				},
			}},
		},
	}
	return flatte.Frame{Title: "Flatte design system", Theme: t, Root: root}
}

// divider is a hairline between panes, vertical or horizontal.
func divider(vertical bool) layout.Node {
	nb := layout.NodeBase{W: layout.Grow(1), H: layout.Points(1), Box: layout.Box{Fill: layout.FillSeparator}}
	if vertical {
		nb.W, nb.H = layout.Points(1), layout.Grow(1)
	}
	return layout.Col{NodeBase: nb}
}

func sidebar(s *State) layout.Node {
	rows := make([]layout.Node, pageCount)
	for i, name := range pageNames {
		rows[i] = kit.ListRow{ID: pageID(i), Icon: pageIcons[i], Title: name, Selected: i == s.page, Active: true}.Layout()
	}
	return layout.Col{NodeBase: layout.NodeBase{ID: idSidebar, W: layout.Fixed(24), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillChrome}}, Children: []layout.Node{
		kit.PaneHeader{Title: "Flatte", Meta: "design system"}.Layout(),
		layout.Col{NodeBase: layout.NodeBase{PadLeft: 1, PadRight: 1}, Children: rows},
	}}
}

// dials are the three seeds — density, shape, accent — as segmented
// choices (named in their tooltips): the whole showcase restyles as they
// turn.
func dials(s *State) layout.Node {
	dial := func(name, id string, options []string, selected int) layout.Node {
		seg := kit.Segmented{ID: id, Options: options, Selected: selected}.Layout().(layout.Row)
		seg.Tooltip = name
		return seg
	}
	names := make([]string, len(accents))
	for i, a := range accents {
		names[i] = a.name
	}
	return layout.Row{
		NodeBase: layout.NodeBase{H: layout.BarSize(), Gap: 2, PadLeft: 2, PadRight: 2, Box: layout.Box{Fill: layout.FillChrome}},
		Align:    layout.AlignCenter,
		Children: []layout.Node{
			dial("Density", idDensity, densityNames, s.density),
			dial("Shape", idShape, shapeNames, s.shape),
			dial("Accent", idAccent, names, s.accent),
			dial("Appearance", idMode, modeNames, s.mode),
		},
	}
}

// componentsPage shows every kit component and the controls in the theme.
func componentsPage(s *State) layout.Node {
	files := []string{"main.go", "theme.go", "kit.go", "README.md"}
	rows := make([]layout.Node, len(files))
	for i, f := range files {
		var trailing []layout.Node
		if i == 1 {
			trailing = []layout.Node{kit.Badge{Text: "M", Tone: kit.ToneWarning}.Layout()}
		}
		rows[i] = kit.ListRow{ID: rowPrefix + strconv.Itoa(i), Icon: layout.IconFile, Title: f, Meta: "cmd/flat-design", Selected: i == s.row, Active: true, Trailing: trailing}.Layout()
	}
	inactive := kit.ListRow{Title: "selected, list not focused", Selected: true}.Layout()
	tabs := []kit.Tab{{ID: tabPrefix + "0", Label: "main.go"}, {ID: tabPrefix + "1", Label: "theme.go", Modified: true}, {ID: tabPrefix + "2", Label: "kit.go"}}
	f := func(id string) bool { return s.focus == id }
	button := func(id, label string, primary, quiet, disabled bool) layout.Node {
		return layout.Button{NodeBase: layout.NodeBase{ID: id}, Label: label, Default: primary, Plain: quiet, Disabled: disabled, Focused: f(id)}
	}
	buttons := layout.Col{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
			button("primary", "Commit", true, false, false),
			button("secondary", "Discard", false, false, false),
			button("quiet", "Later", false, true, false),
			layout.Button{NodeBase: layout.NodeBase{ID: "danger"}, Label: "Delete branch", Style: layout.TextStyle{Color: layout.ColorDanger}, Focused: f("danger")},
			layout.Button{NodeBase: layout.NodeBase{ID: "icon", Tooltip: "Search"}, Icon: layout.IconSearch, Label: "search", Focused: f("icon")},
		}},
		layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
			button("", "Commit", true, false, true),
			button("", "Discard", false, false, true),
			button("", "Later", false, true, true),
			layout.Label{Text: "disabled", Style: layout.TextStyle{Role: layout.TextMeta, Color: layout.ColorMuted}},
		}},
	}}
	fields := layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		layout.Input{NodeBase: layout.NodeBase{ID: idQuery, W: layout.Fixed(28)}, Value: s.query, Caret: len([]rune(s.query)), Placeholder: "Go to file…", Focused: f(idQuery)},
		layout.Input{NodeBase: layout.NodeBase{W: layout.Fixed(16)}, Value: "read only"},
	}}
	toggle := func(id, label string, kind layout.ToggleKind, on, disabled bool) layout.Node {
		return layout.Toggle{NodeBase: layout.NodeBase{ID: id}, Kind: kind, Label: label, Checked: on, Disabled: disabled, Focused: f(id)}
	}
	radios := make([]layout.Node, 3)
	for i, name := range []string{"Light", "Dark", "System"} {
		radios[i] = toggle(idRadio+strconv.Itoa(i), name, layout.ToggleRadio, s.radio == i, false)
	}
	toggles := layout.Col{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: []layout.Node{
			toggle("check:format", "Format on save", layout.ToggleCheckbox, s.format, false),
			toggle("check:trim", "Trim whitespace", layout.ToggleCheckbox, s.trim, false),
			toggle("", "Disabled", layout.ToggleCheckbox, true, true),
		}},
		layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: radios},
		layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: []layout.Node{
			toggle("switch:autosave", "Autosave", layout.ToggleSwitch, s.autosave, false),
			toggle("", "Disabled", layout.ToggleSwitch, false, true),
		}},
		layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Align: layout.AlignCenter, Children: []layout.Node{
			layout.Label{Text: "Volume", Style: layout.TextStyle{Color: layout.ColorSecondary}},
			layout.Slider{NodeBase: layout.NodeBase{ID: idVolume, W: layout.Fixed(24)}, Value: s.volume, Focused: f(idVolume)},
		}},
	}}
	badges := layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		kit.Badge{Text: "idle"}.Layout(), kit.Badge{Text: "working", Tone: kit.ToneInfo}.Layout(),
		kit.Badge{Text: "ready", Tone: kit.ToneSuccess}.Layout(), kit.Badge{Text: "needs you", Tone: kit.ToneWarning}.Layout(),
		kit.Badge{Text: "failed", Tone: kit.ToneDanger}.Layout(), kit.Badge{Text: "3", Tone: kit.ToneAccent}.Layout(),
	}}
	settings := kit.Section{Title: "Editor", Children: []layout.Node{
		kit.Setting{ID: "set:wrap", Name: "Wrap lines", Description: "Soft-wrap lines longer than the pane.", Control: layout.Toggle{NodeBase: layout.NodeBase{ID: idWrap}, Kind: layout.ToggleSwitch, Checked: s.wrap, Focused: f(idWrap)}}.Layout(),
		kit.Setting{ID: "set:tabs", Name: "Indent with", Control: kit.Segmented{ID: idChoice, Options: []string{"Tabs", "2 spaces", "4 spaces"}, Selected: s.choice, Focused: f(idChoice)}.Layout()}.Layout(),
		kit.Setting{ID: "set:font", Name: "Font", Control: layout.Input{NodeBase: layout.NodeBase{ID: idFont, W: layout.Fixed(20)}, Value: "Menlo", Focused: f(idFont)}}.Layout(),
	}}
	body := layout.Col{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
		group("Pane header", kit.PaneHeader{Title: "Changes", Meta: "4 files", Actions: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: "refresh"}, Icon: layout.IconRepeat, Label: "↻", Plain: true},
		}}.Layout()),
		group("Tabs", kit.Tabs{Tabs: tabs, Active: s.tab}.Layout()),
		group("List rows", layout.Col{Children: append(append(rows, kit.Divider{Title: "Other"}.Layout()), inactive)}),
		group("Buttons — primary, secondary, quiet, danger, icon", buttons),
		group("Fields", fields),
		group("Checkboxes, radios, switches, slider", toggles),
		group("Badges", badges),
		group("Settings", settings.Layout()),
		group("Popover", layout.Row{Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{ID: idOpen}, Label: "Open popover"},
		}}),
		group("Empty state", layout.Col{NodeBase: layout.NodeBase{H: layout.Fixed(8), Box: layout.Box{Fill: layout.FillCanvas, Round: true}}, Children: []layout.Node{
			kit.EmptyState{Icon: layout.IconFolder, Title: "No folder open", Hint: "Open a folder to browse its files.", Actions: []layout.Node{
				layout.Button{NodeBase: layout.NodeBase{ID: "openFolder"}, Label: "Open Folder…", Default: true},
			}}.Layout(),
		}}),
	}}
	kids := []layout.Node{body}
	if s.popover {
		kids = append(kids, kit.Popover{ID: idPopover, Anchor: idOpen, Width: layout.Fixed(36), Children: []layout.Node{
			kit.ListRow{Icon: layout.IconFile, Title: "main.go", Meta: "cmd/flat-design", Selected: true, Active: true}.Layout(),
			kit.ListRow{Icon: layout.IconFile, Title: "theme.go", Meta: "theme"}.Layout(),
			kit.ListRow{Icon: layout.IconFile, Title: "kit.go", Meta: "flatui/kit"}.Layout(),
		}}.Layout())
	}
	return page("Components", "flatui/kit and the controls: built from the theme's roles, sizes and radii — turn the dials above; Tab moves focus.",
		layout.Col{Children: kids})
}

// group is a component's caption over its sample.
func group(title string, sample layout.Node) layout.Node {
	return layout.Col{Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{PadBottom: 1}, Text: title, Style: layout.TextStyle{Role: layout.TextCaption}},
		sample,
	}}
}

// shellPage is an app's frame from the kit alone: a files sidebar, an
// editor with tabs, and a sessions sidebar, as an IDE lays them out.
func shellPage(s *State) layout.Node {
	files := []struct {
		name  string
		depth int
		icon  layout.IconName
	}{{"cmd", 0, layout.IconFolder}, {"flat-design", 1, layout.IconFolder}, {"main.go", 2, layout.IconFile}, {"main_test.go", 2, layout.IconFile},
		{"theme", 0, layout.IconFolder}, {"theme.go", 1, layout.IconFile}, {"seeds.go", 1, layout.IconFile}}
	tree := []layout.Node{kit.PaneHeader{Title: "Files", Actions: []layout.Node{layout.Button{NodeBase: layout.NodeBase{ID: "newFile"}, Icon: layout.IconPlus, Label: "+", Plain: true}}}.Layout()}
	for i, f := range files {
		tree = append(tree, kit.ListRow{ID: rowPrefix + strconv.Itoa(i), Icon: f.icon, Title: f.name, Indent: f.depth, Selected: i == s.row, Active: true}.Layout())
	}
	code := []string{"package main", "", "import \"github.com/lunguini/flatte/flatui/kit\"", "", "func row(name string) layout.Node {", "\treturn kit.ListRow{Title: name}.Layout()", "}"}
	lines := make([]layout.Node, len(code))
	for i, l := range code {
		lines[i] = layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(3)}, Text: strconv.Itoa(i + 1), Style: layout.TextStyle{Color: layout.ColorMuted}},
			layout.Label{Text: strings.ReplaceAll(l, "\t", "    "), Style: layout.TextStyle{Fg: nil}},
		}}
	}
	editor := layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)}, Children: []layout.Node{
		kit.Tabs{Tabs: []kit.Tab{{ID: tabPrefix + "0", Label: "main.go"}, {ID: tabPrefix + "1", Label: "theme.go", Modified: true}}, Active: s.tab}.Layout(),
		layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), PadLeft: 1, PadTop: 1, Box: layout.Box{Fill: layout.FillContent}}, Children: lines},
	}}
	sessions := layout.Col{NodeBase: layout.NodeBase{W: layout.Fixed(28), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillChrome}}, Children: []layout.Node{
		kit.PaneHeader{Title: "Agents", Meta: "2"}.Layout(),
		kit.ListRow{Title: "Design system", Trailing: []layout.Node{kit.Badge{Text: "working", Tone: kit.ToneInfo}.Layout()}}.Layout(),
		kit.ListRow{Title: "Fix the tree", Trailing: []layout.Node{kit.Badge{Text: "needs you", Tone: kit.ToneWarning}.Layout()}}.Layout(),
	}}
	shell := layout.Row{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Fixed(16), Box: layout.Box{Border: layout.BorderRounded}}, Children: []layout.Node{
		layout.Col{NodeBase: layout.NodeBase{W: layout.Fixed(26), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillChrome}}, Children: tree},
		divider(true), editor, divider(true), sessions,
	}}
	return page("App shell", "An IDE's frame from the kit alone: no colors or sizes of its own.", shell)
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
