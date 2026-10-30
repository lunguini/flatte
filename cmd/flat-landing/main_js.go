//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/lunguini/flatte"
)

func main() {
	state := NewState()
	if tab, ok := tabFromHash(); ok {
		state.phase = phaseShell
		state.active = tab
	}

	lb := newLeaderboard(state)
	app := newApp(state)
	app.Init = func(_ *State, fx flatte.Effects[State]) {
		if err := flatte.Every(fx, "host-tick", tickInterval, func(s *State, now time.Time) {
			Tick(s, now)
			lb.afterFrame(s)
		}); err != nil {
			panic(err)
		}
	}
	baseHandle := app.Handle
	app.Handle = func(s *State, ev flatte.Event, fx flatte.Effects[State]) {
		before := s.active
		baseHandle(s, ev, fx)
		lb.afterFrame(s)
		if s.active != before {
			syncHash(s)
		}
	}

	var hashchange js.Func
	hashRegistered := false
	baseInit := app.Init
	app.Init = func(s *State, fx flatte.Effects[State]) {
		baseInit(s, fx)
		hashchange = js.FuncOf(func(js.Value, []js.Value) any {
			if tab, ok := tabFromHash(); ok {
				fx.Send(flatte.Named("landing.hash", func(s *State) {
					if tab == s.active {
						return
					}
					s.phase = phaseShell
					s.active = tab
					s.adopt()
					lb.afterFrame(s)
				}))
			}
			return nil
		})
		js.Global().Call("addEventListener", "hashchange", hashchange)
		hashRegistered = true
	}

	lb.afterFrame(state)
	// The page never quits: Ctrl+C is a copy gesture in a browser, not an exit.
	err := runApp(app, flatte.WithoutDefaultQuit())
	if hashRegistered {
		js.Global().Call("removeEventListener", "hashchange", hashchange)
		hashchange.Release()
	}
	exitOnError(err)
}

func tabFromHash() (landingTab, bool) {
	hash := strings.TrimPrefix(js.Global().Get("location").Get("hash").String(), "#")
	for i, id := range tabIDs {
		if id == hash {
			return landingTab(i), true
		}
	}
	return 0, false
}

func syncHash(s *State) {
	if int(s.active) >= len(tabIDs) {
		return
	}
	id := tabIDs[s.active]
	loc := js.Global().Get("location")
	if strings.TrimPrefix(loc.Get("hash").String(), "#") == id {
		return
	}
	js.Global().Get("history").Call("replaceState", js.Null(), "", "#"+id)
}
