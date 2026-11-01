//go:build !darwin && !(js && wasm)

package window

import (
	"image"

	"github.com/lunguini/flatte"
)

func Supported() bool { return false }

func (Options) ConfigureRun(*flatte.RunConfig) {}

func snapshot[S any](flatte.App[S], Options, float64) (image.Image, error) {
	return nil, ErrUnsupported
}
