package main

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type confirmModel struct {
	action     string
	targetID   string
	targetName string
}

type commandModel struct {
	input   flatui.TextField
	histIdx int // -1 = editing fresh; >=0 = browsing history
}

func newCommand() *commandModel {
	return &commandModel{histIdx: -1}
}

func (m *commandModel) Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		s.commandModal = nil
	case flatte.KeyEnter:
		line := strings.TrimSpace(m.input.Value)
		if line != "" {
			s.cmdHistory = append(s.cmdHistory, line)
		}
		m.execute(s, fx, line)
		s.commandModal = nil
	case flatte.KeyBackspace:
		m.input.Backspace()
	case flatte.KeyDelete:
		m.input.Delete()
	case flatte.KeyLeft:
		m.input.MoveLeft()
	case flatte.KeyRight:
		m.input.MoveRight()
	case flatte.KeyUp:
		m.browseHistory(s, -1)
	case flatte.KeyDown:
		m.browseHistory(s, 1)
	case flatte.KeyCharacter:
		m.input.Insert(key.Rune)
	}
}

func (m *commandModel) browseHistory(s *State, dir int) {
	if len(s.cmdHistory) == 0 {
		return
	}
	if dir < 0 {
		if m.histIdx == -1 {
			m.histIdx = len(s.cmdHistory) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
	} else {
		if m.histIdx == -1 {
			return
		}
		m.histIdx++
		if m.histIdx >= len(s.cmdHistory) {
			m.histIdx = -1
			m.input.Value = ""
			m.input.SetCursor(0)
			return
		}
	}
	m.input.Value = s.cmdHistory[m.histIdx]
	m.input.SetCursor(len(m.input.Value))
}

func (m *commandModel) execute(s *State, _ flatte.Effects[State], line string) {
	if line == "" {
		s.containers.pushActivity("cmd  (empty)")
		return
	}
	s.containers.pushActivity("cmd  " + line)
	parts := strings.Fields(line)
	cmd, args := parts[0], parts[1:]
	switch cmd {
	case "filter":
		if len(args) > 0 {
			s.containers.filter.Value = strings.Join(args, " ")
			s.containers.filter.SetCursor(len(s.containers.filter.Value))
			s.containers.refreshFilter()
			s.containers.onSelectionChange(s, flatte.Effects[State]{})
			s.containers.pushActivity("→   filter applied: " + s.containers.filter.Value)
		} else {
			s.containers.filter.Value = ""
			s.containers.refreshFilter()
			s.containers.pushActivity("→   filter cleared")
		}
	case "stop":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		ct.Status = "exited"
		s.containers.recomputeDetailWidgets()
		s.containers.pushActivity("→   stopped " + ct.Name)
	case "remove":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		ct.Status = "removed"
		s.containers.recomputeDetailWidgets()
		s.containers.pushActivity("→   removed " + ct.Name)
	case "goto":
		if len(args) == 0 {
			s.containers.pushActivity("→   usage: goto <containers|images|volumes>")
			return
		}
		switch args[0] {
		case "containers", "1":
			s.screen = screenContainers
		case "images", "2":
			s.screen = screenImages
		case "volumes", "3":
			s.screen = screenVolumes
		default:
			s.containers.pushActivity("→   unknown screen: " + args[0])
			return
		}
		s.headerTabs.SetActive(int(s.screen))
	case "help":
		s.containers.pushActivity("→   filter <text> | stop | remove | goto <screen> | stream | help")
	case "stream":
		ct := s.containers.selected()
		if ct == nil {
			s.containers.pushActivity("→   no container selected")
			return
		}
		s.containers.injectStreamDemo(ct.ID)
		s.containers.pushActivity("→   streamed 8 demo log lines into " + ct.Name)
	case "q", "quit", "exit":
		s.containers.pushActivity("→   press q (lowercase, no colon) to quit")
	default:
		s.containers.pushActivity("→   unknown command: " + cmd + " (try :help)")
	}
}

func (m *commandModel) View(width int) string {
	prompt := lipgloss.NewStyle().Bold(true).Foreground(pal.accent).Render(":")
	input := m.input.Value
	if input == "" {
		placeholder := lipgloss.NewStyle().Foreground(pal.muted).Render("type a command (try :help)")
		return lipgloss.NewStyle().
			Width(width).
			Background(pal.bg).
			Foreground(pal.text).
			Render(prompt + " " + placeholder)
	}
	return lipgloss.NewStyle().
		Width(width).
		Background(pal.bg).
		Foreground(pal.text).
		Render(prompt + " " + input)
}

func (m *commandModel) cursorFrameX() int {
	// ": " prompt = 2 cells, plus the input's cursor column
	return 2 + m.input.CursorColumn()
}

func (m *commandModel) keyHints() string {
	return "enter execute  ↑↓ history  esc cancel"
}

func (m *confirmModel) Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	switch key.Key {
	case flatte.KeyEscape:
		s.confirmModal = nil
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'y', 'Y':
			s.applyModalAction()
			s.confirmModal = nil
		case 'n', 'N':
			s.confirmModal = nil
		}
	}
}

func (s *State) applyModalAction() {
	if s.confirmModal == nil {
		return
	}
	for i := range s.containers.containers {
		if s.containers.containers[i].ID == s.confirmModal.targetID {
			switch s.confirmModal.action {
			case "stop":
				s.containers.containers[i].Status = "exited"
			case "remove":
				s.containers.containers[i].Status = "removed"
			}
			break
		}
	}
	s.containers.recomputeDetailWidgets()
}

func (s *State) openConfirm(action string, ct *Container) {
	if ct == nil {
		return
	}
	s.confirmModal = &confirmModel{
		action:     action,
		targetID:   ct.ID,
		targetName: ct.Name,
	}
}

// modalWidth is the confirm modal's fixed total width (including its border).
const modalWidth = 40

// modalNode builds the confirm dialog as a centered overlay subtree. The engine
// finds it via Overlay:true, clears its rect, and composes it on top of the base
// frame in the single frame walk — no post-hoc string compositing. Bordered
// declares the 1-cell inset for the styled frame; Chrome paints that frame (a
// rounded red border with a title bar) under the body Text children.
func modalNode(m *confirmModel) layout.Node {
	body := lipgloss.NewStyle().Foreground(pal.text).
		Render("  " + m.action + " " + m.targetName + "?")
	yKey := lipgloss.NewStyle().Bold(true).Foreground(pal.good).Render("y")
	nKey := lipgloss.NewStyle().Bold(true).Foreground(pal.bad).Render("n/esc")
	hints := lipgloss.NewStyle().Foreground(pal.muted).Render("  ") +
		yKey + lipgloss.NewStyle().Foreground(pal.muted).Render(" confirm    ") +
		nKey + lipgloss.NewStyle().Foreground(pal.muted).Render(" cancel")

	blank := func() layout.Node {
		return layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}}
	}
	return layout.Col{
		NodeBase: layout.NodeBase{
			ID:       "modal",
			Overlay:  true,
			Bordered: true,
			W:        layout.Fixed(modalWidth),
			Chrome:   modalChrome(m.action),
		},
		Children: []layout.Node{
			blank(),
			layout.Text{NodeBase: layout.NodeBase{ID: "modal:body", H: layout.Fixed(1)}, String: body},
			blank(),
			layout.Text{NodeBase: layout.NodeBase{ID: "modal:hints", H: layout.Fixed(1)}, String: hints},
			blank(),
		},
	}
}

// modalChrome paints the modal's rounded border in the alert color with the
// action embedded as a title in the top border rule. It is painted at the
// node's full rect under the body children; the side/corner glyphs it draws sit
// in the Bordered inset the children never reach, so they survive.
func modalChrome(action string) func(layout.Rect) string {
	return func(r layout.Rect) string {
		if r.W < 4 || r.H < 2 {
			return ""
		}
		border := lipgloss.NewStyle().Foreground(pal.bad)
		title := lipgloss.NewStyle().Bold(true).Foreground(pal.bad).
			Render(" " + action + " container ")
		trail := r.W - 2 - lipgloss.Width(title) - 1 // after "╭─" + title, before "╮"
		if trail < 0 {
			trail = 0
		}
		top := border.Render("╭─") + title + border.Render(strings.Repeat("─", trail)+"╮")
		side := border.Render("│") + strings.Repeat(" ", r.W-2) + border.Render("│")
		bottom := border.Render("╰" + strings.Repeat("─", r.W-2) + "╯")
		rows := make([]string, r.H)
		rows[0] = top
		for i := 1; i < r.H-1; i++ {
			rows[i] = side
		}
		rows[r.H-1] = bottom
		return strings.Join(rows, "\n")
	}
}
