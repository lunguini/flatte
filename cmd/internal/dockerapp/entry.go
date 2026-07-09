package dockerapp

import (
	"time"

	"github.com/lunguini/flatte"
)

// SessionFile is the gob path the standalone binary persists to.
const SessionFile = sessionFile

// NewStateFromSession rebuilds app state from a persisted session.
func NewStateFromSession(s SessionState) *State { return newStateFromSession(s) }

// Session returns the persistable slice of the current state.
func (s *State) Session() SessionState { return s.toSession() }

// Tick is the fx-free stepping API for hosting: it advances the synthetic
// stats and log feeds that Init normally drives via async effects. frame is the
// host's animation counter; stats advance on a slower cadence than logs.
func Tick(s *State, now time.Time, frame int) {
	if frame%6 == 0 {
		s.containers.tickStats(now)
	}
	s.containers.hostAdvanceLogs()
}

// Init arms the background pollers: a stats tick, scoped per-container log
// streams, and a one-time glyph hint pushed into the activity feed.
func Init(s *State, fx flatte.Effects[State]) {
	flatte.Every(fx, "stats-poll", 1*time.Second, func(s *State, now time.Time) {
		s.containers.tickStats(now)
	})
	s.containers.startScopedLogs(s, fx)
	s.containers.pushActivity(tipForGlyphs(pickGlyphs()))
}
