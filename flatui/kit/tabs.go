package kit

import (
	"github.com/lunguini/flatte/flatui/layout"
)

// Tab is one tab of a Tabs strip. Modified shows the unsaved dot and the icon in the warning color; Italic
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
	// Plain keeps the active tab's name at the weight of the others': its
	// edge and color mark it. (A terminal draws no edge; leave it bold there.)
	Plain bool
	// Unfocused marks the active tab's edge in the neutral FillBorder
	// instead of the accent: the strip's pane does not have focus, so
	// another pane's tab is the accented one.
	Unfocused bool
	// Rest fills the room the tabs (and Trailing) leave before End, in the
	// frame they are drawn in, and never makes the strip scroll: put a
	// layout.Fit with a MinWidth there to leave it out when that room is too
	// little. A Fill strip has no room left, and no Rest.
	Rest layout.Node
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
			content.Children = append(append(content.Children, trailing...), t.rest())
			trailing = nil
		}
		kids = []layout.Node{layout.Scroll{NodeBase: layout.NodeBase{ID: t.ViewID(), W: layout.Grow(1), H: layout.Grow(1)}, ScrollX: true, OffsetX: t.Offset, Child: content}}
	} else {
		kids = tabs
	}
	kids = append(kids, trailing...)
	if !t.Fill && !t.Scroll {
		kids = append(kids, t.rest())
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

// rest is the room after an unfilled strip's tabs: Rest, padded as End is,
// or empty.
func (t Tabs) rest() layout.Node {
	if t.Rest == nil {
		return &layout.Spacer{NodeBase: layout.NodeBase{W: layout.Grow(1)}}
	}
	nb := layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), PadLeft: 1}
	if len(t.End) == 0 {
		nb.PadRight = 1
	}
	return layout.Row{NodeBase: nb, Align: layout.AlignCenter, Children: []layout.Node{t.Rest}}
}

func (t Tabs) tab(tab Tab, active bool) layout.Node {
	style := layout.TextStyle{Color: layout.ColorSecondary}
	box := layout.Box{Fill: t.Ground, Interactive: !active, Round: true, Flat: layout.SideBottom}
	if active {
		style = layout.TextStyle{Role: layout.TextStrong}
		if t.Plain {
			style = layout.TextStyle{Color: layout.ColorPrimary}
		}
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
		c := layout.ColorSecondary
		if tab.Modified {
			c = layout.ColorWarning
		}
		parts = append(parts, icon(tab.Icon, c))
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
		parts = append(parts, Action{ID: tab.Close, Icon: layout.IconDot, Glyph: "●", Tooltip: "Unsaved — close", Color: layout.ColorWarning}.Layout())
	case tab.Close != "":
		parts = append(parts, Action{ID: tab.Close, Icon: layout.IconClose, Glyph: "✕", Tooltip: "Close"}.Layout())
	}
	// The close action is inset from the right edge as the tab's start is
	// from the left.
	return layout.Row{
		NodeBase: layout.NodeBase{ID: tab.ID, W: w, H: layout.Grow(1), Gap: 1, PadLeft: 1, PadRight: 1, Box: box},
		Align:    layout.AlignCenter, Children: parts,
	}
}
