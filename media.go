package flatte

import "time"

// NowPlaying is the app's current media snapshot for native system controls.
// Position is elapsed playback time; a native host lets the system advance
// it using Playing while the app is in the background.
type NowPlaying struct {
	Title, Artist, Album string
	Duration, Position   time.Duration
	Playing              bool
	CanNext, CanPrevious bool
}

// MediaCommand is a command from the system media controls.
type MediaCommand uint8

const (
	MediaPlay MediaCommand = iota + 1
	MediaPause
	MediaTogglePlayPause
	MediaNext
	MediaPrevious
)

// MediaEvent delivers a system media command to the app. The app decides
// what the command means and performs its own playback actions.
type MediaEvent struct{ Command MediaCommand }

func (MediaEvent) isEvent() {}
