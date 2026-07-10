// Package snakesim is the pure, dependency-free Snake simulation shared by the
// playable game (cmd/internal/snakeapp) and the headless replay verifier. It
// carries no flatte, no rendering, and no wall-clock timing — just deterministic
// grid mechanics seeded by a uint64. Because a step is timing-independent, a
// seed plus the ordered log of accepted turns reproduces a run exactly (see
// Replay), which is what lets a server re-derive a claimed score without trust.
//
// Keep the import set minimal (math/rand/v2 only) so the package compiles for
// TinyGo's wasm-unknown target with no WASI or runtime imports.
package snakesim

import "math/rand/v2"

// Board dimensions and difficulty caps. These are game rules, so they live with
// the simulation; the host derives its render geometry and speed schedule from
// them (see snakeapp).
const (
	GridW        = 26
	GridH        = 22
	FoodPerLevel = 5

	// MaxLevel caps difficulty: past it, each additional food scores the same
	// and (in the host) the move interval stops shortening. The host's speed
	// schedule floors at exactly this level, and a compile-time check in
	// snakeapp keeps the two in lockstep.
	MaxLevel = 8
)

// Point is a board cell in logical grid coordinates.
type Point struct{ X, Y int }

// Direction is a cardinal heading.
type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

// Vec is the unit step for a heading.
func (d Direction) Vec() Point {
	switch d {
	case Up:
		return Point{0, -1}
	case Down:
		return Point{0, 1}
	case Left:
		return Point{-1, 0}
	default:
		return Point{1, 0}
	}
}

// Opposite is the reversed heading.
func (d Direction) Opposite() Direction {
	switch d {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	default:
		return Left
	}
}

// Input is one accepted turn in a recorded run: Move is the number of steps the
// snake had completed when the turn was accepted, and Dir is the requested
// heading. A seed plus an ordered Input log is a complete, timing-independent
// description of a game — feed it to Replay to reproduce the exact score.
type Input struct {
	Move int       `json:"move"`
	Dir  Direction `json:"dir"`
}

// Game is the mutable simulation state. All gameplay fields are exported so the
// host can render them and a verifier can read the final Score; build one with
// New (live, recording) or via Replay (headless, non-recording).
type Game struct {
	Snake     []Point
	Dir       Direction   // committed heading, applied each step
	DirQueue  []Direction // buffered turns (≤2), applied one per step
	Food      Point
	Score     int
	FoodEaten int
	Level     int
	Over      bool

	// Moves is the count of completed steps. It is also the index a newly
	// accepted turn records against, which is what makes recording and Replay
	// line up: a turn recorded at Moves==k is re-applied just before step k+1.
	Moves int

	// Inputs is the accepted-turn log, populated only while recording. It plus
	// Seed is everything a server needs to re-derive Score.
	Inputs []Input

	seed   uint64
	rng    *rand.Rand
	record bool
}

// New builds a fresh recording game with a seeded RNG. Tests pass a fixed seed
// for byte-stable food placement; hosts seed from the wall clock.
func New(seed uint64) *Game {
	g := &Game{seed: seed, record: true}
	g.Reset()
	return g
}

// Seed is the RNG seed this game was built from; submit it alongside Inputs.
func (g *Game) Seed() uint64 { return g.seed }

// Reset returns the board to a new game while preserving the seed. Re-deriving
// the RNG from the seed means a restart replays the same food sequence — a
// property tests rely on and a fair fixed challenge in play. It also clears the
// input log, since a reset begins a new recordable run.
func (g *Game) Reset() {
	g.rng = rand.New(rand.NewPCG(g.seed, g.seed^0x9e3779b97f4a7c15))
	cx, cy := GridW/2, GridH/2
	g.Snake = []Point{{cx, cy}, {cx - 1, cy}, {cx - 2, cy}}
	g.Dir = Right
	g.DirQueue = nil
	g.Score = 0
	g.FoodEaten = 0
	g.Level = 1
	g.Over = false
	g.Moves = 0
	g.Inputs = nil
	g.placeFood()
}

// Steer buffers a turn if it is neither a reversal of the effective heading nor
// a no-op, recording it in the input log when accepted. Only the latest valid
// request survives to the next step, so at most one turn applies per step.
func (g *Game) Steer(d Direction) {
	// Validate against the EFFECTIVE heading — the last queued turn if any, else
	// the committed direction. A fast second press within one step window (up
	// then left while moving right) must queue behind the first turn, not be
	// dropped as a "reversal" of a heading the snake is about to leave. Two
	// queued turns is a full step of lookahead; further presses are ignored.
	eff := g.Dir
	if n := len(g.DirQueue); n > 0 {
		eff = g.DirQueue[n-1]
	}
	if d == eff || d == eff.Opposite() {
		return
	}
	if len(g.DirQueue) < 2 {
		g.DirQueue = append(g.DirQueue, d)
		if g.record {
			g.Inputs = append(g.Inputs, Input{Move: g.Moves, Dir: d})
		}
	}
}

// Step advances the snake one cell, applying the buffered turn, and resolves
// food and collisions. It is a no-op once the game is over. On a completed step
// (not one that ends the game) it increments Moves.
func (g *Game) Step() {
	if g.Over {
		return
	}
	// Apply one buffered turn (already validated against reversal on input).
	if len(g.DirQueue) > 0 {
		g.Dir = g.DirQueue[0]
		g.DirQueue = g.DirQueue[1:]
	}

	v := g.Dir.Vec()
	head := g.Snake[0]
	next := Point{head.X + v.X, head.Y + v.Y}

	// Wall collision.
	if next.X < 0 || next.X >= GridW || next.Y < 0 || next.Y >= GridH {
		g.Over = true
		return
	}
	// Self collision. The tail cell is about to move, so it is only a collision
	// when the snake is growing (tail stays put this step).
	eating := next == g.Food
	limit := len(g.Snake)
	if !eating {
		limit-- // tail vacates, so ignore it
	}
	for i := 0; i < limit; i++ {
		if g.Snake[i] == next {
			g.Over = true
			return
		}
	}

	// Move: push new head on the front.
	g.Snake = append([]Point{next}, g.Snake...)
	if eating {
		g.FoodEaten++
		g.Level = 1 + g.FoodEaten/FoodPerLevel
		if g.Level > MaxLevel {
			g.Level = MaxLevel
		}
		g.Score += g.Level // each food scores the current level
		g.placeFood()
	} else {
		g.Snake = g.Snake[:len(g.Snake)-1] // drop the tail
	}
	g.Moves++
}

// placeFood puts food on a uniformly random empty cell via rejection sampling.
// Deterministic given the RNG state and the snake, so replays and goldens stay
// stable.
func (g *Game) placeFood() {
	occupied := make(map[Point]bool, len(g.Snake))
	for _, p := range g.Snake {
		occupied[p] = true
	}
	// Guard against a full board (win condition) so we never spin forever.
	if len(occupied) >= GridW*GridH {
		return
	}
	for {
		p := Point{g.rng.IntN(GridW), g.rng.IntN(GridH)}
		if !occupied[p] {
			g.Food = p
			return
		}
	}
}

// Replay re-runs a game from its seed and accepted-turn log and returns the
// resulting Score. It is the verifier's core: identical mechanics with no
// rendering or timing, so the score is a pure function of (seed, inputs).
// stepCap bounds the run so a malformed log cannot spin forever; inputs whose
// Move index is skipped over (out of order, or past game end) are simply
// dropped, which can only lower the replayed score, never inflate it.
func Replay(seed uint64, inputs []Input, stepCap int) int {
	g := &Game{seed: seed} // record stays false
	g.Reset()
	i := 0
	for !g.Over && g.Moves < stepCap {
		for i < len(inputs) && inputs[i].Move == g.Moves {
			g.Steer(inputs[i].Dir)
			i++
		}
		g.Step()
	}
	return g.Score
}
