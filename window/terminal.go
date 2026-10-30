package window

import (
	"context"

	"github.com/lunguini/flatte"
)

// RunTerminal runs app in the terminal and, when App.NowPlaying is set,
// publishes its media state to the operating system's media controls.
//
// On macOS this requires RunTerminal to be called from the process main
// goroutine, like Run. Other platforms run the terminal app without native
// media controls.
func RunTerminal[S any](ctx context.Context, app flatte.App[S], opts ...flatte.Option) error {
	if app.NowPlaying == nil {
		return flatte.Run(ctx, app, opts...)
	}
	return runTerminal(ctx, app, opts...)
}
