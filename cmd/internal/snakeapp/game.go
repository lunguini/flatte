package snakeapp

import (
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/snakesim"
)

// Render and timing constants. The grid mechanics live in snakesim; here we
// keep only what the host adds on top: how wide a cell renders and how fast the
// snake moves per level.
const (
	cellW = 2 // columns per logical cell, so a cell reads roughly square

	// The ticker fires at a fixed baseInterval; speed comes from how many base
	// ticks make one snake step (stepTicks). This keeps a single long-lived
	// ScopeEvery ticker whose rate never changes — deterministic under the
	// fake clock — while the *effective* speed still rises with the level.
	baseInterval = 20 * time.Millisecond

	startMoveMs = 220 // level 1 move interval
	floorMoveMs = 80  // fastest move interval (level snakesim.MaxLevel)
	stepMoveMs  = 20  // interval shortens by this each level
)

// The speed schedule floors at snakesim.MaxLevel — the level at which
// startMoveMs has been shortened down to floorMoveMs. Keep the two in lockstep:
// if this expression goes negative the conversion fails to compile.
const _ = uint(snakesim.MaxLevel - (1 + (startMoveMs-floorMoveMs)/stepMoveMs))

// State is the single mutable app state. Only HighScore is exported, so gob
// (flatte.SaveState / LoadState) persists exactly the high score and nothing
// else; the live game is rebuilt from a seed on boot (see main.go).
type State struct {
	HighScore int // persisted

	// game is the pure simulation, shared with the headless verifier. It is an
	// unexported field, so gob ignores it — the whole board is re-derived from a
	// seed, never persisted.
	game *snakesim.Game

	// Host-side state layered on top of the sim: pause, the tick accumulators,
	// the flatte ticker, and the terminal size.
	paused    bool
	stepAccum int // base ticks accumulated toward the next snake step
	elapsedMs int // real milliseconds accumulated toward the next base tick (host-driven mode)

	// tick is the current ScopeEvery ticker. pause/resume/restart Cancel it and
	// (re-)arm a fresh one — this is the named-cancellation showcase.
	tick *flatte.Scope

	width, height int
}

// newGame builds a fresh playing state with a seeded RNG. Tests pass a fixed
// seed for byte-stable food placement; main seeds from the wall clock.
func newGame(seed uint64) *State {
	return &State{game: snakesim.New(seed)}
}

// reset returns the board to a new game while preserving HighScore and seed,
// then clears the host-side pause/tick state. snakesim.Reset replays the same
// food sequence from the seed — a property the tests rely on.
func (s *State) reset() {
	s.game.Reset()
	s.paused = false
	s.stepAccum = 0
}

// stepTicks is how many base ticks make one snake move at the current level.
func (s *State) stepTicks() int {
	return s.moveIntervalMs() / int(baseInterval/time.Millisecond)
}

// moveIntervalMs is the effective snake-move interval for the current level.
func (s *State) moveIntervalMs() int {
	ms := startMoveMs - (s.game.Level-1)*stepMoveMs
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
	if s.game.Over || s.paused {
		return
	}
	s.stepAccum++
	if s.stepAccum < s.stepTicks() {
		return
	}
	s.stepAccum = 0
	s.game.Step()
	if s.game.Score > s.HighScore {
		s.HighScore = s.game.Score
	}
}
