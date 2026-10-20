package dockerapp

import (
	"strconv"
	"strings"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type Image struct {
	RepoTag, ID, Size, Created string
	Containers                 int
}

var sampleImages = []Image{
	{RepoTag: "nginx:1.25", ID: "sha256:a1b2c3d4", Size: "150MB", Created: "2026-06-15", Containers: 3},
	{RepoTag: "myapp/api:2.1", ID: "sha256:b2c3d4e5", Size: "210MB", Created: "2026-06-10", Containers: 2},
	{RepoTag: "postgres:16", ID: "sha256:c3d4e5f6", Size: "380MB", Created: "2026-05-20", Containers: 1},
	{RepoTag: "redis:7", ID: "sha256:d4e5f6g7", Size: "110MB", Created: "2026-05-15", Containers: 1},
	{RepoTag: "myapp/web:3.0", ID: "sha256:e5f6g7h8", Size: "280MB", Created: "2026-06-08", Containers: 0},
	{RepoTag: "myapp/worker:2.1", ID: "sha256:f6g7h8i9", Size: "190MB", Created: "2026-06-09", Containers: 1},
	{RepoTag: "myapp/migrate:1.4", ID: "sha256:g7h8i9j0", Size: "85MB", Created: "2026-06-05", Containers: 0},
}

type imagesScreen struct {
	width         int
	height        int
	bodyYOffset   int
	focus         flatui.FocusRing
	list          flatui.List
	images        []Image
	listPaneWidth int
	rects         map[string]layout.Rect
	drag          *splitDrag
	barDrag       scrollDrag
}

const (
	imgFocusList = iota
	imgFocusDetail
)

func newImagesScreen() *imagesScreen {
	s := &imagesScreen{images: append([]Image(nil), sampleImages...)}
	s.focus.SetCount(2)
	s.focus.Select(imgFocusList)
	s.list.SetCount(len(s.images))
	return s
}

// bodyNode returns the images body subtree — list | divider | detail — as
// a frame child that fills the body.
func (i *imagesScreen) bodyNode() layout.Node {
	sel := i.selected()
	title, fields := "", [][2]string(nil)
	if sel != nil {
		title, fields = sel.RepoTag, [][2]string{{"id", sel.ID}, {"size", sel.Size}, {"created", sel.Created}, {"containers", strconv.Itoa(sel.Containers)}}
	}
	return layout.Row{
		NodeBase: layout.NodeBase{H: layout.Grow(1)},
		Children: []layout.Node{
			namedListPane(i.listPaneWidth, "images", i.list, func(idx int) string { return i.images[idx].RepoTag }),
			dragDivider("div0", i.drag != nil),
			fieldsPane(title, fields, "(no image selected)"),
		},
	}
}

// initLayout sets body dimensions and the user-owned list width (no solve).
func (i *imagesScreen) initLayout(width, height, bodyYOffset int) {
	i.width, i.height = width, height
	i.bodyYOffset = bodyYOffset
	i.focus.SetCount(2)
	if i.listPaneWidth == 0 {
		i.listPaneWidth = min(30, max(i.width/3, minListWidth))
	}
}

// adopt consumes solved frame geometry (absolute) for hit-testing and sizing.
func (i *imagesScreen) adopt(rects map[string]layout.Rect) {
	i.rects = rects
	i.list.SetHeight(max(rects["list"].H-paneBorderRows-2, 0))
}

func (i *imagesScreen) Handle(s *State, ev flatte.Event, _ flatte.Effects[State]) {
	if m, ok := ev.(flatte.MouseEvent); ok {
		i.handleMouse(m)
		return
	}
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			i.focus.Prev()
		} else {
			i.focus.Next()
		}
	case flatte.KeyUp:
		if i.focus.Focused(imgFocusList) {
			i.list.MoveUp()
		}
	case flatte.KeyDown:
		if i.focus.Focused(imgFocusList) {
			i.list.MoveDown()
		}
	case flatte.KeyCharacter:
		if i.focus.Focused(imgFocusList) {
			switch key.Rune {
			case 'j', 'J':
				i.list.MoveDown()
			case 'k', 'K':
				i.list.MoveUp()
			}
		}
	}
}

func (i *imagesScreen) handleMouse(m flatte.MouseEvent) {
	if bar, y, h, handled := i.barDrag.event(m, listBarID); handled {
		if bar == listBarID {
			scrollList(&i.list, y, h)
		}
		return
	}
	if i.drag != nil {
		if m.Action == flatte.MouseRelease || (m.Button == flatte.MouseNone && m.Action != flatte.MouseMotion) {
			i.drag = nil
			return
		}
		if m.Action == flatte.MouseMotion {
			delta := m.X - i.drag.startX
			newList := i.drag.startWidth + delta
			maxList := i.width - minDetailWidth - dividerWidth
			i.listPaneWidth = max(minListWidth, min(newList, maxList))
			return
		}
		return
	}

	if m.Action == flatte.MousePress && m.Target == "div0" {
		i.drag = &splitDrag{startX: m.X, startWidth: i.listPaneWidth}
		return
	}

	if m.Target == "list" || strings.HasPrefix(m.Target, "list:") {
		i.handleImageListMouse(m)
	}
}

func (i *imagesScreen) keyHints() string {
	if i.focus.Focused(imgFocusList) {
		return "j/k move  tab next  1/2/3 switch  q quit"
	}
	return "tab next  1/2/3 switch  q quit"
}

func (i *imagesScreen) selected() *Image {
	if i.list.Cursor() < 0 || i.list.Cursor() >= len(i.images) {
		return nil
	}
	return &i.images[i.list.Cursor()]
}

// handleImageListMouse selects the clicked row (its ID names the item)
// and scrolls on the wheel.
func (i *imagesScreen) handleImageListMouse(m flatte.MouseEvent) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		if row, ok := listRowTarget(m.Target); ok {
			i.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		i.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		i.list.MoveUp()
	}
}
