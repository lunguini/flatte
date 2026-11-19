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
}

func (h PaneHeader) Layout() layout.Node {
	kids := []layout.Node{layout.Label{Text: h.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true}}
	if h.Meta != "" {
		kids = append(kids, layout.Label{Text: h.Meta, Style: layout.TextStyle{Role: layout.TextMeta}, Truncate: true})
	}
	kids = append(kids, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	kids = append(kids, h.Actions...)
	return layout.Row{
		NodeBase: layout.NodeBase{ID: h.ID, H: layout.BarSize(), Gap: 1, PadLeft: 1, PadRight: 1, Box: layout.Box{Fill: layout.FillChrome}},
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
		kids = append(kids, layout.Label{NodeBase: layout.NodeBase{PadLeft: 1}, Text: s.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true})
	}
	kids = append(kids, s.Children...)
	return layout.Col{NodeBase: layout.NodeBase{ID: s.ID, Gap: 1}, Children: kids}
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
	if r.Selected && r.Active {
		title.Role = layout.TextStrong
	}
	kids = append(kids, layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: r.Title, Style: title, Truncate: true})
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
	case r.Disabled || r.TitleColor == layout.ColorDisabled:
		return layout.ColorDisabled
	case r.Selected && r.Active:
		return layout.ColorAccent
	}
	return layout.ColorSecondary
}

// Tab is one tab of a Tabs strip. Close is its close action's ID (empty:
// none); Modified shows a dot in the close action's place (an unsaved
// file), still clickable by the same ID. Italic marks a preview tab.
type Tab struct {
	ID       string
	Label    string
	Icon     layout.IconName
	Modified bool
	Italic   bool
	Close    string
}

// Tabs is a tab strip at the bar height on the chrome surface: the active
// tab sits on the content surface in strong text under an accent edge
// (Box.Edge, in a window), the others in secondary text. Each tab's node
// ID is Tab.ID; Trailing nodes (a new-tab button, a breadcrumb) follow the
// tabs directly. Fill shares the strip's width equally between the tabs (too many
// to fit at their names' width): long names end in "…".
type Tabs struct {
	ID       string
	Tabs     []Tab
	Active   int
	Fill     bool
	Trailing []layout.Node
}

func (t Tabs) Layout() layout.Node {
	kids := make([]layout.Node, 0, len(t.Tabs)+len(t.Trailing)+1)
	for i, tab := range t.Tabs {
		active := i == t.Active
		style := layout.TextStyle{Color: layout.ColorSecondary}
		box := layout.Box{Interactive: !active}
		if active {
			style = layout.TextStyle{Role: layout.TextStrong}
			box.Fill, box.Edge = layout.FillContent, layout.SideTop
		}
		style.Italic = tab.Italic
		parts := []layout.Node{}
		if tab.Icon != layout.IconNone {
			parts = append(parts, icon(tab.Icon, layout.ColorSecondary))
		}
		label := layout.Label{Text: tab.Label, Style: style, Truncate: true}
		w := layout.Auto()
		if t.Fill {
			label.W, w = layout.Grow(1), layout.Grow(1)
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
		kids = append(kids, layout.Row{
			NodeBase: layout.NodeBase{ID: tab.ID, W: w, H: layout.Grow(1), Gap: 1, PadLeft: 1, PadRight: pad, Box: box},
			Align:    layout.AlignCenter, Children: parts,
		})
	}
	for _, n := range t.Trailing { // centered on the strip's height, not stretched
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{H: layout.Grow(1), NoPadding: true}, Align: layout.AlignCenter, Children: []layout.Node{n}})
	}
	if !t.Fill {
		kids = append(kids, &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}})
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: t.ID, H: layout.BarSize(), Box: layout.Box{Fill: layout.FillChrome}}, Align: layout.AlignStretch, Children: kids}
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
	text := []layout.Node{layout.Label{Text: s.Name, Style: layout.TextStyle{Role: layout.TextStrong}}}
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
}

func (e EmptyState) Layout() layout.Node {
	kids := []layout.Node{}
	if e.Icon != layout.IconNone {
		kids = append(kids, icon(e.Icon, layout.ColorMuted))
	}
	if e.Title != "" {
		kids = append(kids, layout.Label{Text: e.Title, Style: layout.TextStyle{Role: layout.TextTitle}})
	}
	if e.Hint != "" {
		kids = append(kids, layout.Label{Text: e.Hint, Style: layout.TextStyle{Role: layout.TextMeta}})
	}
	if len(e.Actions) > 0 {
		kids = append(kids, layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: e.Actions})
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
	case layout.IconTerminal:
		return ">_"
	case layout.IconChevronRight, layout.IconDisclosureClosed:
		return "›"
	case layout.IconChevronDown, layout.IconDisclosureOpen:
		return "⌄"
	}
	return "•"
}
