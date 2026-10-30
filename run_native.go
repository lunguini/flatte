//go:build !js || !wasm

package flatte

import "context"

// Run owns the selected host's event/update/render loop until the app quits or
// ctx is cancelled. Native builds use the terminal by default; an Option may
// select another host. js/wasm builds use the browser page by default.
func Run[S any](ctx context.Context, app App[S], opts ...Option) (runErr error) {
	config := applyRunOptions(nativeRunConfig(), opts...)
	if handled, err := runWithHost(ctx, app, config); handled {
		return err
	}
	return runTerminalConfig(ctx, app, config.config)
}
