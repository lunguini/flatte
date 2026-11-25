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

// KeepEnd keeps the end of a cut-off line.
func TestLabelKeepsItsEnd(t *testing.T) {
	for _, tc := range []struct {
		text string
		keep int
		w    int
		want string
	}{
		{"agentlist_test.go", 3, 10, "agentl….go"},
		{"agentlist_test.go", 3, 17, "agentlist_test.go"},
		{"agentlist_test.go", 0, 10, "agentlist…"},
		{"agentlist_test.go", 3, 4, "age…"}, // no room for it
		{"ünïcödé.txt", 4, 8, "ünï….txt"},
		{"abc", 9, 2, "a…"},
	} {
		c := Compose(Label{Text: tc.text, Truncate: true, KeepEnd: tc.keep}, tc.w, 1)
		if got := plainLines(c.Frame)[0]; got != tc.want {
			t.Errorf("%q in %d: %q, want %q", tc.text, tc.w, got, tc.want)
		}
	}
	rich := Label{Runs: []Run{{Text: "long", Style: TextStyle{Bold: true}}, {Text: "name.go"}}, Truncate: true, KeepEnd: 3}
	if got := plainLines(Compose(rich, 8, 1).Frame)[0]; got != "long….go" {
		t.Errorf("runs: %q", got)
	}
	if TruncatedTail("é", 1) != "" {
		t.Error("half a rune kept")
	}
}
