//go:build !darwin

package window

import (
	"image"

	"github.com/lunguini/flatte"
	flattehost "github.com/lunguini/flatte/host"
)

func Supported() bool { return false }

func (Options) ConfigureRun(*flattehost.RunConfig) {}

func snapshot[S any](flatte.App[S], Options, float64) (image.Image, error) {
	return nil, ErrUnsupported
}
