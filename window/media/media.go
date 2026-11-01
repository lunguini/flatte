// Package media adds native system media controls to a terminal Run.
// A window host already connects media controls whenever App.NowPlaying is
// set, so Controls can be passed alongside window.Options without conflict.
package media

// Controls opts a macOS terminal run into the system Now Playing controls.
// On other platforms it is an option with no effect.
type Controls struct{}
