package window

import (
	"context"

	"github.com/lunguini/flatte"
)

// MediaControls opts into native media controls while Run uses the terminal
// host. It has no effect on platforms without the native media integration.
type MediaControls struct{}

// RunTerminal is retained temporarily for source migration. Use flatte.Run
// with MediaControls when terminal media integration is wanted.
//
// Deprecated: call flatte.Run(ctx, app, opts..., window.MediaControls{}).
func RunTerminal[S any](ctx context.Context, app flatte.App[S], opts ...flatte.Option) error {
	return flatte.Run(ctx, app, append(opts, MediaControls{})...)
}
