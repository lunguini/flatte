//go:build !darwin

package media

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
)

func TestControlsIsNoopWithoutNativeMediaHost(t *testing.T) {
	app := flatte.App[struct{}]{
		State: new(struct{}),
		View:  func(*struct{}, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	if err := flatte.Run(context.Background(), app, Controls{}, flatte.WithInput(strings.NewReader("")), flatte.WithOutput(io.Discard)); err != nil {
		t.Fatalf("Run with no-op Controls: %v", err)
	}
}
