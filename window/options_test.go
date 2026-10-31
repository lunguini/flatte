package window

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/lunguini/flatte"
)

var (
	_ flatte.Option = Options{}
	_ flatte.Option = MediaControls{}
)

func TestSupportedMatchesWindowBuild(t *testing.T) {
	if got, want := Supported(), runtime.GOOS == "darwin"; got != want {
		t.Fatalf("Supported() = %t on %s, want %t", got, runtime.GOOS, want)
	}
}

func TestRunRejectsWindowAndMediaControlsTogether(t *testing.T) {
	if !Supported() {
		t.Skip("native host options are not installed on this platform")
	}
	app := flatte.App[struct{}]{
		State: new(struct{}),
		View:  func(*struct{}, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	for _, options := range [][]flatte.Option{{Options{}, MediaControls{}}, {MediaControls{}, Options{}}} {
		err := flatte.Run(context.Background(), app, options...)
		if !errors.Is(err, flatte.ErrMultipleHosts) {
			t.Fatalf("Run(%T, %T) error = %v, want ErrMultipleHosts", options[0], options[1], err)
		}
	}
}
