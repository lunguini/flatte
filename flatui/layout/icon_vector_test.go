package layout

import (
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestVectorIconGeometryCoversEveryLargeShape(t *testing.T) {
	for name := IconCheck; name <= IconRepeatOne; name++ {
		if name >= IconChevronLeft && name <= IconChevronDown {
			continue // first-column shapes, as IconChevronRight
		}
		if _, ok := vectorIconGeometry(name); !ok {
			t.Errorf("icon %d has no terminal shape", name)
		}
	}
	for _, name := range []IconName{IconNone, IconBlank, IconChevronRight, IconChevronLeft, IconDot} {
		if _, ok := vectorIconGeometry(name); ok {
			t.Errorf("icon %d drawn as a shape; want its text", name)
		}
	}
}

func TestVectorIconIsAntialiasedShapeInItsColor(t *testing.T) {
	red := color.RGBA{R: 0xff, A: 0xff}
	play := vectorIcon{Name: IconPlay, Color: red}
	if got := play.At(vectorIconSide/2, vectorIconSide/2).(color.NRGBA); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Fatalf("center = %v, want opaque red", got)
	}
	if got := play.At(2, 2).(color.NRGBA); got.A != 0 {
		t.Fatalf("corner = %v, want transparent", got)
	}
	edge := false
	for x := range vectorIconSide {
		if a := play.At(x, vectorIconSide*3/8).(color.NRGBA).A; a > 0 && a < 0xff {
			edge = true
			break
		}
	}
	if !edge {
		t.Fatal("no partly covered pixel along the triangle's edge")
	}

	inverted := vectorIcon{Name: IconPlay, Color: red, Invert: true}
	if got := inverted.At(2, 2).(color.NRGBA); got != (color.NRGBA{R: 0xff, A: 0xff}) {
		t.Fatalf("inverted corner = %v, want the color", got)
	}
	if got := inverted.At(vectorIconSide/2, vectorIconSide/2).(color.NRGBA); got != (color.NRGBA{R: iconInk.R, G: iconInk.G, B: iconInk.B, A: 0xff}) {
		t.Fatalf("inverted center = %v, want the ink", got)
	}
}

func TestTerminalIconButtonCentersItsGlyphUnderAPicture(t *testing.T) {
	button := Button{NodeBase: NodeBase{ID: "play", W: Fixed(8), H: Fixed(3), PadLeft: 1, PadRight: 1}, Icon: IconPlay, Label: "▶", Plain: true}
	c := ComposeWithColorProfile(button, 8, 3, colorprofile.TrueColor)
	if got, want := ansi.Strip(c.Frame), "\n   ▶\n"; got != want {
		t.Fatalf("tall icon button = %q, want %q", got, want)
	}
	if len(c.TerminalImages) != 1 {
		t.Fatalf("placements = %+v, want the icon", c.TerminalImages)
	}
	got := c.TerminalImages[0]
	icon, ok := got.Source.(vectorIcon)
	if !ok || icon.Name != IconPlay || got.Fit != ImageContain || got.Rect != (Rect{X: 1, W: 6, H: 3}) {
		t.Fatalf("placement = %+v, want the play shape inside the padding", got)
	}
}

func TestTerminalIconButtonOnTwoLinesIsAPictureOverItsGlyph(t *testing.T) {
	button := Button{NodeBase: NodeBase{W: Fixed(4), H: Fixed(2)}, Icon: IconPause, Label: "⏸", Plain: true}
	c := ComposeWithColorProfile(button, 4, 2, colorprofile.TrueColor)
	if !strings.Contains(ansi.Strip(c.Frame), "⏸") || len(c.TerminalImages) != 1 {
		t.Fatalf("two-line icon button = %q with %d placements, want its glyph under a picture", ansi.Strip(c.Frame), len(c.TerminalImages))
	}
}

func TestTerminalIconButtonKeepsItsGlyphOnOneLine(t *testing.T) {
	button := Button{NodeBase: NodeBase{W: Fixed(5), H: Fixed(3), Box: Box{Border: BorderRounded}}, Icon: IconPlay, Label: "▶", Plain: true}
	c := ComposeWithColorProfile(button, 5, 3, colorprofile.TrueColor)
	if !strings.Contains(ansi.Strip(c.Frame), "▶") || len(c.TerminalImages) != 0 {
		t.Fatalf("framed one-line icon button = %q with %d placements, want its glyph", ansi.Strip(c.Frame), len(c.TerminalImages))
	}
}

func TestTerminalIconButtonColor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		button Button
		want   color.RGBA
	}{
		{"default", Button{}, color.RGBA{R: 0xe4, G: 0xe4, B: 0xe4, A: 0xff}},
		{"fg", Button{Style: TextStyle{Fg: color.RGBA{R: 0x9d, G: 0x7b, B: 0xff, A: 0xff}}}, color.RGBA{R: 0x9d, G: 0x7b, B: 0xff, A: 0xff}},
		{"subtle", Button{Style: TextStyle{Role: TextSubtle}}, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}},
		{"disabled", Button{Disabled: true, Style: TextStyle{Fg: color.White}}, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}},
	} {
		if got := tc.button.iconColor(); got != tc.want {
			t.Errorf("%s: color = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestVectorIconKeepsAMargin(t *testing.T) {
	play := vectorIcon{Name: IconPlay, Color: color.RGBA{R: 0xff, A: 0xff}}
	edge := vectorIconSide * (1 - vectorIconScale) / 2
	for _, p := range [][2]int{{int(edge) - 2, vectorIconSide / 2}, {vectorIconSide / 2, int(edge) - 2}, {vectorIconSide - int(edge) + 2, vectorIconSide / 2}} {
		if a := play.At(p[0], p[1]).(color.NRGBA).A; a != 0 {
			t.Fatalf("pixel %v in the margin has alpha %d", p, a)
		}
	}
}
