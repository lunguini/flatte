//go:build !darwin

package window

import flattehost "github.com/lunguini/flatte/host"

func (MediaControls) ConfigureRun(*flattehost.RunConfig) {}
