package flatest

import (
	"testing"

	"github.com/lunguini/flatte"
)

func TestOnExitCalledOnQuit(t *testing.T) {
	type onExitState struct {
		N int
	}
	called := false
	state := &onExitState{N: 0}
	app := flatte.App[onExitState]{
		State: state,
		Handle: func(s *onExitState, ev flatte.Event, fx flatte.Effects[onExitState]) {
			if _, ok := ev.(flatte.KeyEvent); ok {
				fx.Quit()
			}
		},
		View: func(s *onExitState, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "x"}
		},
		OnExit: func(s *onExitState) {
			called = true
			s.N = 42
		},
	}

	drv := Start(app, 40)
	drv.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'})
	drv.Settle()
	drv.Close()

	if !called {
		t.Fatalf("OnExit was not called on quit")
	}
	if state.N != 42 {
		t.Fatalf("OnExit should have set N=42, got %d", state.N)
	}
}

func TestOnExitCalledOnContextCancel(t *testing.T) {
	type cancelState struct{}
	called := false
	state := &cancelState{}

	app := flatte.App[cancelState]{
		State: state,
		Handle: func(s *cancelState, ev flatte.Event, fx flatte.Effects[cancelState]) {
			if _, ok := ev.(flatte.KeyEvent); ok {
				fx.Quit()
			}
		},
		View: func(s *cancelState, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "x"}
		},
		OnExit: func(s *cancelState) {
			called = true
		},
	}

	drv := Start(app, 40)
	drv.Send(flatte.KeyEvent{Key: flatte.KeyEscape})
	drv.Settle()
	drv.Close()

	if !called {
		t.Fatalf("OnExit was not called")
	}
}
