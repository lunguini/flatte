package flatui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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

func TestOverlayPreservesStyledBackground(t *testing.T) {
	row := "\x1b[44m" + strings.Repeat("x", 20) + "\x1b[m"
	base := strings.Join([]string{row, row, row}, "\n")

	overlaid := Overlay(base, "[ok]")

	middle := strings.Split(overlaid, "\n")[1]
	plain := ansi.Strip(middle)
	if !strings.Contains(plain, "[ok]") {
		t.Fatalf("overlay content missing from middle row: %q", plain)
	}
	if !strings.HasPrefix(plain, "xxxx") {
		t.Fatalf("base content left of overlay lost: %q", plain)
	}

	buf := uv.NewScreenBuffer(20, 3)
	uv.NewStyledString(overlaid).Draw(buf, buf.Bounds())
	if cell := buf.CellAt(0, 1); cell == nil || cell.Style.Bg == nil {
		t.Fatalf("background lost left of the overlay: %+v", cell)
	}
	if cell := buf.CellAt(19, 1); cell == nil || cell.Style.Bg == nil {
		t.Fatalf("background lost right of the overlay: %+v", cell)
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

func TestCardOriginPointsAtFirstContentCell(t *testing.T) {
	frame := Card([]string{"marker"}, 40)
	x, y := CardOrigin()
	rows := strings.Split(flatuitest.CleanFrame(frame), "\n")
	// Index by rune: the card border is a multi-byte, single-cell rune.
	row := []rune(rows[y])
	if x >= len(row) || !strings.HasPrefix(string(row[x:]), "marker") {
		t.Fatalf("CardOrigin() = (%d,%d), but row %d is %q", x, y, y, rows[y])
	}
}

func TestOverlayOriginMatchesOverlayPlacement(t *testing.T) {
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat("b", 20)+"\n", 7), "\n")
	layer := "XX\nXX"
	x, y := OverlayOrigin(base, layer)
	composed := strings.Split(Overlay(base, layer), "\n")
	if composed[y][x:x+2] != "XX" {
		t.Fatalf("OverlayOrigin() = (%d,%d), row %d = %q", x, y, y, composed[y])
	}
}
