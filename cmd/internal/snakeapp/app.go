package snakeapp

import (
	"time"

	"github.com/lunguini/flatte"
)

// NewGame builds a fresh playing state with a seeded RNG. Tests pass a fixed
// seed for byte-stable food placement; callers (the standalone binary and the
// landing host) seed from the wall clock.
func NewGame(seed uint64) *State { return newGame(seed) }

// TickElapsed advances the game by a real elapsed duration, decoupling snake
// speed from the host's tick cadence. The standalone binary ticks at the 20ms
// baseInterval; a host that ticks at a different rate (the landing showcase at
// ~120ms, and a browser setInterval that may throttle further) passes the wall
// time it actually observed, and this replays the right number of base ticks so
// the effective speed matches the native game. onTick self-guards on paused/over.
func TickElapsed(s *State, d time.Duration) {
	base := int(baseInterval / time.Millisecond)
	if base <= 0 {
		return
	}
	s.elapsedMs += int(d / time.Millisecond)
	for s.elapsedMs >= base {
		s.elapsedMs -= base
		s.onTick()
	}
}

// Init arms the game ticker once the loop is running.
func Init(s *State, fx flatte.Effects[State]) {
	s.startTicker(fx)
}

// startTicker cancels any existing ticker and arms a fresh ScopeEvery under a
// new scope. flatte.Scope is the named-cancellation primitive for interval
// effects: Scope.Cancel() cancels the scope's context, which drops the ticker
// (flatte.Cancel only cancels Latest work, not Every). Because a fold receives
// no Effects, every arm/cancel happens from Init or Handle — never mid-tick.
func (s *State) startTicker(fx flatte.Effects[State]) {
	if s.tick != nil {
		s.tick.Cancel()
	}
	s.tick = flatte.NewScope(fx, "tick")
	flatte.ScopeEvery(s.tick, fx, baseInterval, func(st *State, _ time.Time) {
		st.onTick()
	})
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.width, s.height = e.Width, e.Height
	case flatte.KeyEvent:
		s.handleKey(e, fx)
	}
}

func (s *State) handleKey(key flatte.KeyEvent, fx flatte.Effects[State]) {
	// Quit is always available.
	if key.Key == flatte.KeyEscape {
		fx.Quit()
		return
	}
	if key.Key == flatte.KeyCharacter && (key.Rune == 'q' || key.Rune == 'Q') {
		fx.Quit()
		return
	}

	// While the game-over overlay is up, only r/q act.
	if s.over {
		if key.Key == flatte.KeyCharacter && (key.Rune == 'r' || key.Rune == 'R') {
			s.restart(fx)
		}
		return
	}

	switch key.Key {
	case flatte.KeyUp:
		s.steer(dirUp)
	case flatte.KeyDown:
		s.steer(dirDown)
	case flatte.KeyLeft:
		s.steer(dirLeft)
	case flatte.KeyRight:
		s.steer(dirRight)
	case flatte.KeyCharacter:
		switch key.Rune {
		case 'w', 'W':
			s.steer(dirUp)
		case 's', 'S':
			s.steer(dirDown)
		case 'a', 'A':
			s.steer(dirLeft)
		case 'd', 'D':
			s.steer(dirRight)
		case 'p', 'P':
			s.togglePause(fx)
		case 'r', 'R':
			s.restart(fx)
		}
	}
}

// togglePause stops the ticker on pause and arms a fresh one on resume, so a
// paused game truly stops ticking (no wasted folds) — the pause path is the
// clearest demonstration of Scope.Cancel + re-arm.
func (s *State) togglePause(fx flatte.Effects[State]) {
	if s.paused {
		s.paused = false
		s.startTicker(fx)
		return
	}
	s.paused = true
	if s.tick != nil {
		s.tick.Cancel()
	}
}

// restart resets the board (keeping HighScore and seed) and re-arms the ticker.
func (s *State) restart(fx flatte.Effects[State]) {
	s.reset()
	s.startTicker(fx)
}
