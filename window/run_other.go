//go:build !darwin

package window

import (
	"context"
	"image"

	"github.com/lunguini/flatte"
)

func run[S any](context.Context, flatte.App[S], Options) error { return ErrUnsupported }

func snapshot[S any](flatte.App[S], Options, float64) (image.Image, error) {
	return nil, ErrUnsupported
}
