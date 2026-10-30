//go:build !darwin

package window

import (
	"context"

	"github.com/lunguini/flatte"
)

func runTerminal[S any](ctx context.Context, app flatte.App[S], opts ...flatte.Option) error {
	return flatte.Run(ctx, app, opts...)
}
