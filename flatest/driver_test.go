package flatest

import (
	"strconv"
	"testing"

	"github.com/lunguini/flat"
)

type counter struct{ n int }

func counterApp() flat.App[counter] {
	return flat.App[counter]{
		State: &counter{},
		Handle: func(s *counter, ev flat.Event, fx flat.Effects[counter]) {
			if k, ok := ev.(flat.KeyEvent); ok && k.Rune == '+' {
				s.n++
			}
		},
		View: func(s *counter, ctx flat.RenderContext) flat.Frame {
			return flat.Frame{Content: "n=" + strconv.Itoa(s.n)}
		},
	}
}

func TestDriverSendUpdatesStateAndFrame(t *testing.T) {
	d := Start(counterApp(), 40)
	if got := d.Frame().Content; got != "n=0" {
		t.Fatalf("initial frame = %q, want n=0", got)
	}
	f := d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'})
	if f.Content != "n=1" {
		t.Fatalf("frame after + = %q, want n=1", f.Content)
	}
	if d.State().n != 1 {
		t.Fatalf("state.n = %d, want 1", d.State().n)
	}
}

func TestDriverQuitReflectsEffectQuit(t *testing.T) {
	app := counterApp()
	app.Handle = func(s *counter, ev flat.Event, fx flat.Effects[counter]) {
		if k, ok := ev.(flat.KeyEvent); ok && k.Rune == 'q' {
			fx.Quit()
		}
	}
	d := Start(app, 40)
	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'})
	if !d.Quit() {
		t.Fatal("Driver.Quit() = false after fx.Quit()")
	}
}

func TestDriverDeliversInitialResize(t *testing.T) {
	var sawWidth int
	app := counterApp()
	app.Handle = func(s *counter, ev flat.Event, fx flat.Effects[counter]) {
		if r, ok := ev.(flat.ResizeEvent); ok {
			sawWidth = r.Width
		}
	}
	Start(app, 55)
	if sawWidth != 55 {
		t.Fatalf("initial ResizeEvent width = %d, want 55", sawWidth)
	}
}
