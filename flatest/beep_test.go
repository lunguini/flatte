package flatest_test

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestDriverCountsBeeps(t *testing.T) {
	type st struct{}
	d := flatest.Start(flatte.App[st]{
		State: &st{},
		Handle: func(_ *st, ev flatte.Event, fx flatte.Effects[st]) {
			if _, ok := ev.(flatte.KeyEvent); ok {
				fx.Beep()
			}
		},
		View: func(*st, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}, 10)
	defer d.Close()
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'x'})
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'y'})
	if d.Beeps() != 2 {
		t.Fatalf("beeps = %d", d.Beeps())
	}
}
