package layout

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestLabelTruncate(t *testing.T) {
	l := Label{Text: "Insane — Alastor (Hazbin Hotel)", Truncate: true}
	if got := ansi.Strip(l.Render(Rect{W: 12, H: 1})); got != "Insane — Al…" {
		t.Fatalf("truncated %q", got)
	}
	if got := ansi.Strip(l.Render(Rect{W: 40, H: 1})); strings.TrimRight(got, " ") != l.Text {
		t.Fatalf("a fitting label changed: %q", got)
	}
	runs := Label{Runs: []Run{{Text: "Overthink"}, {Text: "  Kuo, Lin", Style: TextStyle{Role: TextSubtle}}}, Truncate: true}
	if got := ansi.Strip(runs.Render(Rect{W: 13, H: 1})); got != "Overthink  K…" {
		t.Fatalf("runs truncated %q", got)
	}
	// Without Truncate a long label is clipped as before.
	l.Truncate = false
	if got := ansi.Strip(l.Render(Rect{W: 12, H: 1})); strings.Contains(got, "…") {
		t.Fatalf("clipped label has an ellipsis: %q", got)
	}
}
