//go:build !darwin || wasm

package media

import (
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/host"
)

var _ flatte.Option = Controls{}

func (Controls) ConfigureRun(*host.RunConfig) {}
