package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunguini/flat/internal/flatui"
)

const (
	defaultTickInterval = 300 * time.Millisecond
	renderWidth         = 72
)

var spinnerFrames = []string{"-", "\\", "|", "/"}

type tickMsg struct{}

type Model struct {
	ticks       int
	spinner     int
	waiting     bool
	modalOpen   bool
	modalInput  flatui.TextField
	modalResult string
	quit        bool
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval(), func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.ticks++
		if m.waiting {
			m.spinner = (m.spinner + 1) % len(spinnerFrames)
		}
		return m, tickCmd()
	case tea.KeyMsg:
		return m.updateKey(msg)
	default:
		return m, nil
	}
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modalOpen {
		return m.updateModalKey(msg), nil
	}

	switch msg.Type {
	case tea.KeyEnter:
		m.waiting = true
		m.modalOpen = true
		m.modalInput = flatui.TextField{}
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'q' || msg.Runes[0] == 'Q') {
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateModalKey(msg tea.KeyMsg) Model {
	switch msg.Type {
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.modalInput.Insert(r)
		}
	case tea.KeyBackspace:
		m.modalInput.Backspace()
	case tea.KeyDelete:
		m.modalInput.Delete()
	case tea.KeyLeft:
		m.modalInput.MoveLeft()
	case tea.KeyRight:
		m.modalInput.MoveRight()
	case tea.KeyEnter:
		m.modalOpen = false
		m.waiting = false
		m.modalResult = "accepted: " + m.modalInput.Value
	case tea.KeyEsc:
		m.modalOpen = false
		m.waiting = false
		m.modalResult = "cancelled"
	}
	return m
}

func (m Model) View() string {
	base := m.viewMain()
	if !m.modalOpen {
		return base
	}
	return flatui.Overlay(base, m.viewModal())
}

func (m Model) viewMain() string {
	loader := "idle"
	if m.waiting {
		loader = "waiting " + spinnerFrames[m.spinner%len(spinnerFrames)]
	}

	result := "none"
	if m.modalResult != "" {
		result = m.modalResult
	}

	lines := []string{
		flatui.Title("Bubble Modal"),
		fmt.Sprintf("  background ticks: %d | loader: %s", m.ticks, loader),
		flatui.Subtle("TEA modal focus with background Cmds"),
		"  background workspace:",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"  modal result: " + result,
		"  recent messages:",
		"    - tickMsg handled",
		"    - modal owns keyboard focus",
		"",
		flatui.Subtle("enter open modal | q quit"),
	}
	if m.modalOpen {
		lines = append(lines, flatui.Subtle("main view is waiting for modal input"))
	}
	return flatui.Card(lines, renderWidth)
}

func (m Model) viewModal() string {
	lines := []string{
		flatui.Title("Confirm Work"),
		flatui.Subtle("modal captures input"),
		"",
		"  name: " + fakeCursor(m.modalInput, true),
		"",
		flatui.Subtle("enter confirm | esc cancel"),
	}
	return flatui.Card(lines, 32)
}

func tickInterval() time.Duration {
	value := os.Getenv("BUBBLE_MODAL_INTERVAL")
	if value == "" {
		return defaultTickInterval
	}
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		return defaultTickInterval
	}
	return interval
}

func main() {
	if _, err := tea.NewProgram(NewModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// fakeCursor paints the historical ▌ marker. The Flatte counterparts
// moved to the real hardware cursor (Frame.Cursor) in Phase 4; the
// benchmarks keep their original rendering. (BT v2 has an equivalent
// real-cursor facility, View.Cursor - adopt it only if a comparison
// claim requires parity.)
func fakeCursor(f flatui.TextField, focused bool) string {
	if focused {
		return f.Value[:f.Cursor] + "▌" + f.Value[f.Cursor:]
	}
	return f.Value
}
