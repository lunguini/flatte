package dockerapp

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type Volume struct {
	Name, Driver, Mountpoint, Size string
}

var sampleVolumes = []Volume{
	{Name: "data", Driver: "local", Mountpoint: "/var/lib/docker/volumes/data/_data", Size: "1.2GB"},
	{Name: "config", Driver: "local", Mountpoint: "/var/lib/docker/volumes/config/_data", Size: "12MB"},
	{Name: "logs", Driver: "local", Mountpoint: "/var/lib/docker/volumes/logs/_data", Size: "450MB"},
	{Name: "cache", Driver: "local", Mountpoint: "/var/lib/docker/volumes/cache/_data", Size: "890MB"},
	{Name: "backups", Driver: "local", Mountpoint: "/var/lib/docker/volumes/backups/_data", Size: "5.4GB"},
}

type volumesScreen struct {
	width         int
	height        int
	bodyYOffset   int
	focus         flatui.FocusRing
	list          flatui.List
	volumes       []Volume
	listPaneWidth int
	rects         map[string]layout.Rect
	drag          *splitDrag
}

const (
	volFocusList = iota
	volFocusDetail
)

func newVolumesScreen() *volumesScreen {
	s := &volumesScreen{volumes: sampleVolumes}
	s.focus.SetCount(2)
	s.focus.Select(volFocusList)
	s.list.SetCount(len(sampleVolumes))
	return s
}

// --- Layout nodes: the volumes body is a list | divider | detail row. ---

type volumeListPane struct {
	layout.NodeBase
	s *volumesScreen
}

func (p volumeListPane) Render(r layout.Rect) string {
	return p.s.renderVolumeListContent(r.W, r.H)
}

type volumeDivider struct {
	layout.NodeBase
	s *volumesScreen
}

func (p volumeDivider) Render(r layout.Rect) string {
	return renderDragDivider(r.H, p.s.drag != nil)
}

type volumeDetailPane struct {
	layout.NodeBase
	s *volumesScreen
}

func (p volumeDetailPane) Render(r layout.Rect) string {
	return p.s.renderVolumeDetailContent(r.W, r.H)
}

// bodyNode returns the volumes body subtree as a frame child that fills the body.
func (v *volumesScreen) bodyNode() layout.Node {
	return layout.Row{
		NodeBase: layout.NodeBase{H: layout.Grow(1)},
		Children: []layout.Node{
			volumeListPane{NodeBase: layout.NodeBase{ID: "list", W: layout.Fixed(v.listPaneWidth)}, s: v},
			volumeDivider{NodeBase: layout.NodeBase{ID: "div0", W: layout.Fixed(dividerWidth)}, s: v},
			volumeDetailPane{NodeBase: layout.NodeBase{ID: "detail", W: layout.Grow(1)}, s: v},
		},
	}
}

// initLayout sets body dimensions and the user-owned list width (no solve).
func (v *volumesScreen) initLayout(width, height, bodyYOffset int) {
	v.width, v.height = width, height
	v.bodyYOffset = bodyYOffset
	v.focus.SetCount(2)
	if v.listPaneWidth == 0 {
		v.listPaneWidth = min(30, max(v.width/3, minListWidth))
	}
}

// adopt consumes solved frame geometry (absolute) for hit-testing and sizing.
func (v *volumesScreen) adopt(rects map[string]layout.Rect) {
	v.rects = rects
	v.list.SetHeight(max(rects["list"].H-paneBorderRows-2, 0))
}

func (v *volumesScreen) Handle(s *State, ev flatte.Event, _ flatte.Effects[State]) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		v.handleMouse(s, m)
		return
	}
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			v.focus.Prev()
		} else {
			v.focus.Next()
		}
	case flatte.KeyUp:
		if v.focus.Focused(volFocusList) {
			v.list.MoveUp()
		}
	case flatte.KeyDown:
		if v.focus.Focused(volFocusList) {
			v.list.MoveDown()
		}
	case flatte.KeyCharacter:
		if v.focus.Focused(volFocusList) {
			switch key.Rune {
			case 'j', 'J':
				v.list.MoveDown()
			case 'k', 'K':
				v.list.MoveUp()
			}
		}
	}
}

func (v *volumesScreen) handleMouse(root *State, m flatte.MouseEvent) {
	if v.drag != nil {
		if m.Action == flatte.MouseRelease || (m.Button == flatte.MouseNone && m.Action != flatte.MouseMotion) {
			v.drag = nil
			return
		}
		if m.Action == flatte.MouseMotion {
			delta := m.X - v.drag.startX
			newList := v.drag.startWidth + delta
			maxList := v.width - minDetailWidth - dividerWidth
			v.listPaneWidth = max(minListWidth, min(newList, maxList))
			root.refreshActiveRects()
			return
		}
		return
	}

	if m.Action == flatte.MousePress && v.rects["div0"].Contains(m.X, m.Y) {
		v.drag = &splitDrag{startX: m.X, startWidth: v.listPaneWidth}
		return
	}

	if listR := v.rects["list"]; listR.Contains(m.X, m.Y) {
		v.handleVolumeListMouse(m, m.X-listR.X, m.Y-listR.Y)
	}
}

func (v *volumesScreen) keyHints() string {
	if v.focus.Focused(volFocusList) {
		return "j/k move  tab next  1/2/3 switch  q quit"
	}
	return "tab next  1/2/3 switch  q quit"
}

func (v *volumesScreen) selected() *Volume {
	if v.list.Cursor() < 0 || v.list.Cursor() >= len(v.volumes) {
		return nil
	}
	return &v.volumes[v.list.Cursor()]
}

func (v *volumesScreen) renderVolumeListContent(paneWidth, paneHeight int) string {
	contentWidth := max(paneWidth-paneBorderCols-1, 1)
	contentHeight := max(paneHeight-paneBorderRows, 0)
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	scrollStyle := lipgloss.NewStyle().Height(contentHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render("volumes")
	content := v.list.View(func(idx int, selected bool) string {
		marker := "  "
		if selected {
			marker = "> "
		}
		name := v.volumes[idx].Name
		if selected {
			name = lipgloss.NewStyle().Bold(true).Render(name)
		}
		return marker + name
	})
	inner := style.Render(title + "\n\n" + content)
	visible := contentHeight - 2
	bar := strings.Repeat(" ", contentHeight)
	if v.list.Count() > visible {
		bar = scrollbarLines(v.list.Offset(), visible, v.list.Count(), contentHeight)
	}
	combined := withScrollbar(inner, scrollStyle.Foreground(pal.panel).Render(bar))
	return paneStyle(paneWidth, paneHeight, v.focus.Focused(volFocusList), false).Render(combined)
}

func (v *volumesScreen) renderVolumeDetailContent(paneWidth, paneHeight int) string {
	contentWidth := max(paneWidth-paneBorderCols, 1)
	contentHeight := max(paneHeight-paneBorderRows, 0)
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	sel := v.selected()
	if sel == nil {
		empty := lipgloss.NewStyle().Foreground(pal.muted).Render("(no volume selected)")
		return paneStyle(paneWidth, paneHeight, false, false).Render(empty)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(sel.Name)
	rows := []string{title, "",
		"  driver:     " + sel.Driver,
		"  mountpoint: " + sel.Mountpoint,
		"  size:       " + sel.Size}
	inner := style.Render(strings.Join(rows, "\n"))
	return paneStyle(paneWidth, paneHeight, v.focus.Focused(volFocusDetail), false).Render(inner)
}

func (v *volumesScreen) handleVolumeListMouse(m flatte.MouseEvent, localX, localY int) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		row := localY - 2
		if row >= 0 && row < v.list.Count() {
			v.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		v.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		v.list.MoveUp()
	}
}
