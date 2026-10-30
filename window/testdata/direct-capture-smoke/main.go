// Native direct-capture gate: a Services event must not activate a window.
package main

import (
	"context"
	"fmt"
	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
	"os"
	"time"
)

type state struct {
	tick     int
	received bool
	failure  string
}

func sel(s string) objc.SEL { return objc.RegisterName(s) }
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s := &state{}
	app := flatte.App[state]{State: s, Init: func(s *state, fx flatte.Effects[state]) {
		_ = flatte.Every(fx, "direct-capture-smoke", 100*time.Millisecond, func(s *state, _ time.Time) {
			s.tick++
			nsapp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
			if s.tick == 1 {
				board := objc.ID(objc.GetClass("NSPasteboard")).Send(sel("pasteboardWithUniqueName"))
				defer board.Send(sel("releaseGlobally"))
				literal := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), "Harmless direct-capture test")
				typ := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), "public.utf8-plain-text")
				board.Send(sel("clearContents"))
				board.Send(sel("setString:forType:"), literal, typ)
				var failure objc.ID
				nsapp.Send(sel("servicesProvider")).Send(sel("receiveText:userData:error:"), board, objc.ID(0), &failure)
				if failure != 0 {
					s.failure = "Services callback failed"
				}
			}
			if s.tick == 2 {
				if !s.received {
					s.failure = "Services capture not delivered"
				}
				wins := nsapp.Send(sel("windows"))
				for i := uint(0); i < objc.Send[uint](wins, sel("count")); i++ {
					win := wins.Send(sel("objectAtIndex:"), i)
					title := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), "Direct capture test")
					if objc.Send[bool](win.Send(sel("title")), sel("isEqualToString:"), title) && objc.Send[bool](win, sel("isVisible")) {
						s.failure = "Capture unexpectedly showed a window"
					}
				}
				fx.Quit()
			}
		})
	}, Handle: func(s *state, e flatte.Event, _ flatte.Effects[state]) {
		if ev, ok := e.(flatte.DesktopEvent); ok && ev.Source == "service" {
			s.received = ev.Text == "Harmless direct-capture test" && ev.Error == ""
		}
	}, View: func(*state, flatte.RenderContext) flatte.Frame {
		return flatte.Frame{Root: layout.Label{Text: "Direct capture smoke"}}
	}}
	err := window.Run(ctx, app, window.Options{Width: 40, Height: 10, Desktop: &flatte.Desktop{Title: "Direct capture test"}})
	if err != nil || s.failure != "" || s.tick != 2 {
		fmt.Fprintln(os.Stderr, "FAIL", err, s.failure)
		os.Exit(1)
	}
	fmt.Println("PASS Services capture delivered without showing or activating a window")
}
