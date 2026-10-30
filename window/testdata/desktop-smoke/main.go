// Native lifecycle smoke: run manually in a macOS login session. It neither
// captures another application nor touches the clipboard or terminal sessions.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
)

type state struct {
	tick   int
	exited bool
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s := &state{}
	app := flatte.App[state]{State: s,
		Init: func(s *state, fx flatte.Effects[state]) {
			flatte.Every(fx, "native-lifecycle", 80*time.Millisecond, func(s *state, _ time.Time) {
				s.tick++
				switch s.tick {
				case 1:
					fx.Desktop(flatte.DesktopCommand{Kind: "show"})
				case 2:
					fx.Desktop(flatte.DesktopCommand{Kind: "hide"})
				case 3:
					fx.Quit()
				}
			})
		},
		View: func(s *state, _ flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Title: "Flatte native lifecycle smoke", Root: layout.Label{Text: "Native semantic window — harmless smoke"}}
		},
		OnExit: func(s *state) { s.exited = true },
	}
	shortcut := flatte.KeyEvent{Key: flatte.KeyF12, Mod: flatte.ModCtrl | flatte.ModAlt}
	err := window.Run(ctx, app, window.Options{Width: 52, Height: 12, Desktop: &flatte.Desktop{Title: "Flatte smoke", Shortcut: &shortcut}})
	if err != nil || s.tick != 3 || !s.exited {
		fmt.Fprintf(os.Stderr, "FAIL native lifecycle: %v, ticks=%d, exit=%v\n", err, s.tick, s.exited)
		os.Exit(1)
	}
	fmt.Println("PASS native semantic desktop host: hidden start, show, hide, quit, cleanup")
}
