package layout

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The solver corpus pins the cell solver's complete observable output —
// composed frame, rects, paint-order hits, cursor, and diagnostics — for a
// deterministic set of generated trees. It guards refactors of the solver
// (the generic cell/point solver must reproduce cells exactly). Regenerate
// only for an intended behavior change: FLAT_REGEN_LAYOUT_CORPUS=1.

const corpusTrees = 400

func TestSolverCorpus(t *testing.T) {
	var b strings.Builder
	for i := range corpusTrees {
		rng := rand.New(rand.NewPCG(uint64(i), 0x5eed))
		g := &treeGen{rng: rng}
		root := g.node(0)
		w, h := 1+rng.IntN(60), 1+rng.IntN(30)
		describeComposition(&b, i, root, w, h)
	}
	got := b.String()
	path := filepath.Join("testdata", "solver-corpus.golden")
	if os.Getenv("FLAT_REGEN_LAYOUT_CORPUS") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("corpus diverges at line %d:\ngot:  %s\nwant: %s", i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("corpus length differs: got %d lines, want %d", len(gl), len(wl))
	}
}

func describeComposition(b *strings.Builder, i int, root Node, w, h int) {
	c := Compose(root, w, h)
	fmt.Fprintf(b, "tree %d %dx%d frame %x\n", i, w, h, sha256.Sum256([]byte(c.Frame)))
	ids := make([]string, 0, len(c.Rects))
	for id := range c.Rects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(b, "  rect %s %v\n", id, c.Rects[id])
	}
	for _, d := range c.Diagnostics {
		fmt.Fprintf(b, "  dup %s %v %v\n", d.ID, d.FirstRect, d.Duplicate)
	}
	if x, y, ok := c.Cursor(); ok {
		fmt.Fprintf(b, "  cursor %d,%d\n", x, y)
	}
	var hits strings.Builder
	for y := -1; y <= h; y++ {
		for x := -1; x <= w; x++ {
			id, r := c.HitTest(x, y)
			if id != "" {
				fmt.Fprintf(&hits, " %d,%d=%s%v", x, y, id, r)
			}
		}
	}
	fmt.Fprintf(b, "  hits %x\n", sha256.Sum256([]byte(hits.String())))
	if solved := Solve(root, w, h); fmt.Sprint(solved) != fmt.Sprint(c.Rects) {
		fmt.Fprintf(b, "  SOLVE-DIVERGES\n")
	}
}

// treeGen builds random trees. Sizes deliberately include malformed values
// (negative fixed, NaN/Inf/negative weights, unknown kinds) because the
// solver's fallbacks are part of its contract.
type treeGen struct {
	rng *rand.Rand
	ids int
	// wrap turns some Labels into wrapping paragraphs. Off for the golden
	// corpus, which it would otherwise change.
	wrap bool
}

func (g *treeGen) size() Size {
	switch g.rng.IntN(12) {
	case 0, 1, 2:
		return Auto()
	case 3, 4, 5:
		return Fixed(g.rng.IntN(12))
	case 6, 7:
		return Grow(float64(1 + g.rng.IntN(3)))
	case 8:
		return Grow(g.rng.Float64())
	case 9:
		return Size{Kind: SizeFixed, Value: -3}
	case 10:
		return Size{Kind: SizeGrow, Weight: []float64{0, -1, math.NaN(), math.Inf(1), 1e308}[g.rng.IntN(5)]}
	default:
		return Size{Kind: SizeKind(7)}
	}
}

func (g *treeGen) base() NodeBase {
	nb := NodeBase{W: g.size(), H: g.size()}
	if g.rng.IntN(3) == 0 {
		nb.Pad = g.rng.IntN(3)
	}
	if g.rng.IntN(4) == 0 {
		nb.PadLeft, nb.PadTop = g.rng.IntN(3), g.rng.IntN(2)
	}
	if g.rng.IntN(3) == 0 {
		nb.Gap = g.rng.IntN(3) - (g.rng.IntN(5) / 4) // occasionally -1
	}
	switch g.rng.IntN(6) {
	case 0:
		nb.Bordered = true
	case 1:
		nb.Box = Box{Border: BorderNormal, BorderColor: lipgloss.Color("240")}
	case 2:
		nb.Box = Box{Background: lipgloss.Color("4")}
	}
	if g.rng.IntN(2) == 0 {
		g.ids++
		nb.ID = fmt.Sprintf("n%d", g.ids)
		if g.rng.IntN(15) == 0 {
			nb.ID = "dup"
		}
	}
	if g.rng.IntN(14) == 0 {
		nb.Overlay = true
	}
	return nb
}

func (g *treeGen) children(depth int) []Node {
	n := g.rng.IntN(5)
	out := make([]Node, n)
	for i := range out {
		out[i] = g.node(depth + 1)
	}
	return out
}

var corpusWords = []string{"a", "hello", "日本語", "x\ny", "wide 🙂 text", "", "multi\nline\ntext"}

func (g *treeGen) word() string { return corpusWords[g.rng.IntN(len(corpusWords))] }

func (g *treeGen) node(depth int) Node {
	kind := g.rng.IntN(9)
	if depth >= 4 {
		kind = 5 + g.rng.IntN(4) // leaves only
	}
	switch kind {
	case 0, 1:
		return Row{NodeBase: g.base(), Children: g.children(depth)}
	case 2, 3:
		return Col{NodeBase: g.base(), Children: g.children(depth)}
	case 4:
		s := Scroll{NodeBase: g.base(), OffsetY: g.rng.IntN(12) - 2}
		if g.rng.IntN(5) != 0 {
			s.Child = g.node(depth + 1)
		}
		return s
	case 5:
		return Text{NodeBase: g.base(), String: g.word()}
	case 6:
		if g.wrap {
			words := make([]string, 1+g.rng.IntN(8))
			for i := range words {
				words[i] = g.word()
			}
			return Label{NodeBase: g.base(), Text: strings.Join(words, " "), Wrap: true}
		}
		return Label{NodeBase: g.base(), Text: g.word(), Style: TextStyle{Role: TextRole(g.rng.IntN(4))}}
	case 7:
		v := g.word()
		return Input{NodeBase: g.base(), Value: v, Caret: g.rng.IntN(len(v) + 2), Focused: g.rng.IntN(2) == 0, Placeholder: "ph"}
	default:
		sp := NewSpacer()
		if g.rng.IntN(2) == 0 {
			sp.NodeBase = g.base()
		}
		return sp
	}
}
