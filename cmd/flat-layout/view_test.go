package main

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func newApp() flatte.App[State] {
	return flatte.App[State]{State: &State{}, Handle: Handle, View: View}
}

func key(r rune) flatte.KeyEvent { return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r} }

// The goldens were captured from the pre-Frame.Root View, which solved the
// tree itself at the resized width×height. The runtime-composed frame must be
// identical.
func TestViewGolden(t *testing.T) {
	d := flatest.Start(newApp(), 80)
	d.Send(key('j'))
	d.Send(key('j'))
	flatest.AssertGoldenFrame(t, "testdata/view.golden", d.Send(key('j')))
	flatest.AssertGoldenFrame(t, "testdata/view-modal.golden", d.Send(key('m')))
}

func TestMouseTargetsFromRoot(t *testing.T) {
	d := flatest.Start(flatte.App[State]{
		State: &State{},
		Handle: func(s *State, ev flatte.Event, fx flatte.Effects[State]) {
			if m, ok := ev.(flatte.MouseEvent); ok && m.Target == "sidebar" {
				s.Cursor++
			}
			Handle(s, ev, fx)
		},
		View: View,
	}, 80)
	d.Send(flatte.MouseEvent{X: 2, Y: 5, Action: flatte.MousePress})  // sidebar
	d.Send(flatte.MouseEvent{X: 40, Y: 5, Action: flatte.MousePress}) // content
	if got := d.State().Cursor; got != 1 {
		t.Fatalf("Cursor = %d, want 1 (only the sidebar click counts)", got)
	}
}
