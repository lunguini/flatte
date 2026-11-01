//go:build !darwin || wasm

package media

import (
	"github.com/lunguini/flatte"
)

var _ flatte.Option = Controls{}

func (Controls) ConfigureRun(*flatte.RunConfig) {}
