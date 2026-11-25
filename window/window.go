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
	// Appearance fixes the design system's appearance (AppearanceLight or
	// AppearanceDark) instead of following the system; Unknown follows it
	// (Snapshot draws Dark). It applies when Theme is nil.
	Appearance flatte.Appearance
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
	// TitleBarHeight, with HideTitleBar, is the strip's height in points
	// when the app draws a taller one (a toolbar, a tab strip): the whole
	// strip drags the window. On macOS the window buttons sit in the middle
	// of the system's title bar nearest that height, 40 (38 before macOS 26)
	// or 52 points tall, so a strip of one of those has them centered; the
	// 52-point one places them up to 80 points from the left. Zero or less
	// than the system's TitleBarHeight keeps the system's.
	TitleBarHeight int
	// Translucent shows the desktop, blurred, through the window wherever
	// the frame draws no background of its own — for good; a frame's
	// Frame.Translucent turns it on frame by frame instead.
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

// dark is whether the window draws its design's dark palette, given the
// system's appearance.
func (o Options) dark(system bool) bool {
	switch o.Appearance {
	case flatte.AppearanceLight:
		return false
	case flatte.AppearanceDark:
		return true
	}
	return system
}

// follows reports whether the window follows the system's appearance.
func (o Options) follows() bool { return o.Theme == nil && o.Appearance == flatte.AppearanceUnknown }

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
