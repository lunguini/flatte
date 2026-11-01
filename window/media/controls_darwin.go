//go:build darwin && !wasm

package media

import (
	"context"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/window/internal/appkit"
)

var _ flatte.Option = Controls{}

func (Controls) ConfigureRun(config *flatte.RunConfig) {
	config.UseFallbackHost(terminalHost{})
}

type terminalHost struct{}

func (terminalHost) Run(ctx context.Context, session *flatte.Session) error {
	return appkit.RunTerminalMedia(ctx, session)
}
