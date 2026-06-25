package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
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
	return &State{screen: screenContainers}
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
	help := " 1/2/3 switch  q quit "
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(help)
	return styled
}

type containersScreen struct {
	width, height int
}

func (c *containersScreen) layout(width, height int) {
	c.width, c.height = width, height
}

func (c *containersScreen) Handle(_ *State, _ flatte.Event, _ flatte.Effects[State]) {}

func (c *containersScreen) View(root *State) string {
	return placeholderBody(root.screen.Name(), c.width, c.height)
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
