//go:build !darwin

package window

import "github.com/lunguini/flatte"

func (MediaControls) ConfigureRun(*flatte.RunConfig) {}
