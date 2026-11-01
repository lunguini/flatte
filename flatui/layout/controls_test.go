package layout

import (
	"math"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func cellsOfNode(n Node, w int) string {
	return strings.TrimRight(ansi.Strip(Compose(Col{Children: []Node{Row{Children: []Node{n}}}}, w, 1).Frame), " ")
}

func TestControlSpellings(t *testing.T) {
	for _, tc := range []struct {
		node Node
		want string
	}{
		{Button{Label: "Save"}, "[ Save ]"},
		{Toggle{Label: "Wi-Fi", Checked: true}, "[x] Wi-Fi"},
		{Toggle{Label: "Wi-Fi"}, "[ ] Wi-Fi"},
		{Toggle{Kind: ToggleSwitch, Label: "Sync", Checked: true}, "[ ●] Sync"},
		{Toggle{Kind: ToggleSwitch, Label: "Sync"}, "[○ ] Sync"},
		{Toggle{Kind: ToggleRadio, Label: "Dark", Checked: true}, "(•) Dark"},
		{Toggle{Kind: ToggleRadio}, "( )"},
	} {
		if got := cellsOfNode(tc.node, 20); got != tc.want {
			t.Errorf("%T %+v = %q, want %q", tc.node, tc.node, got, tc.want)
		}
		w, h := tc.node.Size()
		if w != (Size{Kind: SizeContent, Value: ansi.StringWidth(tc.want)}) || h != (Size{Kind: SizeContent, Value: 1}) {
			t.Errorf("%+v size %+v × %+v, want its spelling", tc.node, w, h)
		}
	}
}

func TestControlTerminalLooks(t *testing.T) {
	render := func(n Node) string { return Compose(Col{Children: []Node{Row{Children: []Node{n}}}}, 20, 1).Frame }
	if !strings.Contains(render(Button{Label: "OK", Default: true}), "\x1b[1m") {
		t.Error("default button not bold")
	}
	if !strings.Contains(render(Button{Label: "OK", Focused: true}), "\x1b[7m") {
		t.Error("focused button not reversed")
	}
	if strings.Contains(render(Button{Label: "OK", Focused: true, Disabled: true}), "\x1b[7m") {
		t.Error("disabled button shows focus")
	}
	if !strings.Contains(render(Toggle{Label: "x", Focused: true}), "\x1b[7m") {
		t.Error("focused toggle indicator not reversed")
	}
	if !strings.Contains(render(Button{Label: "OK", Disabled: true, Style: TextStyle{Fg: lipgloss.Color("#ff0000")}}), "244") {
		t.Error("disabled button not subtle")
	}
}

func TestSliderTrack(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		want string
	}{
		{0, "●─────────"}, {1, "━━━━━━━━━●"}, {0.5, "━━━━━●────"}, {math.NaN(), "●─────────"}, {-3, "●─────────"},
	} {
		if got := cellsOfNode(Slider{NodeBase: NodeBase{W: Fixed(10)}, Value: tc.v}, 10); got != tc.want {
			t.Errorf("slider %v = %q, want %q", tc.v, got, tc.want)
		}
	}
	if w, h := (Slider{}).Size(); w.Kind != SizeGrow || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Errorf("slider size %+v × %+v, want grow × 1", w, h)
	}
	s := Slider{}
	for _, tc := range []struct {
		x, w int
		want float64
	}{{0, 10, 0}, {9, 10, 1}, {3, 7, 0.5}, {-2, 10, 0}, {40, 10, 1}, {0, 1, 0}} {
		if got := s.ValueAt(tc.x, tc.w); got != tc.want {
			t.Errorf("ValueAt(%d, %d) = %v, want %v", tc.x, tc.w, got, tc.want)
		}
	}
}

func TestSceneControlsAndTooltips(t *testing.T) {
	root := Col{NodeBase: NodeBase{Tooltip: "panel"}, Children: []Node{
		Button{NodeBase: NodeBase{ID: "save", Tooltip: "Save changes"}, Label: "Save", Default: true},
		Toggle{NodeBase: NodeBase{ID: "sync"}, Kind: ToggleSwitch, Label: "Sync", Checked: true},
		Slider{NodeBase: NodeBase{ID: "vol"}, Value: 0.25, Focused: true},
		Col{NodeBase: NodeBase{Overlay: true, W: Fixed(4), H: Fixed(1)}, Children: []Node{Label{Text: "pop"}}},
	}}
	s := LayoutScene(root, 40*testMetrics.Column, 9*testMetrics.Line, testMetrics)
	var ops []ControlOp
	for _, op := range s.Ops {
		if c, ok := op.(ControlOp); ok {
			ops = append(ops, c)
		}
	}
	if len(ops) != 3 {
		t.Fatalf("%d control ops, want 3", len(ops))
	}
	if b := ops[0]; b.ID != "save" || b.Kind != ControlButton || b.Label != "Save" || !b.Default || b.Rect != s.Rects["save"] {
		t.Errorf("button op %+v", b)
	}
	if sw := ops[1]; sw.Kind != ControlSwitch || !sw.Checked || sw.Label != "Sync" {
		t.Errorf("switch op %+v", sw)
	}
	if sl := ops[2]; sl.Kind != ControlSlider || sl.Value != 0.25 || !sl.Focused {
		t.Errorf("slider op %+v", sl)
	}
	save, sync := s.Rects["save"], s.Rects["sync"]
	if got := s.TooltipAt(save.X+1, save.Y+1); got != "Save changes" {
		t.Errorf("tooltip over the button = %q", got)
	}
	if got := s.TooltipAt(sync.X+1, sync.Y+1); got != "panel" {
		t.Errorf("tooltip over the switch = %q, want its container's", got)
	}
	if got := s.TooltipAt(1e4, 1e4); got != "" {
		t.Errorf("tooltip outside = %q", got)
	}
	// The overlay is centered over the panel and has no tooltip: it hides the panel's.
	var layer LayerOp
	for _, op := range s.Ops {
		if l, ok := op.(LayerOp); ok {
			layer = l
		}
	}
	if got := s.TooltipAt(layer.Rect.X+1, layer.Rect.Y+1); got != "" {
		t.Errorf("tooltip under an overlay = %q", got)
	}
}

func TestIconAndPlainButtonSpellings(t *testing.T) {
	for _, c := range []struct {
		b    Button
		want string
	}{
		{Button{Label: "Save"}, "[ Save ]"},
		{Button{Label: "▶", Icon: IconPlay, Default: true}, "[▶]"},
		{Button{Label: "⏭", Icon: IconNext, Plain: true}, " ⏭ "},
		{Button{Label: "Back", Plain: true}, " Back "},
	} {
		if got := c.b.spelling(); got != c.want {
			t.Errorf("%+v spells %q, want %q", c.b, got, c.want)
		}
		w, _ := c.b.Size()
		if got := ansi.StringWidth(ansi.Strip(c.b.Render(Rect{W: w.Value, H: 1}))); got != w.Value {
			t.Errorf("%q renders %d wide, sized %d", c.want, got, w.Value)
		}
	}
}
