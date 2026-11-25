// Package kit is Flatte's design-system components: pane headers, tabs,
// list rows, segmented choices, badges, sections, setting rows, empty
// states and popovers, built from layout nodes in the theme's roles (fills,
// text roles and colors) and token sizes (layout.RowSize, BarSize…). They
// hold no colors or lengths of their own, so a theme — its palette,
// density and roundness — restyles them all, in a window and in a
// terminal alike.
//
// Each component is a plain struct; Layout builds its tree, as flatui widgets do. Components are
// stateless: selection, focus and values come from app state, and clicks
// arrive as MouseEvents targeting the IDs each one documents. Every slot
// that takes nodes (Actions, Trailing, Children) takes any layout.Node, so
// an app can put its own pieces where the recipe leaves room.
package kit

import (
	"strings"

	"strconv"

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

// PaneHeader is the bar on top of a pane or sidebar: a caption title,
// optional meta text beside it, and trailing actions (icon buttons,
// usually) at the right, on the chrome surface at the bar height.
type PaneHeader struct {
	ID      string
	Title   string
	Meta    string
	Actions []layout.Node
	// Active brightens the caption while its pane has keyboard focus.
	Active bool
	// Body follows the title and takes the room before the actions: a
	// panel's tab strip (kit.Tabs), a filter field.
	Body layout.Node
}

func (h PaneHeader) Layout() layout.Node {
	caption := layout.TextStyle{Role: layout.TextCaption}
	if h.Active {
		caption.Color = layout.ColorPrimary
	}
	kids := []layout.Node{layout.Label{Text: h.Title, Style: caption, Truncate: true}}
	if h.Meta != "" {
		kids = append(kids, layout.Label{Text: h.Meta, Style: layout.TextStyle{Role: layout.TextMeta}, Truncate: true})
	}
	if h.Body != nil {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), NoPadding: true}, Align: layout.AlignStretch, Children: []layout.Node{h.Body}})
	} else {
		kids = append(kids, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	}
	kids = append(kids, h.Actions...)
	return layout.Row{
		NodeBase: layout.NodeBase{ID: h.ID, H: layout.BarSize(), Gap: 1, PadLeft: 1, PadRight: 1, Box: layout.Box{Fill: layout.FillChrome, Round: true, Flat: layout.SideBottom}},
		Align:    layout.AlignCenter, Children: kids,
	}
}

// Section is a group under a caption: settings, a sidebar's group.
type Section struct {
	ID       string
	Title    string
	Children []layout.Node
}

func (s Section) Layout() layout.Node {
	kids := make([]layout.Node, 0, len(s.Children)+1)
	if s.Title != "" {
		// At the bar height, low in it: nearer its rows than what is above.
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{H: layout.BarSize(), PadLeft: 1}, Align: layout.AlignEnd, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: s.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true},
		}})
	}
	kids = append(kids, s.Children...)
	return layout.Col{NodeBase: layout.NodeBase{ID: s.ID, Gap: 1}, Children: kids}
}

// Divider separates the groups of a list: the next group's title between
// two lines (one line without a title), at the bar height so it stands away
// from the rows around it. First is the one a list begins with: it stands
// no further off than a row.
type Divider struct {
	ID      string
	Title   string
	Tooltip string
	First   bool
}

func (d Divider) Layout() layout.Node {
	h := layout.BarSize()
	if d.First {
		h = layout.RowSize()
	}
	kids := []layout.Node{layout.Rule{}}
	if d.Title != "" {
		kids = append(kids, layout.Label{Text: d.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true}, layout.Rule{})
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: d.ID, H: h, Gap: 1, PadLeft: 1, PadRight: 1, Tooltip: d.Tooltip}, Align: layout.AlignCenter, Children: kids}
}

// NameEnd is the length of a file name's extension, its dot included: what
// to keep of a cut-off name (KeepEnd). 0 for a name without one, a dot file,
// or an "extension" too long to be one.
func NameEnd(name string) int {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 || len(name)-i > 10 || strings.ContainsAny(name[i:], " /") {
		return 0
	}
	return len(name) - i
}

// ListRow is one row of a list or tree at the row height: an optional
// leading icon, a title, meta text after it, and trailing nodes (a badge,
// a count). Selected rows take the selection fill — the strong one when
// Active (the list has focus), the quiet one otherwise; rows show hover in
// a window. Indent shifts the content right by steps (tree depth).
type ListRow struct {
	ID       string
	Icon     layout.IconName
	Title    string
	Meta     string
	Trailing []layout.Node
	Selected bool
	Active   bool
	Indent   int
	Disabled bool
	// Disclosure is a tree row's chevron, before the icon: DisclosureLeaf
	// keeps the chevron's room empty so leaves line up with folders.
	Disclosure Disclosure
	// TitleColor colors the title (and dims the icon with it) while the row
	// stays clickable: secondary files, disabled-grey ignored entries.
	TitleColor layout.TextColor
	// Italic sets the title in italics (a placeholder, a preview).
	Italic bool
	// KeepEnd keeps that many bytes at the end of a cut-off title (a file's
	// extension, NameEnd): see layout.Label.
	KeepEnd int
}

// Disclosure is a tree row's expand state (ListRow.Disclosure).
type Disclosure int

const (
	// DisclosureNone is no chevron and no room for one (a flat list).
	DisclosureNone Disclosure = iota
	// DisclosureLeaf is the chevron's room, empty: a file among folders.
	DisclosureLeaf
	DisclosureClosed
	DisclosureOpen
)

func (r ListRow) Layout() layout.Node {
	kids := []layout.Node{}
	color := r.TitleColor
	if r.Disabled {
		color = layout.ColorDisabled
	}
	if r.Disclosure != DisclosureNone {
		name, g := layout.IconBlank, " "
		switch r.Disclosure {
		case DisclosureClosed:
			name, g = layout.IconDisclosureClosed, "▸"
		case DisclosureOpen:
			name, g = layout.IconDisclosureOpen, "▾"
		}
		kids = append(kids, layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(disclosurePoints)}, Name: name, Text: g, Style: layout.TextStyle{Color: iconColor(r)}})
	}
	if r.Icon != layout.IconNone {
		kids = append(kids, icon(r.Icon, iconColor(r)))
	}
	title := layout.TextStyle{Color: color, Italic: r.Italic}
	if r.Selected && r.Active { // readable on the strong selection, dimmed or not
		title.Role, title.Color = layout.TextStrong, layout.ColorDefault
	}
	kids = append(kids, layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: r.Title, Style: title, Truncate: true, KeepEnd: r.KeepEnd})
	if r.Meta != "" {
		kids = append(kids, layout.Label{Text: r.Meta, Style: layout.TextStyle{Role: layout.TextMeta}, Truncate: true})
	}
	kids = append(kids, r.Trailing...)
	box := layout.Box{Round: true, Interactive: !r.Disabled}
	switch {
	case r.Selected && r.Active:
		box.Selected = true
	case r.Selected:
		box.Fill = layout.FillSelectionInactive
	}
	return layout.Row{
		NodeBase: layout.NodeBase{ID: r.ID, H: layout.RowSize(), Gap: 1, PadLeft: 1 + max(r.Indent, 0), PadRight: 1, Box: box},
		Align:    layout.AlignCenter, Children: kids,
	}
}

// disclosurePoints is a chevron's width in a window (a cell in a terminal,
// before the row's gap).
const disclosurePoints = 8

func iconColor(r ListRow) layout.TextColor {
	switch {
	case r.Selected && r.Active:
		return layout.ColorAccent
	case r.Disabled || r.TitleColor == layout.ColorDisabled:
		return layout.ColorDisabled
	}
	return layout.ColorSecondary
}

// Tab is one tab of a Tabs strip. Modified shows the unsaved dot; Italic
// marks a preview tab; Close, when set, is the ID of a close action shown
// after the label (in place of the dot's when Modified: the dot then closes
// too). Leading nodes (a number, a badge) come before the icon.
type Tab struct {
	ID       string
	Label    string
	Icon     layout.IconName
	Leading  []layout.Node
	Modified bool
	Italic   bool
	Close    string
	// W is the tab's width (Auto: its label's, or its share of a Fill
	// strip): a held strip's (see HoldTabs).
	W layout.Size
	// KeepEnd keeps that many bytes at the end of a cut-off label.
	KeepEnd int
}

// Tabs is a tab strip at the bar height on the chrome surface: the active
// tab sits on the content surface (or Surface, the panel's below) in strong text under an accent edge
// (Box.Edge, in a window), the others in secondary text, with a hairline
// between tabs (TabGap) so each reads as its own.
// In a window the strip and each tab round their top corners (Box.Flat),
// standing on the panel below. Each tab's node ID is Tab.ID.
//
// Trailing nodes (a new-tab button) follow the tabs; End nodes (where the
// shown file is) sit at the strip's far end, so they never look like one
// more tab. Fill shares the strip's width equally between the tabs (too
// many to fit at their names' width): long names end in "…".
//
// Scroll keeps tabs readable however many there are: a strip wider than its
// room scrolls sideways instead of squeezing its tabs, and Fill never makes
// one narrower than MinTabCols. Offset is app state, in columns: clamp it
// with layout.ClampScroll over the rects of ViewID and ContentID, and bring
// a tab into view with layout.ScrollToShow. It needs an ID.
type Tabs struct {
	ID       string
	Tabs     []Tab
	Active   int
	Fill     bool
	Trailing []layout.Node
	End      []layout.Node
	Scroll   bool
	Offset   int
	// Held is a strip whose tabs keep the widths they were given (Tab.W)
	// after one was closed, until the pointer leaves: it reports that
	// (layout.Box.ReportLeave, under ID), ignores Fill, and its content
	// stays at least Held columns wide so that nothing scrolls.
	Held int
	// Surface is the active tab's fill when the panel the strip stands on
	// is not the content surface.
	Surface layout.Fill
	// Ground gives every tab, the active one too (unless Surface says
	// otherwise), a fill of its own on an unfilled strip, with a plain gap
	// (DetachedTabGap) between them instead of a hairline.
	Ground layout.Fill
	// Unfocused marks the active tab's edge in the neutral FillBorder
	// instead of the accent: the strip's pane does not have focus, so
	// another pane's tab is the accented one.
	Unfocused bool
}

// Tab geometry: the hairline between tabs and the gap between those with a
// Ground, in points (one column in a terminal), and the narrowest tab of a
// scrolling Fill strip, in columns.
const (
	TabGap         = 1
	DetachedTabGap = 4
	MinTabCols     = 18
)

// Gap is the room between the strip's tabs, in points.
func (t Tabs) Gap() int {
	if t.Ground != layout.FillNone {
		return DetachedTabGap
	}
	return TabGap
}

// ViewID is the ID of a scrolling strip's window, ContentID of everything
// in it.
func (t Tabs) ViewID() string    { return t.ID + ".view" }
func (t Tabs) ContentID() string { return t.ID + ".content" }

// HoldTabs is the widths to hold a strip's tabs at after tab i, of the tabs
// with these widths (the gap between them gap), was closed by its close
// action: the tab taking its place is as wide as it was, so the next close
// action is right under the pointer, as in a browser. Past the end there is
// no next tab: the new last one reaches as far, at most limit wide, and the
// tabs of a Fill strip share the room.
func HoldTabs(widths []float64, i int, gap, limit float64, fill bool) []float64 {
	n := len(widths)
	if i < 0 || i >= n {
		return nil
	}
	out := append([]float64{}, widths...)
	if i < n-1 {
		return append(out[:i+1], out[i+2:]...)
	}
	out = out[:i]
	if i == 0 {
		return out
	}
	if !fill {
		out[i-1] = max(out[i-1], min(out[i-1]+gap+widths[i], limit))
		return out
	}
	total := 0.0
	for _, w := range widths {
		total += w
	}
	for j := range out {
		out[j] = (total + gap) / float64(i)
	}
	return out
}

func (t Tabs) Layout() layout.Node {
	if t.Held > 0 {
		t.Fill = false
	}
	tabs := make([]layout.Node, 0, 2*len(t.Tabs))
	for i, tab := range t.Tabs {
		if i > 0 {
			line := layout.Box{Fill: layout.FillSeparator}
			if t.Ground != layout.FillNone {
				line = layout.Box{}
			}
			tabs = append(tabs, layout.Col{NodeBase: layout.NodeBase{W: layout.Points(t.Gap()), H: layout.Grow(1), NoPadding: true, Box: line}})
		}
		tabs = append(tabs, t.tab(tab, i == t.Active))
	}
	trailing := make([]layout.Node, 0, len(t.Trailing))
	center := func(n layout.Node) layout.Node { // on the strip's height, not stretched
		return layout.Row{NodeBase: layout.NodeBase{H: layout.Grow(1), NoPadding: true}, Align: layout.AlignCenter, Children: []layout.Node{n}}
	}
	for _, n := range t.Trailing {
		trailing = append(trailing, center(n))
	}
	var kids []layout.Node
	if t.Scroll {
		content := layout.Row{NodeBase: layout.NodeBase{ID: t.ContentID(), H: layout.Grow(1)}, Align: layout.AlignStretch, Children: tabs}
		if t.Fill { // equal shares, at least MinTabCols each
			content.W = layout.Size{Kind: layout.SizeContent, Value: len(t.Tabs) * MinTabCols}
		} else {
			if t.Held > 0 {
				content.W = layout.Size{Kind: layout.SizeContent, Value: t.Held}
			}
			// The trailing nodes scroll along, right after the tabs.
			content.Children = append(append(content.Children, trailing...), &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
			trailing = nil
		}
		kids = []layout.Node{layout.Scroll{NodeBase: layout.NodeBase{ID: t.ViewID(), W: layout.Grow(1), H: layout.Grow(1)}, ScrollX: true, OffsetX: t.Offset, Child: content}}
	} else {
		kids = tabs
	}
	kids = append(kids, trailing...)
	if !t.Fill && !t.Scroll {
		kids = append(kids, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	}
	if len(t.End) > 0 {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{H: layout.Grow(1), PadLeft: 1, PadRight: 1, Gap: 1}, Align: layout.AlignCenter, Children: t.End})
	}
	box := layout.Box{Fill: layout.FillChrome, Round: true, Flat: layout.SideBottom}
	if t.Ground != layout.FillNone {
		box = layout.Box{}
	}
	box.ReportLeave = t.Held > 0
	return layout.Row{NodeBase: layout.NodeBase{ID: t.ID, H: layout.BarSize(), Box: box}, Align: layout.AlignStretch, Children: kids}
}

func (t Tabs) tab(tab Tab, active bool) layout.Node {
	style := layout.TextStyle{Color: layout.ColorSecondary}
	box := layout.Box{Fill: t.Ground, Interactive: !active, Round: true, Flat: layout.SideBottom}
	if active {
		style = layout.TextStyle{Role: layout.TextStrong}
		box.Edge = layout.SideTop
		if t.Ground == layout.FillNone {
			box.Fill = layout.FillContent
		}
		if t.Surface != layout.FillNone {
			box.Fill = t.Surface
		}
		if t.Unfocused {
			box.EdgeFill = layout.FillBorder
		}
	}
	style.Italic = tab.Italic
	parts := append([]layout.Node{}, tab.Leading...)
	if tab.Icon != layout.IconNone {
		parts = append(parts, icon(tab.Icon, layout.ColorSecondary))
	}
	label := layout.Label{Text: tab.Label, Style: style, Truncate: true, KeepEnd: tab.KeepEnd}
	w := layout.Auto()
	if t.Fill {
		label.W, w = layout.Grow(1), layout.Grow(1)
	}
	if tab.W.Kind != layout.SizeAuto {
		label.W, w = layout.Grow(1), tab.W
	}
	parts = append(parts, label)
	switch {
	case tab.Modified:
		parts = append(parts, Action{ID: tab.Close, Icon: layout.IconDot, Glyph: "●", Tooltip: "Unsaved — close"}.Layout())
	case tab.Close != "":
		parts = append(parts, Action{ID: tab.Close, Icon: layout.IconClose, Glyph: "✕", Tooltip: "Close"}.Layout())
	}
	pad := 1
	if tab.Close != "" || tab.Modified {
		pad = 0 // the action's own box is the right margin
	}
	return layout.Row{
		NodeBase: layout.NodeBase{ID: tab.ID, W: w, H: layout.Grow(1), Gap: 1, PadLeft: 1, PadRight: pad, Box: box},
		Align:    layout.AlignCenter, Children: parts,
	}
}

// Action is a small icon button inside a row, a tab or a pane header — a
// tab's close, a row's reveal: in a window a compact square with the hover
// wash under its icon, in a terminal its glyph. Its node ID is ID.
type Action struct {
	ID      string
	Icon    layout.IconName
	Glyph   string // the terminal spelling ("" : the icon's glyph)
	Tooltip string
}

func (a Action) Layout() layout.Node {
	g := a.Glyph
	if g == "" {
		g = glyph(a.Icon)
	}
	return layout.Row{
		NodeBase: layout.NodeBase{ID: a.ID, W: layout.Points(actionPoints), NoPadding: true, Tooltip: a.Tooltip, Box: layout.Box{Round: true, Interactive: a.ID != ""}},
		Align:    layout.AlignCenter,
		Children: []layout.Node{
			&layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}},
			layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(actionIconPoints)}, Name: a.Icon, Text: g, Style: layout.TextStyle{Color: layout.ColorSecondary}},
			&layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}},
		},
	}
}

// An Action's box and icon widths in a window (two cells each in a
// terminal: the glyph and a space).
const (
	actionPoints     = 16
	actionIconPoints = 10
)

// Segmented is a choice of a few options shown side by side, the chosen
// one filled. Option i's node ID is ID + ":" + its index (OptionID).
type Segmented struct {
	ID       string
	Options  []string
	Selected int
	Focused  bool
}

// OptionID is the node ID of a Segmented's option i.
func OptionID(id string, i int) string { return id + ":" + strconv.Itoa(i) }

func (s Segmented) Layout() layout.Node {
	kids := make([]layout.Node, len(s.Options))
	for i, o := range s.Options {
		box := layout.Box{Round: true, Interactive: i != s.Selected}
		style := layout.TextStyle{Color: layout.ColorSecondary}
		if i == s.Selected {
			box.Fill = layout.FillRaised
			box.Selected = s.Focused
			style = layout.TextStyle{Role: layout.TextStrong}
		}
		// The option fills the track's height and centers its label in it
		// (a Label draws its text at the top of its rect).
		kids[i] = layout.Row{
			NodeBase: layout.NodeBase{ID: OptionID(s.ID, i), PadLeft: 1, PadRight: 1, NoPadding: true, Box: box},
			Align:    layout.AlignCenter,
			Children: []layout.Node{layout.Label{Text: o, Style: style}},
		}
	}
	return layout.Row{
		NodeBase: layout.NodeBase{ID: s.ID, H: layout.ControlSize(), NoPadding: true, Box: layout.Box{Fill: layout.FillPressed, Round: true}},
		Align:    layout.AlignStretch, Children: kids,
	}
}

// Badge is a short status label on its tone's tint: a count, a state.
type Badge struct {
	Text string
	Tone Tone
}

func (b Badge) Layout() layout.Node {
	return layout.Label{
		NodeBase: layout.NodeBase{PadLeft: 1, PadRight: 1, NoPadding: true, Box: layout.Box{Fill: b.Tone.fill(), Round: true}},
		Text:     b.Text, Style: layout.TextStyle{Role: layout.TextMeta, Color: b.Tone.color()},
	}
}

// Setting is a settings row: a name with an optional description under
// it, and the control (a toggle, a segmented choice, a field) at the right.
type Setting struct {
	ID          string
	Name        string
	Description string
	Control     layout.Node
}

func (s Setting) Layout() layout.Node {
	text := []layout.Node{layout.Label{Text: s.Name, Style: layout.TextStyle{Role: layout.TextStrong}, Wrap: true}}
	if s.Description != "" {
		text = append(text, layout.Label{Text: s.Description, Style: layout.TextStyle{Role: layout.TextMeta}, Wrap: true})
	}
	kids := []layout.Node{layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Children: text}}
	if s.Control != nil {
		kids = append(kids, s.Control)
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: s.ID, Gap: 2, PadLeft: 1, PadRight: 1}, Align: layout.AlignCenter, Children: kids}
}

// EmptyState fills a pane with nothing to show: an icon, a title, a hint
// and actions (buttons), centered.
type EmptyState struct {
	ID      string
	Icon    layout.IconName
	Title   string
	Hint    string
	Actions []layout.Node
	// Inline sets it at the top left with the hint wrapping, for a narrow
	// sidebar, instead of centered in the pane. It adds no inset: put it
	// in the pane's body, which has layout.NodeBase.PanePadding.
	Inline bool
}

func (e EmptyState) Layout() layout.Node {
	kids := []layout.Node{}
	if e.Icon != layout.IconNone {
		kids = append(kids, icon(e.Icon, layout.ColorMuted))
	}
	if e.Title != "" {
		role := layout.TextTitle
		if e.Inline { // a sidebar's title is strong body text, not a page title
			role = layout.TextStrong
		}
		kids = append(kids, layout.Label{Text: e.Title, Style: layout.TextStyle{Role: role}})
	}
	if e.Hint != "" {
		kids = append(kids, layout.Label{Text: e.Hint, Style: layout.TextStyle{Role: layout.TextMeta}, Wrap: e.Inline})
	}
	if len(e.Actions) > 0 {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: e.Actions})
	}
	if e.Inline {
		return layout.Col{NodeBase: layout.NodeBase{ID: e.ID, W: layout.Grow(1), Gap: 1}, Align: layout.AlignStart, Children: kids}
	}
	return layout.Col{
		NodeBase: layout.NodeBase{ID: e.ID, W: layout.Grow(1), H: layout.Grow(1)},
		Align:    layout.AlignCenter,
		Children: []layout.Node{
			&layout.Spacer{NodeBase: layout.NodeBase{H: layout.Grow(1)}},
			layout.Col{NodeBase: layout.NodeBase{Gap: 1}, Align: layout.AlignCenter, Children: kids},
			&layout.Spacer{NodeBase: layout.NodeBase{H: layout.Grow(1)}},
		},
	}
}

// Popover is a raised panel over the frame: a quick-open list, a menu of
// choices. Anchor places it under the node with that ID (empty: centered);
// Width is its width (zero: its content's).
type Popover struct {
	ID       string
	Anchor   string
	Width    layout.Size
	Children []layout.Node
}

func (p Popover) Layout() layout.Node {
	return layout.Col{
		// The border band is the inset: no padding on top of it.
		NodeBase: layout.NodeBase{ID: p.ID, Overlay: true, Anchor: p.Anchor, W: p.Width, NoPadding: true,
			Box: layout.Box{Border: layout.BorderRounded, Fill: layout.FillRaised}},
		Children: p.Children,
	}
}

// icon is a component's icon: the shape in a GUI, 16pt wide, and its
// glyph in a terminal.
func icon(name layout.IconName, c layout.TextColor) layout.Node {
	return layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(16)}, Name: name, Text: glyph(name), Style: layout.TextStyle{Color: c}}
}

// glyph is an icon's terminal spelling.
func glyph(name layout.IconName) string {
	switch name {
	case layout.IconFile:
		return "▤"
	case layout.IconFolder:
		return "▸"
	case layout.IconDot:
		return "●"
	case layout.IconCircle:
		return "○"
	case layout.IconCheck:
		return "✓"
	case layout.IconClose:
		return "×"
	case layout.IconPlus:
		return "+"
	case layout.IconMinus:
		return "−"
	case layout.IconSearch:
		return "⌕"
	case layout.IconWarning:
		return "!"
	case layout.IconError:
		return "✕"
	case layout.IconInfo:
		return "i"
	case layout.IconGear:
		return "⚙"
	case layout.IconBranch:
		return "⎇"
	case layout.IconTerminal:
		return ">_"
	case layout.IconChevronRight, layout.IconDisclosureClosed:
		return "›"
	case layout.IconChevronDown, layout.IconDisclosureOpen:
		return "⌄"
	}
	return "•"
}
