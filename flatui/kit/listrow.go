package kit

import (
	"strings"

	"github.com/lunguini/flatte/flatui/layout"
)

// Divider separates the groups of a list: the next group's title and a
// line after it (the line alone without a title), at the bar height so it stands away
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
	var kids []layout.Node
	if d.Title != "" {
		kids = append(kids, layout.Label{Text: d.Title, Style: layout.TextStyle{Role: layout.TextCaption}, Truncate: true})
	}
	kids = append(kids, layout.Rule{})
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
	// Plain keeps a selected row's title at the weight of the others' (its
	// background marks it); Strong makes a row's title bold, selected or not.
	Plain, Strong bool
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
		title.Color = layout.ColorDefault
		if !r.Plain {
			title.Role = layout.TextStrong
		}
	}
	if r.Strong {
		title.Role = layout.TextStrong
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
