package window

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	flattehost "github.com/lunguini/flatte/host"
	"github.com/lunguini/flatte/window/media"
)

var (
	_ flatte.Option = Options{}
	_ flatte.Option = media.Controls{}
)

func TestSupportedMatchesWindowBuild(t *testing.T) {
	if got, want := Supported(), runtime.GOOS == "darwin"; got != want {
		t.Fatalf("Supported() = %t on %s, want %t", got, runtime.GOOS, want)
	}
}

func TestRunAllowsWindowAndMediaControlsTogether(t *testing.T) {
	if !Supported() {
		t.Skip("native host options are not installed on this platform")
	}
	app := flatte.App[struct{}]{
		State: new(struct{}),
		View:  func(*struct{}, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
		NowPlaying: func(*struct{}) *flatte.NowPlaying {
			return &flatte.NowPlaying{Title: "test"}
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, options := range [][]flatte.Option{{Options{}}, {media.Controls{}}, {Options{}, media.Controls{}}, {media.Controls{}, Options{}}} {
		options = append(options, flatte.WithInput(strings.NewReader("")), flatte.WithOutput(io.Discard))
		err := flatte.Run(ctx, app, options...)
		if errors.Is(err, flatte.ErrMultipleHosts) {
			t.Fatalf("Run(%T...) returned ErrMultipleHosts for a compatible combination", options[0])
		}
	}
}

type optionTestHost struct{ ran *bool }

func (h optionTestHost) Run(context.Context, flattehost.Session) error {
	*h.ran = true
	return nil
}

type optionTestHostOption struct{ host flattehost.Host }

func (o optionTestHostOption) ConfigureRun(config *flattehost.RunConfig) { config.UseHost(o.host) }

func TestMediaControlsYieldsToAnOrdinaryHost(t *testing.T) {
	app := flatte.App[struct{}]{
		State: new(struct{}),
		View:  func(*struct{}, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	for _, controlsFirst := range []bool{true, false} {
		ran := false
		hostOption := optionTestHostOption{host: optionTestHost{ran: &ran}}
		var options []flatte.Option
		if controlsFirst {
			options = []flatte.Option{media.Controls{}, hostOption}
		} else {
			options = []flatte.Option{hostOption, media.Controls{}}
		}
		if err := flatte.Run(context.Background(), app, options...); err != nil {
			t.Fatalf("Run with fallback and ordinary host: %v", err)
		}
		if !ran {
			t.Fatal("ordinary host did not run")
		}
	}
}

func TestRunRejectsTwoWindowHosts(t *testing.T) {
	if !Supported() {
		t.Skip("native host options are not installed on this platform")
	}
	app := flatte.App[struct{}]{
		State: new(struct{}),
		View:  func(*struct{}, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	if err := flatte.Run(context.Background(), app, Options{}, Options{}); !errors.Is(err, flatte.ErrMultipleHosts) {
		t.Fatalf("two window options error = %v, want ErrMultipleHosts", err)
	}
}
