package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lunguini/flat/internal/flatui"
)

const (
	defaultTickInterval = 300 * time.Millisecond
	renderWidth         = 72
)

var spinnerFrames = []string{"-", "\\", "|", "/"}

type tickMsg struct{}

type Model struct {
	ticks           int
	spinner         int
	waiting         bool
	modalOpen       bool
	modalInput      flatui.TextField
	modalResult     string
	clipboardStatus string
	quit            bool
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
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	case tea.ClipboardMsg:
		m.clipboardStatus = "clipboard: " + msg.Content
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.modalOpen {
		return m.updateModalKey(msg), nil
	}

	key := msg.Key()
	switch {
	case key.Code == tea.KeyEnter:
		m.waiting = true
		m.modalOpen = true
		m.modalInput = flatui.TextField{}
	case key.Text == "q" || key.Text == "Q":
		m.quit = true
		return m, tea.Quit
	case key.Text == "c":
		text := m.modalResult
		if text == "" {
			text = "none"
		}
		m.clipboardStatus = "copying: " + text
		return m, tea.SetClipboard(text)
	case key.Text == "p":
		m.clipboardStatus = "reading clipboard"
		return m, tea.ReadClipboard
	}
	return m, nil
}

func (m Model) updateModalKey(msg tea.KeyPressMsg) Model {
	key := msg.Key()
	switch {
	case len(key.Text) > 0:
		for _, r := range key.Text {
			m.modalInput.Insert(r)
		}
	case key.Code == tea.KeyBackspace:
		m.modalInput.Backspace()
	case key.Code == tea.KeyDelete:
		m.modalInput.Delete()
	case key.Code == tea.KeyLeft:
		m.modalInput.MoveLeft()
	case key.Code == tea.KeyRight:
		m.modalInput.MoveRight()
	case key.Code == tea.KeyEnter:
		m.modalOpen = false
		m.waiting = false
		m.modalResult = "accepted: " + m.modalInput.Value
	case key.Code == tea.KeyEscape:
		m.modalOpen = false
		m.waiting = false
		m.modalResult = "cancelled"
	}
	return m
}

func (m Model) View() tea.View {
	base := m.viewMain()
	if m.modalOpen {
		base = flatui.Overlay(base, m.viewModal())
	}

	view := tea.NewView(base)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "Flatte Bubble Tea v2 comparison"
	return view
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

	clipboard := "idle"
	if m.clipboardStatus != "" {
		clipboard = m.clipboardStatus
	}

	lines := []string{
		flatui.Title("Bubble v2 Modal"),
		fmt.Sprintf("  background ticks: %d | loader: %s", m.ticks, loader),
		flatui.Subtle("TEA v2 view metadata, mouse mode, clipboard Cmds"),
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
		"  clipboard: " + clipboard,
		"  recent messages:",
		"    - tickMsg handled",
		"    - modal owns keyboard focus",
		"",
		flatui.Subtle("enter open modal | c copy result | p read clipboard | q quit"),
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
	value := os.Getenv("BUBBLE_V2_MODAL_INTERVAL")
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
