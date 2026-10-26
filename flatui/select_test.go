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

func TestSelectCustomButtonKeepsPresentationAcrossOpenAndClosed(t *testing.T) {
	var s Select
	s.SetOptions([]string{"One", "Two"})
	template := layout.Button{NodeBase: layout.NodeBase{ID: "choice", W: layout.Points(140), H: layout.Points(28), Tooltip: "Choose one"}, Focused: true, Plain: true, Default: true, Style: layout.TextStyle{Bold: true}}
	metrics := layout.Metrics{Column: 8, Line: 16, Step: 4, Border: 1}
	for _, open := range []bool{false, true} {
		if open {
			s.Open()
		}
		scene := layout.LayoutScene(layout.Col{Children: []layout.Node{s.LayoutWithButton(template)}}, 400, 200, metrics)
		found := false
		for _, raw := range scene.Ops {
			if op, ok := raw.(layout.ControlOp); ok && op.ID == "choice" {
				found = true
				if op.Rect.W != 140 || op.Rect.H != 28 || !op.Focused || !op.Plain || !op.Default || !op.Style.Bold || op.Label != "One ▾" {
					t.Fatalf("open=%v button %+v", open, op)
				}
			}
		}
		if !found || template.Label != "" {
			t.Fatal("trigger missing or template mutated")
		}
		if scene.TooltipAt(1, 1) != "Choose one" {
			t.Fatal("trigger tooltip lost")
		}
		if open {
			rect := scene.Rects["choice:option:1"]
			id, _ := scene.HitTest(rect.X+1, rect.Y+1)
			if !s.Click(id, template.ID) || s.Value() != "Two" || s.IsOpen() {
				t.Fatalf("custom trigger option target %q", id)
			}
		}
	}
}

func TestSelectCustomButtonCanSupplyItsOwnLabel(t *testing.T) {
	var s Select
	s.SetOptions([]string{"One"})
	node := s.LayoutWithButton(layout.Button{NodeBase: layout.NodeBase{ID: "choice"}, Label: "Sort songs", Icon: layout.IconChevronDown, Disabled: true}).(layout.Button)
	if node.Label != "Sort songs" || node.Icon != layout.IconChevronDown || !node.Disabled {
		t.Fatalf("custom button %+v", node)
	}
}

func TestSelectNativeOptionsHavePaddingAndFitMeasuredLabels(t *testing.T) {
	var s Select
	s.SetOptions([]string{"One", "WWW WWW"})
	s.Open()
	m := layout.Metrics{Column: 8, Line: 20, Step: 4, Border: 1, DefaultPadding: true, ControlHeight: 32,
		MeasureText: func(text string, _ layout.TextStyle, _ float64) (float64, float64) {
			return float64(len([]rune(text))) * 20, 20
		},
	}
	scene := layout.LayoutScene(layout.Col{Children: []layout.Node{s.Layout("choice", true)}}, 500, 350, m)
	for _, id := range []string{"choice:option:0", "choice:option:1"} {
		row := scene.Rects[id]
		if row.H != 28 {
			t.Fatalf("option lacks vertical padding: %s %v", id, row)
		}
		target, _ := scene.HitTest(row.X+1, row.Y+1)
		if target != id {
			t.Fatalf("padding outside option hit target %q want %q", target, id)
		}
	}
	menu := scene.Rects["choice:menu"]
	for _, raw := range scene.Ops {
		if op, ok := raw.(layout.TextOp); ok && op.Text == "WWW WWW" {
			row := scene.Rects["choice:option:1"]
			if op.Rect.Y-row.Y != 4 || op.Rect.X-row.X < 8 || op.Rect.W < 140 || op.Rect.X+op.Rect.W > menu.X+menu.W {
				t.Fatalf("padded option clipped or misaligned: text %v row %v menu %v", op.Rect, row, menu)
			}
			return
		}
	}
	t.Fatal("missing option label")
}

func TestSelectHasCompactNativeOuterInsetsAndInteractiveOptions(t *testing.T) {
	var selectMenu Select
	selectMenu.SetOptions([]string{"First", "Second"})
	selectMenu.Open()
	scene := layout.LayoutScene(layout.Col{Children: []layout.Node{selectMenu.Layout("sort", false)}}, 400, 250, layout.Metrics{Column: 8, Line: 20, Step: 4, Border: 1, DefaultPadding: true, ControlHeight: 32})
	menu, row := scene.Rects["sort:menu"], scene.Rects["sort:option:0"]
	if row.X-menu.X != 1 || row.Y-menu.Y != 1 {
		t.Fatalf("popup outerinset menu=%v row=%v", menu, row)
	}
	found := 0
	for _, raw := range scene.Ops {
		if op, ok := raw.(layout.BoxOp); ok && (op.ID == "sort:option:0" || op.ID == "sort:option:1") {
			if !op.Box.Interactive || !op.Box.Round {
				t.Fatalf("option has no default interactive presentation: %+v", op)
			}
			found++
		}
	}
	if found != 2 {
		t.Fatalf("interactive option boxes %d", found)
	}
	if selectMenu.Selected() != 0 || selectMenu.Highlighted() != 0 {
		t.Fatal("render changed selection")
	}
}
