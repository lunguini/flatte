//go:build !js || !wasm

package flatte

import "context"

// Run owns the platform event/update/render loop until the app quits or ctx is
// cancelled. Native builds use the terminal; js/wasm builds mount the first
// element marked data-flatte in the browser DOM.
func Run[S any](ctx context.Context, app App[S], opts ...Option) (runErr error) {
	return runTerminal(ctx, app, opts...)
}
