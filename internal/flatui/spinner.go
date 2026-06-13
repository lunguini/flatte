package flatui

// Spinner is a frame-cycling activity indicator. The app advances it — via
// flatcore.Every or any other tick source on the loop goroutine — so the widget
// owns no goroutine and no timer, consistent with the rest of flatui. View
// returns the current frame.
type Spinner struct {
	frames []string
	index  int
}

// Preset frame sets. Apps may also pass their own to NewSpinner.
var (
	// SpinnerDots is the braille "dots" animation (single display cell wide).
	SpinnerDots = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	// SpinnerLine is the ASCII bar animation.
	SpinnerLine = []string{"|", "/", "-", "\\"}
)

// NewSpinner builds a Spinner over a copy of frames. With no frames, View
// returns "" and Tick is a no-op.
func NewSpinner(frames []string) Spinner {
	return Spinner{frames: append([]string(nil), frames...)}
}

// Tick advances to the next frame, wrapping at the end.
func (s *Spinner) Tick() {
	if len(s.frames) == 0 {
		return
	}
	s.index = (s.index + 1) % len(s.frames)
}

// View returns the current frame ("" when there are no frames).
func (s Spinner) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.frames[s.index]
}

// Frame returns the current frame index (for tests and indicators).
func (s Spinner) Frame() int { return s.index }
