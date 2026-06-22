package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

type delivery struct {
	name   string
	owner  string
	status string
}

var deliveries = []delivery{
	{name: "deploy-api", owner: "platform", status: "ready"},
	{name: "billing-sync", owner: "finance", status: "watch"},
	{name: "edge-cache", owner: "infra", status: "ready"},
	{name: "search-index", owner: "growth", status: "blocked"},
}

type State struct {
	list     flatui.List
	progress flatui.Progress
}

func NewState() *State {
	s := &State{progress: flatui.NewProgress(16)}
	s.progress.SetPercent(70)
	s.list.SetCount(len(deliveries))
	s.list.SetHeight(4)
	return s
}

func (s *State) layout(width, height int) {
	s.list.SetHeight(max(min(height-12, len(deliveries)), 1))
	s.progress.SetWidth(max(width/4, 8))
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(ev.Width, ev.Height)
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	switch key.Key {
	case flatte.KeyDown:
		s.list.MoveDown()
	case flatte.KeyUp:
		s.list.MoveUp()
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'j', 'J':
			s.list.MoveDown()
		case 'k', 'K':
			s.list.MoveUp()
		case 'h', 'H':
			s.progress.SetPercent(s.progress.Percent() - 10)
		case 'l', 'L':
			s.progress.SetPercent(s.progress.Percent() + 10)
		case 'q', 'Q':
			fx.Quit()
		}
	}
}

type palette struct {
	base     color.Color
	muted    color.Color
	panel    color.Color
	accent   color.Color
	good     color.Color
	warn     color.Color
	bad      color.Color
	selected color.Color
}

func defaultPalette() palette {
	return palette{
		base:     lipgloss.Color("252"),
		muted:    lipgloss.Color("245"),
		panel:    lipgloss.Color("238"),
		accent:   lipgloss.Color("117"),
		good:     lipgloss.Color("114"),
		warn:     lipgloss.Color("222"),
		bad:      lipgloss.Color("203"),
		selected: lipgloss.Color("229"),
	}
}

type styles struct {
	title    lipgloss.Style
	subtle   lipgloss.Style
	section  lipgloss.Style
	panel    lipgloss.Style
	status   lipgloss.Style
	selected lipgloss.Style
	good     lipgloss.Style
	warn     lipgloss.Style
	bad      lipgloss.Style
}

func newStyles(p palette) styles {
	base := lipgloss.NewStyle()
	return styles{
		title: base.
			Bold(true).
			Foreground(p.accent),
		subtle: base.
			Foreground(p.muted),
		section: base.
			Bold(true).
			Foreground(p.base),
		panel: base.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.panel).
			Padding(0, 1),
		status: base.
			Foreground(p.base).
			Background(p.panel),
		selected: base.
			Bold(true).
			Foreground(p.selected),
		good: base.
			Foreground(p.good),
		warn: base.
			Foreground(p.warn),
		bad: base.
			Foreground(p.bad),
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	st := newStyles(defaultPalette())
	width := max(ctx.Width, 40)
	bodyWidth := max(width-4, 36)
	leftOuter := max((bodyWidth-2)*2/3, 24)
	rightOuter := max(bodyWidth-2-leftOuter, 18)
	if leftOuter+rightOuter+2 > bodyWidth {
		leftOuter = max(bodyWidth-rightOuter-2, 18)
	}

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		st.title.Width(max(bodyWidth-18, 12)).Render("Flat Style"),
		st.status.Render(fmt.Sprintf(" Delivery %3.0f%%", s.progress.Percent())),
	)

	left := st.panel.Width(leftOuter - 2).Render(deliveryPanel(s, st, leftOuter-4))
	right := st.panel.Width(rightOuter - 2).Render(palettePanel(st, rightOuter-4))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)

	footer := st.subtle.Render("j/k move  h/l progress  q quit")
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
	return flatte.Frame{Content: trimRightLines(content)}
}

func deliveryPanel(s *State, st styles, width int) string {
	rows := []string{st.section.Render("Delivery"), ""}
	list := s.list.View(func(index int, selected bool) string {
		item := deliveries[index]
		row := fmt.Sprintf("%-14s %-8s %s", item.name, item.owner, statusLabel(item.status, st))
		row = truncate(row, width)
		if selected {
			return st.selected.Render("> " + row)
		}
		return "  " + row
	})
	if list != "" {
		rows = append(rows, strings.Split(list, "\n")...)
	}
	rows = append(rows, "", truncate(s.progress.ViewWithStyle(flatui.ProgressStyle{
		Filled: st.good,
		Empty:  st.subtle,
		Label:  st.section,
	}), width))
	return strings.Join(rows, "\n")
}

func palettePanel(st styles, width int) string {
	rows := []string{
		st.section.Render("Palette"),
		"",
		st.good.Render("ready") + "   deploy",
		st.warn.Render("watch") + "   review",
		st.bad.Render("blocked") + " stop",
		"",
		st.subtle.Render("Local palette"),
		st.subtle.Render("App-owned styles"),
	}
	return strings.Join(rows, "\n")
}

func statusLabel(status string, st styles) string {
	switch status {
	case "ready":
		return st.good.Render(status)
	case "watch":
		return st.warn.Render(status)
	case "blocked":
		return st.bad.Render(status)
	default:
		return status
	}
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	out := ""
	for _, r := range s {
		next := out + string(r)
		if lipgloss.Width(next) > width {
			break
		}
		out = next
	}
	return out
}

func trimRightLines(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
