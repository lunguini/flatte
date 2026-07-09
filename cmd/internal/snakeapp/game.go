package snakeapp

import (
	"math/rand/v2"
	"time"

	"github.com/lunguini/flatte"
)

// Board and timing constants. The grid is logical cells; each renders cellW
// columns wide so a cell reads roughly square in a terminal. The board pane and
// side panel together fill an 80x24 frame (see view.go).
const (
	gridW = 26
	gridH = 22
	cellW = 2

	// The ticker fires at a fixed baseInterval; speed comes from how many base
	// ticks make one snake step (stepTicks). This keeps a single long-lived
	// ScopeEvery ticker whose rate never changes — deterministic under the
	// fake clock — while the *effective* speed still rises with the level.
	baseInterval = 20 * time.Millisecond

	startMoveMs = 220 // level 1 move interval
	floorMoveMs = 80  // fastest move interval (level maxLevel)
	stepMoveMs  = 20  // interval shortens by this each level

	foodPerLevel = 5
	maxLevel     = 1 + (startMoveMs-floorMoveMs)/stepMoveMs // = 8
)

type point struct{ x, y int }

type direction int

const (
	dirUp direction = iota
	dirDown
	dirLeft
	dirRight
)

func (d direction) vec() point {
	switch d {
	case dirUp:
		return point{0, -1}
	case dirDown:
		return point{0, 1}
	case dirLeft:
		return point{-1, 0}
	default:
		return point{1, 0}
	}
}

func (d direction) opposite() direction {
	switch d {
	case dirUp:
		return dirDown
	case dirDown:
		return dirUp
	case dirLeft:
		return dirRight
	default:
		return dirLeft
	}
}

// State is the single mutable app state. Only HighScore is exported, so gob
// (flatte.SaveState / LoadState) persists exactly the high score and nothing
// else; the live game is rebuilt from a seed on boot (see main.go).
type State struct {
	HighScore int // persisted

	// live game — unexported, so gob ignores it
	snake     []point
	dir       direction   // committed heading, applied each step
	dirQueue  []direction // buffered turns (≤2), applied one per step
	food      point
	score     int
	foodEaten int
	level     int
	over      bool
	paused    bool
	stepAccum int // base ticks accumulated toward the next snake step
	elapsedMs int // real milliseconds accumulated toward the next base tick (host-driven mode)

	seed uint64
	rng  *rand.Rand

	// tick is the current ScopeEvery ticker. pause/resume/restart Cancel it and
	// (re-)arm a fresh one — this is the named-cancellation showcase.
	tick *flatte.Scope

	width, height int
}

// newGame builds a fresh playing state with a seeded RNG. Tests pass a fixed
// seed for byte-stable food placement; main seeds from the wall clock.
func newGame(seed uint64) *State {
	s := &State{seed: seed}
	s.reset()
	return s
}

// reset returns the board to a new game while preserving HighScore and seed.
// Re-deriving the RNG from the seed means a restart replays the same food
// sequence — a property the tests rely on and a fair fixed challenge in play.
func (s *State) reset() {
	s.rng = rand.New(rand.NewPCG(s.seed, s.seed^0x9e3779b97f4a7c15))
	cx, cy := gridW/2, gridH/2
	s.snake = []point{{cx, cy}, {cx - 1, cy}, {cx - 2, cy}}
	s.dir = dirRight
	s.dirQueue = nil
	s.score = 0
	s.foodEaten = 0
	s.level = 1
	s.over = false
	s.paused = false
	s.stepAccum = 0
	s.placeFood()
}

// stepTicks is how many base ticks make one snake move at the current level.
func (s *State) stepTicks() int {
	return s.moveIntervalMs() / int(baseInterval/time.Millisecond)
}

// moveIntervalMs is the effective snake-move interval for the current level.
func (s *State) moveIntervalMs() int {
	ms := startMoveMs - (s.level-1)*stepMoveMs
	if ms < floorMoveMs {
		ms = floorMoveMs
	}
	return ms
}

// onTick is the fold body run by the ScopeEvery ticker. It advances the snake
// once every stepTicks base ticks. It receives no Effects (folds never do), so
// it cannot re-arm the ticker — which is exactly why speed is expressed as a
// per-step tick count rather than by cancelling and restarting Every here.
func (s *State) onTick() {
	if s.over || s.paused {
		return
	}
	s.stepAccum++
	if s.stepAccum < s.stepTicks() {
		return
	}
	s.stepAccum = 0
	s.step()
}

// step advances the snake one cell, applying the buffered turn, and resolves
// food and collisions.
func (s *State) step() {
	// Apply one buffered turn (already validated against reversal on input).
	if len(s.dirQueue) > 0 {
		s.dir = s.dirQueue[0]
		s.dirQueue = s.dirQueue[1:]
	}

	v := s.dir.vec()
	head := s.snake[0]
	next := point{head.x + v.x, head.y + v.y}

	// Wall collision.
	if next.x < 0 || next.x >= gridW || next.y < 0 || next.y >= gridH {
		s.gameOver()
		return
	}
	// Self collision. The tail cell is about to move, so it is only a collision
	// when the snake is growing (tail stays put this step).
	eating := next == s.food
	limit := len(s.snake)
	if !eating {
		limit-- // tail vacates, so ignore it
	}
	for i := 0; i < limit; i++ {
		if s.snake[i] == next {
			s.gameOver()
			return
		}
	}

	// Move: push new head on the front.
	s.snake = append([]point{next}, s.snake...)
	if eating {
		s.foodEaten++
		s.level = 1 + s.foodEaten/foodPerLevel
		if s.level > maxLevel {
			s.level = maxLevel
		}
		s.score += s.level // each food scores the current level
		if s.score > s.HighScore {
			s.HighScore = s.score
		}
		s.placeFood()
	} else {
		s.snake = s.snake[:len(s.snake)-1] // drop the tail
	}
}

func (s *State) gameOver() {
	s.over = true
}

// placeFood puts food on a uniformly random empty cell via rejection sampling.
// Deterministic given the RNG state and the snake, so goldens stay stable.
func (s *State) placeFood() {
	occupied := make(map[point]bool, len(s.snake))
	for _, p := range s.snake {
		occupied[p] = true
	}
	// Guard against a full board (win condition) so we never spin forever.
	if len(occupied) >= gridW*gridH {
		return
	}
	for {
		p := point{s.rng.IntN(gridW), s.rng.IntN(gridH)}
		if !occupied[p] {
			s.food = p
			return
		}
	}
}

// steer buffers a turn if it is neither a reversal of the committed heading nor
// a no-op. Only the latest valid request survives to the next step, so at most
// one turn applies per step.
func (s *State) steer(d direction) {
	// Validate against the EFFECTIVE heading — the last queued turn if any,
	// else the committed direction. A fast second press within one tick
	// (up then left while moving right) must queue behind the first turn,
	// not be dropped as a "reversal" of a heading the snake is about to
	// leave. Two queued turns is a full tick of lookahead; further presses
	// in the same window are ignored.
	eff := s.dir
	if n := len(s.dirQueue); n > 0 {
		eff = s.dirQueue[n-1]
	}
	if d == eff || d == eff.opposite() {
		return
	}
	if len(s.dirQueue) < 2 {
		s.dirQueue = append(s.dirQueue, d)
	}
}
