package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/lunguini/flat/flatest"
)

func TestCardUsesCompactWidthAndBorders(t *testing.T) {
	frame := Card([]string{Title("Flat"), Subtle("sample"), "", "  body"}, 72)
	got := flatest.CleanFrame(frame)

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

func TestCardWithStyleUsesProvidedStyles(t *testing.T) {
	got := CardWithStyle([]string{"hello"}, 20, CardStyle{
		BorderForeground: lipgloss.Color("5"),
	})
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("CardWithStyle() missing ANSI styling: %q", got)
	}
	if clean := ansi.Strip(got); !strings.Contains(clean, "hello") {
		t.Fatalf("CardWithStyle() missing content: %q", clean)
	}
}

func TestTitleAndSubtleWithStyleUseProvidedStyle(t *testing.T) {
	title := TitleWithStyle("Hi", lipgloss.NewStyle().Foreground(lipgloss.Color("2")))
	if !strings.Contains(title, "\x1b[") || ansi.Strip(title) != "Hi" {
		t.Fatalf("TitleWithStyle() = %q", title)
	}

	subtle := SubtleWithStyle("lo", lipgloss.NewStyle().Bold(true))
	if !strings.Contains(subtle, "\x1b[") || ansi.Strip(subtle) != "lo" {
		t.Fatalf("SubtleWithStyle() = %q", subtle)
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
	got := flatest.CleanFrame(frame)

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

	got := flatest.CleanFrame(Overlay(base, layer))

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
	rows := strings.Split(flatest.CleanFrame(frame), "\n")
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

func TestCardBodyWidthMatchesCardChrome(t *testing.T) {
	const total = 40
	bw := CardBodyWidth(total)
	// A body line exactly CardBodyWidth wide must not push the card past total.
	card := Card([]string{strings.Repeat("x", bw)}, total)
	if w := lipgloss.Width(strings.Split(card, "\n")[0]); w != total {
		t.Fatalf("card width = %d, want %d (body sized via CardBodyWidth=%d)", w, total, bw)
	}
}

func TestCardBodyHeightMatchesCardChrome(t *testing.T) {
	const total, pinned = 12, 3
	bh := CardBodyHeight(total, pinned)
	lines := make([]string, pinned+bh)
	for i := range lines {
		lines[i] = "x"
	}
	rows := strings.Split(Card(lines, 40), "\n")
	if len(rows) != total {
		t.Fatalf("card = %d rows, want %d (pinned %d + body %d + 2 border)", len(rows), total, pinned, bh)
	}
}

func TestCardBodyDimensionsClampToZero(t *testing.T) {
	if got := CardBodyHeight(1, 5); got != 0 {
		t.Fatalf("CardBodyHeight(1,5) = %d, want 0", got)
	}
	if got := CardBodyWidth(2); got != 0 {
		t.Fatalf("CardBodyWidth(2) = %d, want 0", got)
	}
}
