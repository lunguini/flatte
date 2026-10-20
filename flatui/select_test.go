package flatui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestSelectState(t *testing.T) {
	var s Select
	s.SetOptions([]string{"English", "Français", "Deutsch"})
	if s.Value() != "English" || s.IsOpen() {
		t.Fatalf("initial %q open %v", s.Value(), s.IsOpen())
	}
	s.Open()
	s.MoveDown()
	s.MoveDown()
	if s.Highlighted() != 2 || s.Selected() != 0 {
		t.Fatalf("highlight %d selected %d", s.Highlighted(), s.Selected())
	}
	s.Confirm()
	if s.Value() != "Deutsch" || s.IsOpen() {
		t.Fatalf("after confirm %q open %v", s.Value(), s.IsOpen())
	}
	s.Open() // highlights the selection again
	if s.Highlighted() != 2 {
		t.Fatalf("reopened highlight %d", s.Highlighted())
	}
	s.MoveUp()
	s.Close()
	if s.Value() != "Deutsch" {
		t.Fatalf("close chose %q", s.Value())
	}
	s.SetOptions([]string{"one"})
	if s.Selected() != 0 {
		t.Fatalf("selection out of range after SetOptions: %d", s.Selected())
	}
	var empty Select
	empty.Open()
	if empty.IsOpen() || empty.Value() != "" {
		t.Fatal("an empty select opened")
	}
}

func TestSelectLayoutAndClicks(t *testing.T) {
	var s Select
	s.SetOptions([]string{"Light", "Dark"})
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{s.Layout("theme", true), layout.NewSpacer()}}
	}
	comp := layout.Compose(root(), 20, 6)
	if first := strings.Split(ansi.Strip(comp.Frame), "\n")[0]; strings.TrimRight(first, " ") != "[ Light ▾ ]" {
		t.Fatalf("closed select %q", first)
	}
	if !s.Click("theme", "theme") || !s.IsOpen() {
		t.Fatal("clicking the button did not open the list")
	}
	comp = layout.Compose(root(), 20, 6)
	lines := strings.Split(ansi.Strip(comp.Frame), "\n")
	if !strings.Contains(lines[1], "╭") || !strings.Contains(lines[2], "> Light") || !strings.Contains(lines[3], "  Dark") {
		t.Fatalf("open select:\n%s", strings.Join(lines, "\n"))
	}
	id, _ := comp.HitTest(3, 3)
	if !s.Click(id, "theme") || s.Value() != "Dark" || s.IsOpen() {
		t.Fatalf("clicking %q: value %q open %v", id, s.Value(), s.IsOpen())
	}
	if s.Click("other", "theme") {
		t.Fatal("a foreign target claimed")
	}
}
