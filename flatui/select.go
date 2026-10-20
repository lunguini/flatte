package flatui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// Select is a dropdown: a choice among options, shown as a button, whose
// list opens under it. The app owns it (like List): it holds the options,
// the selected one, whether the list is open, and the highlighted option
// while it is; the app decides which keys open, move, choose, and close.
type Select struct {
	options  []string
	selected int
	open     bool
	list     List
}

// SetOptions sets the choices, keeping the selection in range.
func (s *Select) SetOptions(options []string) {
	s.options = options
	s.list.SetCount(len(options))
	s.list.SetHeight(len(options))
	s.selected = min(max(s.selected, 0), max(len(options)-1, 0))
}

// Options are the choices.
func (s Select) Options() []string { return s.options }

// Selected is the selected option's index; Value its text ("" with none).
func (s Select) Selected() int { return s.selected }

func (s Select) Value() string {
	if s.selected < len(s.options) {
		return s.options[s.selected]
	}
	return ""
}

// Select chooses option i (clamped) and closes the list.
func (s *Select) Select(i int) {
	s.selected = min(max(i, 0), max(len(s.options)-1, 0))
	s.open = false
}

// IsOpen reports whether the list is open; Highlighted is the option
// highlighted in it.
func (s Select) IsOpen() bool     { return s.open }
func (s Select) Highlighted() int { return s.list.Cursor() }

// Open shows the list with the selected option highlighted; Close hides it
// without choosing; Toggle does whichever applies.
func (s *Select) Open() {
	s.open = len(s.options) > 0
	s.list.Select(s.selected)
}

func (s *Select) Close() { s.open = false }

func (s *Select) Toggle() {
	if s.open {
		s.Close()
		return
	}
	s.Open()
}

// MoveUp and MoveDown move the highlight while the list is open.
func (s *Select) MoveUp()   { s.list.MoveUp() }
func (s *Select) MoveDown() { s.list.MoveDown() }

// Confirm chooses the highlighted option and closes the list.
func (s *Select) Confirm() {
	if s.open {
		s.Select(s.list.Cursor())
	}
}

// Click handles a click whose target is the select laid out with id: on the
// button it toggles the list, on an option it chooses it. It reports
// whether the target was the select's.
func (s *Select) Click(target, id string) bool {
	if target == id {
		s.Toggle()
		return true
	}
	if n, ok := strings.CutPrefix(target, optionPrefix(id)); ok {
		if i, err := strconv.Atoi(n); err == nil {
			s.Select(i)
			return true
		}
	}
	return false
}

func optionPrefix(id string) string { return id + ":option:" }

// Layout returns the select as a Button with ID id showing the value and a
// disclosure mark ("[ English ▾ ]" in the terminal), and — while open — its
// list as a bordered overlay anchored under the button, whose rows have IDs
// "id:option:i" (the highlighted one marked "> " and Selected, the chosen
// one bold). Click maps clicks on either.
func (s Select) Layout(id string, focused bool) layout.Node {
	button := layout.Button{NodeBase: layout.NodeBase{ID: id}, Label: s.Value() + " ▾", Focused: focused}
	if !s.open {
		return button
	}
	width := 0
	for _, o := range s.options {
		width = max(width, ansi.StringWidth(o))
	}
	rows := s.list.Rows(strings.TrimSuffix(optionPrefix(id), ":"), func(i int, highlighted bool) layout.Node {
		marker := layout.Icon{Text: "  "}
		if highlighted {
			marker = layout.Icon{Name: layout.IconChevronRight, Text: "> "}
		}
		return layout.Row{Children: []layout.Node{marker, layout.Label{Text: s.options[i], Style: layout.TextStyle{Bold: i == s.selected}}}}
	})
	menu := layout.Col{
		NodeBase: layout.NodeBase{ID: id + ":menu", Overlay: true, Anchor: id, Bordered: true, W: layout.Fixed(width + 4)},
		Children: []layout.Node{rows},
	}
	return layout.Row{Children: []layout.Node{button, menu}}
}
