package flatui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestCardUsesCompactWidthAndBorders(t *testing.T) {
	frame := Card([]string{Title("Flat"), Subtle("sample"), "", "  body"}, 72)
	got := flatuitest.CleanFrame(frame)

	for _, want := range []string{
		"┌──────────┐",
		"│   Flat   │",
		"│  sample  │",
		"│          │",
		"│    body  │",
		"└──────────┘",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Card() missing %q:\n%s", want, got)
		}
	}
}

func TestCardCapsWidthToRenderContext(t *testing.T) {
	frame := Card([]string{"  this line is too long for the target width"}, 24)

	for _, line := range strings.Split(frame, "\n") {
		if width := lipgloss.Width(line); width > 24 {
			t.Fatalf("line width = %d, want <= 24:\n%q", width, line)
		}
	}
}

func TestCardMeasuresMultilineRows(t *testing.T) {
	frame := Card([]string{"x\na much longer row"}, 72)
	got := flatuitest.CleanFrame(frame)

	if !strings.Contains(got, "│  a much longer row  │") {
		t.Fatalf("Card() did not size to multiline content:\n%s", got)
	}
}

func TestOverlayCentersLayerOverBase(t *testing.T) {
	base := strings.Join([]string{
		"aaaaaaaaaa",
		"bbbbbbbbbb",
		"cccccccccc",
		"dddddddddd",
		"eeeeeeeeee",
	}, "\n")
	layer := strings.Join([]string{
		"XXXX",
		"YYYY",
	}, "\n")

	got := Overlay(base, layer)

	want := strings.Join([]string{
		"aaaaaaaaaa",
		"bbbXXXXbbb",
		"cccYYYYccc",
		"dddddddddd",
		"eeeeeeeeee",
	}, "\n")
	if got != want {
		t.Fatalf("Overlay() =\n%s\nwant:\n%s", got, want)
	}
}

func TestOverlayCoversTheLayerRectangle(t *testing.T) {
	base := strings.Join([]string{
		"aaaaaaaaaa",
		"bbbbbbbbbb",
		"cccccccccc",
	}, "\n")
	layer := strings.Join([]string{
		"XX",
		"Y",
	}, "\n")

	got := Overlay(base, layer)

	want := strings.Join([]string{
		"aaaaXXaaaa",
		"bbbbY bbbb",
		"cccccccccc",
	}, "\n")
	if got != want {
		t.Fatalf("Overlay() =\n%s\nwant:\n%s", got, want)
	}
}

func TestOverlayUsesVisibleWidthForStyledContent(t *testing.T) {
	base := Card([]string{"  one", "  two", "  a much wider background row", "  four", "  five", "  six"}, 40)
	layer := Card([]string{Title("Modal"), "  ok"}, 20)

	got := flatuitest.CleanFrame(Overlay(base, layer))

	if strings.Contains(got, "\n\n") {
		t.Fatalf("Overlay() stacked the layer instead of overlaying it:\n%s", got)
	}
	if !strings.Contains(got, "│   Modal   │") {
		t.Fatalf("Overlay() missing modal title:\n%s", got)
	}
	if !strings.Contains(got, "┌───────────┐") {
		t.Fatalf("Overlay() missing overlaid modal border:\n%s", got)
	}
}
