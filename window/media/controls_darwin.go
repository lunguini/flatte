//go:build darwin && !wasm

package media

import (
	"context"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/host"
	"github.com/lunguini/flatte/window/internal/appkit"
)

var _ flatte.Option = Controls{}

func (Controls) ConfigureRun(config *host.RunConfig) {
	config.UseFallbackHost(terminalHost{})
}

type terminalHost struct{}

func (terminalHost) Run(ctx context.Context, session host.Session) error {
	return appkit.RunTerminalMedia(ctx, session)
}
