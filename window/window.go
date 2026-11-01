package window

import (
	"errors"
	"image"

	"github.com/lunguini/flatte"
)

// ErrUnsupported is returned by Snapshot on platforms without a window
// backend. Window Options fall back to Run's default host on those platforms.
var ErrUnsupported = errors.New("window: not supported on this platform")

// Options configures the window.
type Options struct {
	// Desktop opts into a background status-menu utility. Its main window
	// starts hidden, floats when shown, and hides instead of quitting on
	// close. It uses the same semantic renderer and native input as a normal
	// window. Nil keeps ordinary window behavior unchanged.
	Desktop *flatte.Desktop
	// Title is the window title until the app's Frame.Title sets one.
	Title string
	// Width and Height are the initial content size in columns and lines of
	// the UI font. Zero means 100×32.
	Width, Height int
	// Theme is the window's look. Nil follows the system's appearance,
	// switching between Dark and Light as it changes; Snapshot draws Dark.
	Theme *Theme
	// MinWidth and MinHeight are the smallest content size in columns and
	// lines (at least 20×4).
	MinWidth, MinHeight int
	// Autosave, when set, names the window's saved frame: its position and
	// size are restored from it at launch and saved as they change.
	Autosave string
	// HideTitleBar extends the content under the title bar: the frame fills
	// the whole window and the window's buttons float over its top-left
	// corner, which the app leaves clear (TitleBarHeight tall,
	// TitleBarButtonsWidth wide). The strip still drags the window.
	HideTitleBar bool
	// Translucent shows the desktop, blurred, through the window wherever
	// the frame draws no background of its own.
	Translucent bool
	// Fonts are font files (TrueType or OpenType) for this process to use
	// before the window draws, so Theme.Font can name a family the system
	// does not have — an app embeds its font and names it.
	Fonts [][]byte
}

// The window buttons' corner of a window with HideTitleBar, in points.
const (
	TitleBarHeight       = 28
	TitleBarButtonsWidth = 78
)

func (o Options) size() (cols, lines int) {
	cols, lines = o.Width, o.Height
	if cols <= 0 {
		cols = 100
	}
	if lines <= 0 {
		lines = 32
	}
	return cols, lines
}

// Snapshot renders app's first frame — after Init, the first ResizeEvent,
// and layout settling — offscreen at scale pixels per point, for visual
// tests and screenshots. It needs no display and may be called from any
// goroutine.
func Snapshot[S any](app flatte.App[S], opts Options, scale float64) (image.Image, error) {
	return snapshot(app, opts, scale)
}
