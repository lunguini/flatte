package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
)

const stateFile = ".flat-game-state.gob"

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

func main() {
	loaded := flatte.LoadState(stateFile, State{})
	s := newGame(uint64(time.Now().UnixNano()))
	s.HighScore = loaded.HighScore

	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  s,
		Init:   Init,
		Handle: Handle,
		View:   View,
		OnExit: func(s *State) {
			_ = flatte.SaveState(stateFile, *s)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
