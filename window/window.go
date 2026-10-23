package window

import (
	"context"
	"errors"
	"image"

	"github.com/lunguini/flatte"
)

// ErrUnsupported is returned by Run and Snapshot on platforms without a
// window backend.
var ErrUnsupported = errors.New("window: not supported on this platform")

// Options configures the window.
type Options struct {
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

// Run opens a window and runs app in it until the app quits (fx.Quit), the
// window is closed or the app is quit from its menu, or ctx is done. It
// must be called from the main goroutine: the platform UI runs on the
// process's main thread. OnExit runs before Run returns.
func Run[S any](ctx context.Context, app flatte.App[S], opts Options) error {
	return run(ctx, app, opts)
}

// Snapshot renders app's first frame — after Init, the first ResizeEvent,
// and layout settling — offscreen at scale pixels per point, for visual
// tests and screenshots. It needs no display and may be called from any
// goroutine.
func Snapshot[S any](app flatte.App[S], opts Options, scale float64) (image.Image, error) {
	return snapshot(app, opts, scale)
}
