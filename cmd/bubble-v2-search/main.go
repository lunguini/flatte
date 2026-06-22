package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lunguini/flatte/flatui"
)

const (
	defaultSearchDelay = 300 * time.Millisecond
	renderWidth        = 72
)

var corpus = []string{
	"haiku",
	"sonnet",
	"opus",
	"freeform",
	"villanelle",
	"limerick",
	"ghazal",
}

// searchResultMsg carries one finished search back to Update. The generation
// stamp lets Update drop results from searches that were superseded while the
// goroutine slept; Bubble Tea cannot cancel an in-flight Cmd.
type searchResultMsg struct {
	generation int
	results    []string
	err        error
}

type Model struct {
	query      flatui.TextField
	focused    bool
	searching  bool
	generation int
	results    []string
	err        error
}

func NewModel() Model {
	return Model{focused: true}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	case searchResultMsg:
		if msg.generation != m.generation {
			return m, nil // stale: a newer search superseded this one
		}
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.results = msg.results
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.Key()

	if !m.focused {
		if key.Text == "q" || key.Text == "Q" {
			return m, tea.Quit
		}
		if key.Code == tea.KeyEnter {
			m.focused = true
			m.query.SetCursor(len(m.query.Value))
		}
		return m, nil
	}

	switch {
	case len(key.Text) > 0:
		for _, r := range key.Text {
			m.query.Insert(r)
		}
		return m.startSearch()
	case key.Code == tea.KeyBackspace:
		m.query.Backspace()
		return m.startSearch()
	case key.Code == tea.KeyDelete:
		m.query.Delete()
		return m.startSearch()
	case key.Code == tea.KeyLeft:
		m.query.MoveLeft()
	case key.Code == tea.KeyRight:
		m.query.MoveRight()
	case key.Code == tea.KeyEnter:
		m.focused = false
	}
	return m, nil
}

func (m Model) startSearch() (tea.Model, tea.Cmd) {
	m.err = nil
	m.generation++ // supersede any in-flight search

	query := m.query.Value
	if strings.TrimSpace(query) == "" {
		m.searching = false
		m.results = nil
		return m, nil
	}

	m.searching = true
	return m, searchCmd(m.generation, query)
}

func searchCmd(generation int, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := search(query)
		return searchResultMsg{generation: generation, results: results, err: err}
	}
}

func search(query string) ([]string, error) {
	time.Sleep(searchDelay())

	query = strings.ToLower(query)
	var results []string
	for _, item := range corpus {
		if strings.Contains(strings.ToLower(item), query) {
			results = append(results, item)
		}
	}
	return results, nil
}

func (m Model) View() tea.View {
	status := "idle"
	if m.searching {
		status = "searching..."
	}
	if m.err != nil {
		status = m.err.Error()
	}

	rows := []string{
		flatui.Title("Bubble v2 Search"),
		flatui.Subtle("input-triggered async sample"),
		"",
		"  query: " + fakeCursor(m.query, m.focused),
		"  state: " + status,
		"",
	}
	if len(m.results) == 0 {
		rows = append(rows, flatui.Subtle("  no results"))
	} else {
		for _, result := range m.results {
			rows = append(rows, "  - "+result)
		}
	}
	rows = append(rows, "", flatui.Subtle("enter blur/focus | q quits when blurred"))

	view := tea.NewView(flatui.Card(rows, renderWidth))
	view.AltScreen = true
	view.WindowTitle = "Flatte Bubble Tea v2 comparison"
	return view
}

func searchDelay() time.Duration {
	value := os.Getenv("FLAT_SEARCH_DELAY")
	if value == "" {
		return defaultSearchDelay
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return defaultSearchDelay
	}
	return delay
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
