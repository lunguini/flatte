package layout

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/theme"
)

func withCellTheme(t *testing.T, p *theme.Palette) {
	t.Helper()
	UseCellTheme(p)
	t.Cleanup(func() { UseCellTheme(nil) })
}

func TestThemedCellsResolveRoles(t *testing.T) {
	d := theme.Default()
	withCellTheme(t, &d.Dark)
	// A caption: upper case, bold, in TextMuted (#999288 = 153;146;136).
	out := TextStyle{Role: TextCaption}.Render("files")
	if !strings.Contains(out, "FILES") || !strings.Contains(out, "38;2;153;146;136") || !strings.Contains(out, "1") {
		t.Fatalf("caption: %q", out)
	}
	// A color role wins over the text role's color; Fg wins over both.
	if out := (TextStyle{Role: TextMeta, Color: ColorWarning}).Render("x"); !strings.Contains(out, "38;2;227;168;59") {
		t.Fatalf("warning meta: %q", out)
	}
	// A fill paints its background in cells.
	content, _ := SolveAndCompose(Row{NodeBase: NodeBase{W: Fixed(4), H: Fixed(1), Box: Box{Fill: FillChrome}}, Children: []Node{Label{Text: "ab"}}}, 4, 1)
	if !strings.Contains(content, "48;2;28;27;24") { // Chrome #1c1b18
		t.Fatalf("chrome fill: %q", content)
	}
	// A translucent fill composes over the content surface.
	if b := cellBox(Box{Fill: FillPressed}); b.Background == nil {
		t.Fatal("pressed fill has no background")
	} else if _, _, _, a := b.Background.RGBA(); a != 0xffff {
		t.Fatalf("pressed fill is not opaque: alpha %x", a)
	}
}

// A cell theme is for terminal cells only: a GUI scene of selected and
// filled boxes lays out and paints the same with or without one (a
// selected box given the cell background would gain panel padding).
func TestCellThemeLeavesGUIScenesAlone(t *testing.T) {
	root := Col{NodeBase: NodeBase{W: Fixed(30)}, Children: []Node{
		Row{NodeBase: NodeBase{ID: "sel", Box: Box{Selected: true}}, Children: []Node{Label{Text: "selected"}}},
		Row{NodeBase: NodeBase{ID: "fill", Box: Box{Fill: FillChrome}}, Children: []Node{Label{Text: "chrome"}}},
	}}
	m := Metrics{Column: 8, Line: 20, Step: 4, Border: 1, DefaultPadding: true, ControlHeight: 32}
	UseCellTheme(nil)
	want := LayoutScene(root, 240, 200, m)
	d := theme.Default()
	withCellTheme(t, &d.Dark)
	got := LayoutScene(root, 240, 200, m)
	for _, id := range []string{"sel", "fill"} {
		if got.Rects[id] != want.Rects[id] {
			t.Errorf("%s laid out at %v with a cell theme, %v without", id, got.Rects[id], want.Rects[id])
		}
	}
	for i, op := range got.Ops {
		if b, ok := op.(BoxOp); ok && b.Box.Background != nil && i < len(want.Ops) {
			if w, ok := want.Ops[i].(BoxOp); !ok || w.Box.Background == nil {
				t.Errorf("op %d: a GUI box painted the cell background %v", i, b.Box.Background)
			}
		}
	}
}

func TestUnthemedCellsKeepTheTerminalsColors(t *testing.T) {
	UseCellTheme(nil)
	if out := (TextStyle{Role: TextSubtle}).Render("x"); !strings.Contains(out, "38;5;244") {
		t.Fatalf("subtle without a theme: %q", out)
	}
	content, _ := SolveAndCompose(Row{NodeBase: NodeBase{W: Fixed(4), H: Fixed(1), Box: Box{Fill: FillChrome}}, Children: []Node{Label{Text: "ab"}}}, 4, 1)
	if strings.Contains(content, "48;2") {
		t.Fatalf("a fill painted without a theme: %q", content)
	}
}

func TestFillColorCoversEveryRole(t *testing.T) {
	d := theme.Default()
	for f := FillSurface; f <= FillDiffBlank; f++ {
		if _, ok := FillColor(&d.Light, f); !ok {
			t.Errorf("fill %d has no color", f)
		}
	}
	if _, ok := FillColor(&d.Light, FillNone); ok {
		t.Error("FillNone has a color")
	}
}

// Without a theme, color roles draw in the terminal's own palette, so a
// warning or a match keeps its color in an unthemed terminal.
func TestUnthemedCellsKeepColorRoles(t *testing.T) {
	for c, want := range map[TextColor]string{ColorWarning: "33", ColorDanger: "31", ColorSuccess: "32", ColorInfo: "34", ColorAccent: "34", ColorSecondary: "38;5;244"} {
		if out := (TextStyle{Color: c}).Render("x"); !strings.Contains(out, "\x1b["+want+"m") && !strings.Contains(out, ";"+want+"m") {
			t.Errorf("role %d: %q, want SGR %s", c, out, want)
		}
	}
	if out := (TextStyle{Color: ColorPrimary}).Render("x"); out != "x" {
		t.Errorf("primary text is styled: %q", out)
	}
}
