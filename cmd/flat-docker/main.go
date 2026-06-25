package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

type screen int

const (
	screenContainers screen = iota
	screenImages
	screenVolumes
)

func (sc screen) Name() string {
	switch sc {
	case screenContainers:
		return "containers"
	case screenImages:
		return "images"
	case screenVolumes:
		return "volumes"
	}
	return "unknown"
}

type State struct {
	width, height int
	screen        screen

	containers containersScreen
	images     imagesScreen
	volumes    volumesScreen
}

func NewState() *State {
	s := &State{screen: screenContainers}
	s.containers = newContainersScreen()
	return s
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.resize(e.Width, e.Height)
		return
	case flatte.KeyEvent:
		if handleGlobalKey(s, e, fx) {
			return
		}
	}

	switch s.screen {
	case screenContainers:
		s.containers.Handle(s, ev, fx)
	case screenImages:
		s.images.Handle(s, ev, fx)
	case screenVolumes:
		s.volumes.Handle(s, ev, fx)
	}
}

func handleGlobalKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) bool {
	if key.Key != flatte.KeyCharacter {
		return false
	}
	switch key.Rune {
	case '1':
		s.screen = screenContainers
		return true
	case '2':
		s.screen = screenImages
		return true
	case '3':
		s.screen = screenVolumes
		return true
	case 'q', 'Q':
		fx.Quit()
		return true
	}
	return false
}

const (
	chromeRowsTop    = 2
	chromeRowsBottom = 1
)

func (s *State) resize(width, height int) {
	s.width, s.height = width, height
	bodyWidth := width
	bodyHeight := max(height-chromeRowsTop-chromeRowsBottom, 0)
	s.containers.layout(bodyWidth, bodyHeight)
	s.images.layout(bodyWidth, bodyHeight)
	s.volumes.layout(bodyWidth, bodyHeight)
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	width := ctx.Width
	if s.width > 0 {
		width = s.width
	}

	header := renderTabBar(s, width)
	separator := strings.Repeat("─", width)
	footer := renderFooter(s, width)

	var body string
	switch s.screen {
	case screenContainers:
		body = s.containers.View(s)
	case screenImages:
		body = s.images.View(s)
	case screenVolumes:
		body = s.volumes.View(s)
	}
	content := strings.Join([]string{header, separator, body, separator, footer}, "\n")
	return flatte.Frame{
		Content: content,
		Title:   "flat-docker — " + s.screen.Name(),
	}
}

func renderTabBar(s *State, width int) string {
	labels := []struct {
		name   string
		active bool
	}{
		{"1 containers", s.screen == screenContainers},
		{"2 images", s.screen == screenImages},
		{"3 volumes", s.screen == screenVolumes},
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		text := " " + l.name + " "
		if l.active {
			text = "[" + l.name + "]"
		}
		if l.active {
			text = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")).Render(text)
		} else {
			text = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(text)
		}
		parts[i] = text
	}
	return strings.Join(parts, " ")
}

func renderFooter(s *State, width int) string {
	hints := ""
	switch s.screen {
	case screenContainers:
		hints = s.containers.keyHints()
	case screenImages:
		hints = "images screen"
	case screenVolumes:
		hints = "volumes screen"
	}
	help := " " + hints + " "
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(help)
	return styled
}

const (
	focusFilter = iota
	focusList
	focusDetail
)

type Container struct {
	ID, Name, Image, Status, Ports string
}

var sampleContainers = []Container{
	{ID: "a1b2c3d4e5", Name: "nginx-proxy", Image: "nginx:1.25", Status: "running", Ports: "0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp"},
	{ID: "b2c3d4e5f6", Name: "api-server", Image: "myapp/api:2.1", Status: "running", Ports: "0.0.0.0:8080->8080/tcp"},
	{ID: "c3d4e5f6g7", Name: "postgres", Image: "postgres:16", Status: "running", Ports: "5432/tcp"},
	{ID: "d4e5f6g7h8", Name: "redis-cache", Image: "redis:7", Status: "running", Ports: "6379/tcp"},
	{ID: "e5f6g7h8i9", Name: "frontend", Image: "myapp/web:3.0", Status: "exited", Ports: ""},
	{ID: "f6g7h8i9j0", Name: "worker", Image: "myapp/worker:2.1", Status: "running", Ports: ""},
	{ID: "g7h8i9j0k1", Name: "nginx-web", Image: "nginx:1.25", Status: "running", Ports: "80/tcp"},
	{ID: "h8i9j0k1l2", Name: "migration", Image: "myapp/migrate:1.4", Status: "exited", Ports: ""},
}

type containersScreen struct {
	width, height      int
	listPaneWidth      int
	detailPaneWidth    int

	focus      flatui.FocusRing
	filter     flatui.TextField
	list       flatui.List
	containers []Container
	filtered   []int
}

func newContainersScreen() containersScreen {
	c := containersScreen{containers: sampleContainers}
	c.focus.SetCount(3)
	c.focus.Select(focusList)
	return c
}

func (c *containersScreen) layout(width, height int) {
	c.width, c.height = width, height
	c.focus.SetCount(3)
	c.listPaneWidth = min(30, max(width/3, 16))
	c.detailPaneWidth = max(width-c.listPaneWidth-2, 0)
	const listChromeRows = 3
	c.list.SetHeight(max(height-listChromeRows, 0))
	if len(c.filtered) == 0 && len(c.containers) > 0 {
		c.refreshFilter()
	}
}

func (c *containersScreen) refreshFilter() {
	q := strings.ToLower(strings.TrimSpace(c.filter.Value))
	c.filtered = c.filtered[:0]
	for i, ct := range c.containers {
		haystack := strings.ToLower(ct.Name + " " + ct.Image)
		if q == "" || strings.Contains(haystack, q) {
			c.filtered = append(c.filtered, i)
		}
	}
	c.list.SetCount(len(c.filtered))
}

func (c *containersScreen) selected() *Container {
	if len(c.filtered) == 0 {
		return nil
	}
	idx := c.list.Cursor()
	if idx < 0 || idx >= len(c.filtered) {
		return nil
	}
	return &c.containers[c.filtered[idx]]
}

func (c *containersScreen) Handle(_ *State, ev flatte.Event, _ flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			c.focus.Prev()
		} else {
			c.focus.Next()
		}
	case flatte.KeyUp:
		if c.focus.Focused(focusList) {
			c.list.MoveUp()
		}
	case flatte.KeyDown:
		if c.focus.Focused(focusList) {
			c.list.MoveDown()
		}
	case flatte.KeyBackspace:
		if c.focus.Focused(focusFilter) {
			c.filter.Backspace()
			c.refreshFilter()
		}
	case flatte.KeyDelete:
		if c.focus.Focused(focusFilter) {
			c.filter.Delete()
			c.refreshFilter()
		}
	case flatte.KeyLeft:
		if c.focus.Focused(focusFilter) {
			c.filter.MoveLeft()
		}
	case flatte.KeyRight:
		if c.focus.Focused(focusFilter) {
			c.filter.MoveRight()
		}
	case flatte.KeyCharacter:
		c.handleChar(key.Rune)
	}
}

func (c *containersScreen) handleChar(r rune) {
	switch c.focus.Index() {
	case focusFilter:
		c.filter.Insert(r)
		c.refreshFilter()
	case focusList:
		switch r {
		case 'j', 'J':
			c.list.MoveDown()
		case 'k', 'K':
			c.list.MoveUp()
		}
	}
}

func (c *containersScreen) View(_ *State) string {
	listPane := c.renderListPane()
	detailPane := c.renderDetailPane()
	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, "  ", detailPane)
}

func (c *containersScreen) keyHints() string {
	switch c.focus.Index() {
	case focusFilter:
		return "type filter  tab next  1/2/3 switch  q quit"
	case focusList:
		return "j/k move  tab next  1/2/3 switch  q quit"
	case focusDetail:
		return "tab next  1/2/3 switch  q quit"
	}
	return "1/2/3 switch  q quit"
}

func (c *containersScreen) renderListPane() string {
	style := lipgloss.NewStyle().Width(c.listPaneWidth)

	filterLine := "filter: " + c.filter.Value
	if c.filter.Value == "" {
		filterLine = "filter: (all)"
	}
	if c.focus.Focused(focusFilter) {
		filterLine = lipgloss.NewStyle().Bold(true).Render(filterLine)
	}

	listContent := c.list.View(func(idx int, selected bool) string {
		if idx < 0 || idx >= len(c.filtered) {
			return ""
		}
		ct := c.containers[c.filtered[idx]]
		marker := "  "
		if selected {
			marker = "> "
		}
		statusIcon := "●"
		if ct.Status != "running" {
			statusIcon = "○"
		}
		return fmt.Sprintf("%s%s%s", statusIcon, marker, ct.Name)
	})
	if listContent == "" {
		listContent = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  (no matches)")
	}

	return style.Render(filterLine + "\n\n" + listContent)
}

func (c *containersScreen) renderDetailPane() string {
	style := lipgloss.NewStyle().Width(c.detailPaneWidth)

	selected := c.selected()
	if selected == nil {
		return style.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no container selected)"))
	}

	title := selected.Name
	if c.focus.Focused(focusDetail) {
		title = lipgloss.NewStyle().Bold(true).Render(title)
	}

	statusLine := selected.Status
	if selected.Status == "running" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Render(selected.Status)
	} else {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(selected.Status)
	}

	rows := []string{
		title,
		"",
		"  image:  " + selected.Image,
		"  status: " + statusLine,
		"  ports:  " + selected.Ports,
		"  id:     " + selected.ID,
	}
	return style.Render(strings.Join(rows, "\n"))
}

type imagesScreen struct {
	width, height int
}

func (i *imagesScreen) layout(width, height int) {
	i.width, i.height = width, height
}

func (i *imagesScreen) Handle(_ *State, _ flatte.Event, _ flatte.Effects[State]) {}

func (i *imagesScreen) View(root *State) string {
	return placeholderBody(root.screen.Name(), i.width, i.height)
}

type volumesScreen struct {
	width, height int
}

func (v *volumesScreen) layout(width, height int) {
	v.width, v.height = width, height
}

func (v *volumesScreen) Handle(_ *State, _ flatte.Event, _ flatte.Effects[State]) {}

func (v *volumesScreen) View(root *State) string {
	return placeholderBody(root.screen.Name(), v.width, v.height)
}

func placeholderBody(screenName string, width, height int) string {
	lines := []string{
		"",
		fmt.Sprintf("  %s screen", screenName),
		fmt.Sprintf("  body: %d wide × %d tall", width, height),
		"",
		"  placeholder — task 2 adds real content",
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func main() {
	state := NewState()
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
