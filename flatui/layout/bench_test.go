package layout

import (
	"math/rand/v2"
	"testing"
)

type corpusCase struct {
	root Node
	w, h int
}

func corpusCases() []corpusCase {
	out := make([]corpusCase, corpusTrees)
	for i := range out {
		rng := rand.New(rand.NewPCG(uint64(i), 0x5eed))
		root := (&treeGen{rng: rng}).node(0)
		out[i] = corpusCase{root, 1 + rng.IntN(60), 1 + rng.IntN(30)}
	}
	return out
}

// BenchmarkComposeCorpus composes every corpus tree once per iteration.
func BenchmarkComposeCorpus(b *testing.B) {
	cases := corpusCases()
	b.ReportAllocs()
	for b.Loop() {
		for _, c := range cases {
			Compose(c.root, c.w, c.h)
		}
	}
}

// BenchmarkSolveCorpus is the geometry-only walk over the same trees.
func BenchmarkSolveCorpus(b *testing.B) {
	cases := corpusCases()
	b.ReportAllocs()
	for b.Loop() {
		for _, c := range cases {
			Solve(c.root, c.w, c.h)
		}
	}
}
