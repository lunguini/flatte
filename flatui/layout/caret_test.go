package layout

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

// boundaries are the grapheme-cluster boundaries of s, as byte offsets.
func boundaries(s string) []int {
	out := []int{0}
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		_, to := g.Positions()
		out = append(out, to)
	}
	return out
}

// TestCaretAtRoundTripsTheCursor: for every caret position, the cursor
// cell the Input reports maps back to that offset through CaretAt —
// single-line and multi-line, wrapped or not, scrolled.
func TestCaretAtRoundTripsTheCursor(t *testing.T) {
	pieces := []string{"ab", " ", "cd", "界", "é", "🙂", "\n", "xyz "}
	for seed := range 300 {
		rng := rand.New(rand.NewPCG(uint64(seed), 3))
		var b strings.Builder
		for range rng.IntN(30) {
			b.WriteString(pieces[rng.IntN(len(pieces))])
		}
		multi := rng.IntN(3) > 0
		value := b.String()
		if !multi {
			value = strings.ReplaceAll(value, "\n", "")
		}
		w, h := 2+rng.IntN(10), 1+rng.IntN(5)
		if !multi {
			h = 1
		}
		wrap, offY := rng.IntN(2) == 0, rng.IntN(4)
		for _, at := range boundaries(value) {
			in := Input{NodeBase: NodeBase{ID: "in", W: Fixed(w), H: Fixed(h)}, Value: value, Caret: at, Focused: true,
				Multiline: multi, Wrap: wrap, OffsetY: offY}
			c := Compose(Col{Children: []Node{in}}, w, h)
			x, y, ok := c.Cursor()
			if !ok {
				continue // a cursor past a full row's end, outside the frame
			}
			got, ok := c.CaretAt(x, y)
			if !ok || got != at {
				t.Fatalf("seed %d (%dx%d multi=%v wrap=%v %q): caret %d at cell %d,%d maps back to %d (%v)",
					seed, w, h, multi, wrap, value, at, x, y, got, ok)
			}
		}
	}
}

func TestCaretAtClampsAndMisses(t *testing.T) {
	root := Col{Children: []Node{
		Label{Text: "name:"},
		Input{NodeBase: NodeBase{ID: "in", H: Fixed(2)}, Value: "ab\ncdef", Multiline: true},
		Label{NodeBase: NodeBase{ID: "label"}, Text: "x"},
	}}
	c := Compose(root, 10, 5)
	for _, tc := range []struct{ x, y, want int }{
		{0, 1, 0}, {1, 1, 1}, {9, 1, 2}, // past "ab": its end
		{2, 2, 5}, {9, 2, 7},
	} {
		if got, ok := c.CaretAt(tc.x, tc.y); !ok || got != tc.want {
			t.Errorf("CaretAt(%d, %d) = %d %v, want %d", tc.x, tc.y, got, ok, tc.want)
		}
	}
	if _, ok := c.CaretAt(0, 3); ok {
		t.Error("CaretAt over a Label reports a caret")
	}
	if _, ok := c.CaretAt(0, 0); ok {
		t.Error("CaretAt over an unID'd node reports a caret")
	}
}
