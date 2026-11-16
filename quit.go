package flatte

// QuitRequest reports that the user asked to quit — ⌘Q or the main
// window's close button in a window, Ctrl+C under default quit in a
// terminal or a cell-grid page — in an app run WithQuitRequests. Nothing has stopped: the app
// quits with Effects.Quit when it is ready (after asking about unsaved
// changes, say) or keeps running.
type QuitRequest struct{}

func (QuitRequest) isEvent() {}

// WithQuitRequests makes hosts ask instead of quitting: the user's quit
// gestures arrive as a QuitRequest for the app to answer. Without it they
// quit at once, as before. In a browser only the default Ctrl+C quit is a
// quit gesture (closing a tab is not interceptable).
func WithQuitRequests() Option {
	return optionFunc(func(c *runConfig) { c.quitRequests = true })
}

// QuitRequests reports whether the app asked (WithQuitRequests) to receive
// a QuitRequest instead of quitting. Hosts call it when the user asks to
// quit: false means stop now; true means deliver QuitRequest and keep
// running until a Step reports Quit.
func (s *Session) QuitRequests() bool { return s.quitRequests }
