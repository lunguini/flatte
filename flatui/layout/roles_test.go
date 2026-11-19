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
