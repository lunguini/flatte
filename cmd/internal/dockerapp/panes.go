package dockerapp

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

const (
	dividerWidth     = 1
	minListWidth     = 20
	minDetailWidth   = 20
	minActivityWidth = 12
)

// splitDrag tracks a 2-pane divider drag (images/volumes screens).
type splitDrag struct {
	startX     int
	startWidth int
}

// renderDragDivider produces a 1-col-wide, height-tall divider with a
// background color matching the pane borders, and a single │ in the
// vertical center as a visible grip indicator. The bg fills the column
// (spaces are invisible against it); the │ stands out as a contrast.
func renderDragDivider(height int, dragging bool) string {
	bg := pal.panel
	gripFg := pal.bg // dark against gray — clearly visible
	if dragging {
		bg = pal.accent
		gripFg = pal.dark // dark against blue
	}
	midY := height / 2
	rows := make([]string, height)
	for i := 0; i < height; i++ {
		if i == midY {
			rows[i] = "│"
		} else {
			rows[i] = " "
		}
	}
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(dividerWidth).
		Height(height).
		MaxHeight(height).
		Foreground(gripFg).
		Background(bg).
		Render(content)
}

// --- Pane nodes shared by the screens ---

// paneBase is a pane's frame: padding on the sides and bottom (none on top,
// so pane headers line up across the body).
func paneBase(id string, w layout.Size) layout.NodeBase {
	return layout.NodeBase{ID: id, W: w, H: layout.Auto(), PadLeft: panePadding, PadRight: panePadding, PadBottom: panePadding}
}

// dragDivider is a draggable pane divider: a filled column with a grip mark
// halfway down, in the accent while it is being dragged.
func dragDivider(id string, dragging bool) layout.Node {
	bg, grip := pal.panel, pal.bg // dark against gray
	if dragging {
		bg, grip = pal.accent, pal.dark // dark against blue
	}
	return layout.Col{
		NodeBase: layout.NodeBase{ID: id, W: layout.Fixed(dividerWidth), H: layout.Auto(), Box: layout.Box{Background: bg}},
		Children: []layout.Node{
			layout.NewSpacer(),
			layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: lipgloss.NewStyle().Foreground(grip).Render("│")},
			layout.NewSpacer(),
		},
	}
}

// namedListPane is a screen's list pane ("list"): a bold title, a blank
// row, and the visible rows ("list:i": a selection marker and the name, bold
// when selected) beside the scrollbar column.
func namedListPane(width int, title string, list flatui.List, name func(i int) string) layout.Node {
	rows := list.Rows("list", func(i int, selected bool) layout.Node {
		marker, label := layout.Icon{Text: "  "}, layout.Label{Text: name(i)}
		if selected {
			marker, label.Style = layout.Icon{Name: layout.IconChevronRight, Text: "> "}, layout.TextStyle{Bold: true}
		}
		return layout.Row{Children: []layout.Node{marker, label}}
	})
	return layout.Col{
		NodeBase: paneBase("list", layout.Fixed(width)),
		Children: []layout.Node{layout.Row{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Children: []layout.Node{
				layout.Label{Text: title, Style: layout.TextStyle{Bold: true, Fg: pal.accent}},
				layout.Label{},
				rows,
			}},
			listBar(list),
		}}},
	}
}

// fieldsPane is a screen's detail pane ("detail"): a bold title, a blank
// row, and aligned "name: value" rows — or the empty placeholder when
// nothing is selected.
func fieldsPane(title string, fields [][2]string, empty string) layout.Node {
	pane := layout.Col{NodeBase: paneBase("detail", layout.Grow(1))}
	if title == "" {
		pane.Children = []layout.Node{layout.Label{Text: empty, Style: layout.TextStyle{Fg: pal.muted}}}
		return pane
	}
	pane.Children = []layout.Node{layout.Label{Text: title, Style: layout.TextStyle{Bold: true, Fg: pal.accent}}, layout.Label{}}
	for _, f := range fields {
		pane.Children = append(pane.Children, layout.Row{NodeBase: layout.NodeBase{PadLeft: 2}, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(fieldNameWidth)}, Text: f[0] + ":"},
			layout.Label{Text: f[1]},
		}})
	}
	return pane
}

// fieldNameWidth is the detail panes' name column: the longest name
// ("containers:", "mountpoint:") and a space.
const fieldNameWidth = 12

// listRowTarget is the item index a mouse target names in a list pane, if
// it is one of its rows ("list:i").
func listRowTarget(target string) (int, bool) {
	i, ok := strings.CutPrefix(target, "list:")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(i)
	return n, err == nil
}

// scrollDrag follows a press and drag on a scroll bar: the bar pressed
// keeps the drag, wherever the pointer goes, until the button is released.
type scrollDrag struct {
	id   string
	rect layout.Rect
}

// event reports whether m belongs to a scroll bar — a left press on one of
// ids, or any event during a drag — and, when it scrolls one, which bar
// and the line on it (node-local, of a bar height lines tall).
func (d *scrollDrag) event(m flatte.MouseEvent, ids ...string) (bar string, y, height int, handled bool) {
	if d.id != "" {
		if m.Action == flatte.MouseRelease || (m.Button == flatte.MouseNone && m.Action != flatte.MouseMotion) {
			d.id = ""
			return "", 0, 0, true
		}
		return d.id, m.Y - d.rect.Y, d.rect.H, m.Action == flatte.MouseMotion || m.Action == flatte.MousePress
	}
	if m.Action != flatte.MousePress || m.Button != flatte.MouseLeft || !slices.Contains(ids, m.Target) {
		return "", 0, 0, false
	}
	d.id, d.rect = m.Target, m.TargetRect
	return d.id, m.Y - d.rect.Y, d.rect.H, true
}

// listBarID names a list pane's scroll bar.
const listBarID = "listbar"

// listBar is a list's scroll bar.
func listBar(list flatui.List) layout.ScrollBar {
	return layout.ScrollBar{NodeBase: layout.NodeBase{ID: listBarID}, Offset: list.Offset(), Visible: list.Height(), Total: list.Count(), Color: pal.panel}
}

// scrollList scrolls list for a press or drag at line y of its bar,
// reporting whether the cursor moved with it.
func scrollList(list *flatui.List, y, height int) bool {
	before := list.Cursor()
	list.ScrollTo(listBar(*list).OffsetAt(y, height))
	return list.Cursor() != before
}
