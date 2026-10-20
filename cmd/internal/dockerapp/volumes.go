package dockerapp

import (
	"strings"

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
	barDrag       scrollDrag
}

const (
	volFocusList = iota
	volFocusDetail
)

func newVolumesScreen() *volumesScreen {
	s := &volumesScreen{volumes: append([]Volume(nil), sampleVolumes...)}
	s.focus.SetCount(2)
	s.focus.Select(volFocusList)
	s.list.SetCount(len(s.volumes))
	return s
}

// bodyNode returns the volumes body subtree — list | divider | detail — as
// a frame child that fills the body.
func (v *volumesScreen) bodyNode() layout.Node {
	sel := v.selected()
	title, fields := "", [][2]string(nil)
	if sel != nil {
		title, fields = sel.Name, [][2]string{{"driver", sel.Driver}, {"mountpoint", sel.Mountpoint}, {"size", sel.Size}}
	}
	return layout.Row{
		NodeBase: layout.NodeBase{H: layout.Grow(1)},
		Children: []layout.Node{
			namedListPane(v.listPaneWidth, "volumes", v.list, func(idx int) string { return v.volumes[idx].Name }),
			dragDivider("div0", v.drag != nil),
			fieldsPane(title, fields, "(no volume selected)"),
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
		v.handleMouse(m)
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

func (v *volumesScreen) handleMouse(m flatte.MouseEvent) {
	if bar, y, h, handled := v.barDrag.event(m, listBarID); handled {
		if bar == listBarID {
			scrollList(&v.list, y, h)
		}
		return
	}
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
			return
		}
		return
	}

	if m.Action == flatte.MousePress && m.Target == "div0" {
		v.drag = &splitDrag{startX: m.X, startWidth: v.listPaneWidth}
		return
	}

	if m.Target == "list" || strings.HasPrefix(m.Target, "list:") {
		v.handleVolumeListMouse(m)
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

// handleVolumeListMouse selects the clicked row (its ID names the item)
// and scrolls on the wheel.
func (v *volumesScreen) handleVolumeListMouse(m flatte.MouseEvent) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		if row, ok := listRowTarget(m.Target); ok {
			v.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		v.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		v.list.MoveUp()
	}
}
