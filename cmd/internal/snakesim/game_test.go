package snakesim

import "testing"

// playGreedy drives a recording game by steering the head toward the food each
// step (never reversing), until the game ends or the step cap is hit. It is a
// stand-in for a real player: it eats food, climbs levels, and turns often, so
// the recorded input log exercises the queue, scoring, and RNG-driven food
// placement. It returns the finished game (with Score and Inputs populated).
func playGreedy(seed uint64, stepCap int) *Game {
	g := New(seed)
	for !g.Over && g.Moves < stepCap {
		head := g.Snake[0]
		dx := g.Food.X - head.X
		dy := g.Food.Y - head.Y
		// Prefer the axis with the larger gap; Steer rejects reversals/no-ops on
		// its own, so an illegal pick is simply ignored and the snake keeps its
		// heading — exactly what a fumbling player would produce.
		if abs(dx) >= abs(dy) {
			if dx > 0 {
				g.Steer(Right)
			} else if dx < 0 {
				g.Steer(Left)
			}
		} else {
			if dy > 0 {
				g.Steer(Down)
			} else if dy < 0 {
				g.Steer(Up)
			}
		}
		g.Step()
	}
	return g
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestReplayReproducesRecordedScore is the load-bearing test for the leaderboard:
// a seed plus the accepted-turn log must re-derive the exact score with no
// rendering or timing, across many seeds. If this ever fails, the game and the
// verifier have drifted and no submitted score can be trusted.
func TestReplayReproducesRecordedScore(t *testing.T) {
	const stepCap = 100_000
	for seed := uint64(0); seed < 200; seed++ {
		g := playGreedy(seed, stepCap)
		got := Replay(g.Seed(), g.Inputs, stepCap)
		if got != g.Score {
			t.Fatalf("seed %d: replay score %d != recorded score %d (moves=%d, inputs=%d)",
				seed, got, g.Score, g.Moves, len(g.Inputs))
		}
	}
}

// TestReplayIsDeterministic runs the same recorded log twice and expects the
// same score — a guard against hidden nondeterminism (map iteration, global
// state) leaking into the sim.
func TestReplayIsDeterministic(t *testing.T) {
	g := playGreedy(12345, 100_000)
	a := Replay(g.Seed(), g.Inputs, 100_000)
	b := Replay(g.Seed(), g.Inputs, 100_000)
	if a != b {
		t.Fatalf("replay not deterministic: %d != %d", a, b)
	}
	if a != g.Score {
		t.Fatalf("replay %d != recorded %d", a, g.Score)
	}
}

// TestReplayEmptyLogRunsStraight verifies the no-input baseline: the snake keeps
// its initial rightward heading, eats nothing it isn't handed, and dies on the
// right wall with score 0. This pins the "cheater submits an empty log" path.
func TestReplayEmptyLogRunsStraight(t *testing.T) {
	if got := Replay(1, nil, 100_000); got != 0 {
		t.Fatalf("empty-log replay score = %d, want 0 (straight into the wall)", got)
	}
}

// TestStepCapBoundsReplay ensures a tiny cap stops the run early rather than
// spinning — a malformed submission with a huge implied game can't hang the
// verifier.
func TestStepCapBoundsReplay(t *testing.T) {
	g := playGreedy(7, 100_000)
	// A cap below the real length must not reproduce the full score.
	capped := Replay(g.Seed(), g.Inputs, 3)
	if capped >= g.Score && g.Score > 0 {
		t.Fatalf("cap=3 replay scored %d, expected less than the full %d", capped, g.Score)
	}
}
