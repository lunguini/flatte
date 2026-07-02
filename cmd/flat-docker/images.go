package main

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

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
}

const (
	imgFocusList = iota
	imgFocusDetail
)

func newImagesScreen() *imagesScreen {
	s := &imagesScreen{images: sampleImages}
	s.focus.SetCount(2)
	s.focus.Select(imgFocusList)
	s.list.SetCount(len(sampleImages))
	return s
}

// --- Layout nodes: the images body is a list | divider | detail row. ---

type imageListPane struct {
	layout.NodeBase
	s *imagesScreen
}

func (p imageListPane) Render(r layout.Rect) string {
	return p.s.renderImageListContent(r.W, r.H)
}

type imageDivider struct {
	layout.NodeBase
	s *imagesScreen
}

func (p imageDivider) Render(r layout.Rect) string {
	return renderDragDivider(r.H, p.s.drag != nil)
}

type imageDetailPane struct {
	layout.NodeBase
	s *imagesScreen
}

func (p imageDetailPane) Render(r layout.Rect) string {
	return p.s.renderImageDetailContent(r.W, r.H)
}

// bodyNode returns the images body subtree as a frame child that fills the body.
func (i *imagesScreen) bodyNode() layout.Node {
	return layout.Row{
		NodeBase: layout.NodeBase{H: layout.Grow(1)},
		Children: []layout.Node{
			imageListPane{NodeBase: layout.NodeBase{ID: "list", W: layout.Fixed(i.listPaneWidth)}, s: i},
			imageDivider{NodeBase: layout.NodeBase{ID: "div0", W: layout.Fixed(dividerWidth)}, s: i},
			imageDetailPane{NodeBase: layout.NodeBase{ID: "detail", W: layout.Grow(1)}, s: i},
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
		i.handleMouse(s, m)
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

func (i *imagesScreen) handleMouse(root *State, m flatte.MouseEvent) {
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
			root.refreshActiveRects()
			return
		}
		return
	}

	if m.Action == flatte.MousePress && i.rects["div0"].Contains(m.X, m.Y) {
		i.drag = &splitDrag{startX: m.X, startWidth: i.listPaneWidth}
		return
	}

	if listR := i.rects["list"]; listR.Contains(m.X, m.Y) {
		i.handleImageListMouse(m, m.X-listR.X, m.Y-listR.Y)
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

func (i *imagesScreen) renderImageListContent(paneWidth, paneHeight int) string {
	contentWidth := max(paneWidth-paneBorderCols-1, 1)
	contentHeight := max(paneHeight-paneBorderRows, 0)
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	scrollStyle := lipgloss.NewStyle().Height(contentHeight)
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render("images")
	content := i.list.View(func(idx int, selected bool) string {
		marker := "  "
		if selected {
			marker = "> "
		}
		name := i.images[idx].RepoTag
		if selected {
			name = lipgloss.NewStyle().Bold(true).Render(name)
		}
		return marker + name
	})
	inner := style.Render(title + "\n\n" + content)
	visible := contentHeight - 2
	bar := strings.Repeat(" ", contentHeight)
	if i.list.Count() > visible {
		bar = scrollbarLines(i.list.Offset(), visible, i.list.Count(), contentHeight)
	}
	combined := withScrollbar(inner, scrollStyle.Foreground(pal.panel).Render(bar))
	return paneStyle(paneWidth, paneHeight, i.focus.Focused(imgFocusList), false).Render(combined)
}

func (i *imagesScreen) renderImageDetailContent(paneWidth, paneHeight int) string {
	contentWidth := max(paneWidth-paneBorderCols, 1)
	contentHeight := max(paneHeight-paneBorderRows, 0)
	style := lipgloss.NewStyle().Width(contentWidth).Height(contentHeight)
	sel := i.selected()
	if sel == nil {
		empty := lipgloss.NewStyle().Foreground(pal.muted).Render("(no image selected)")
		return paneStyle(paneWidth, paneHeight, false, false).Render(empty)
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(sel.RepoTag)
	rows := []string{title, "",
		"  id:         " + sel.ID,
		"  size:       " + sel.Size,
		"  created:    " + sel.Created,
		"  containers: " + strconv.Itoa(sel.Containers)}
	inner := style.Render(strings.Join(rows, "\n"))
	return paneStyle(paneWidth, paneHeight, i.focus.Focused(imgFocusDetail), false).Render(inner)
}

func (i *imagesScreen) handleImageListMouse(m flatte.MouseEvent, localX, localY int) {
	switch {
	case m.Action == flatte.MousePress && m.Button == flatte.MouseLeft:
		row := localY - 2
		if row >= 0 && row < i.list.Count() {
			i.list.Select(row)
		}
	case m.Button == flatte.MouseWheelDown:
		i.list.MoveDown()
	case m.Button == flatte.MouseWheelUp:
		i.list.MoveUp()
	}
}
