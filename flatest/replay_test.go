package flatest

import (
	"context"
	"testing"

	"github.com/lunguini/flat"
)

type adder struct{ n int }

// adderApp folds an async increment named "inc" on each '+'.
func adderApp() flat.App[adder] {
	return flat.App[adder]{
		State: &adder{},
		Handle: func(s *adder, ev flat.Event, fx flat.Effects[adder]) {
			if k, ok := ev.(flat.KeyEvent); ok && k.Rune == '+' {
				flat.Go(fx, "inc",
					func(context.Context) (int, error) { return 1, nil },
					func(s *adder, v int, _ error) { s.n += v })
			}
		},
		View: func(s *adder, ctx flat.RenderContext) flat.Frame {
			return flat.Frame{Content: "n"}
		},
	}
}

func TestRecorderCapturesEventAndUpdateStream(t *testing.T) {
	rec := &Recorder{}
	app := adderApp()
	app.Tracer = rec

	d := Start(app, 40)
	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'})
	d.Settle()

	// Expect: an initial ResizeEvent, the KeyEvent, then the "inc" update.
	if names := rec.Updates(); len(names) != 1 || names[0] != "inc" {
		t.Fatalf("recorded updates = %v, want [inc]", names)
	}
	var sawKey bool
	for _, s := range rec.Steps {
		if k, ok := s.Event.(flat.KeyEvent); ok && k.Rune == '+' {
			sawKey = true
		}
	}
	if !sawKey {
		t.Fatalf("recorder did not capture the '+' key event: %#v", rec.Steps)
	}
}

func TestReplayReproducesUpdateStream(t *testing.T) {
	rec := &Recorder{}
	app := adderApp()
	app.Tracer = rec

	d := Start(app, 40)
	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'})
	d.Settle()
	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'})
	d.Settle()

	replayed := Replay(adderApp(), 40, rec)

	got, want := replayed.Updates(), rec.Updates()
	if len(got) != len(want) {
		t.Fatalf("replayed updates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("replayed update[%d] = %q, want %q (full: %v vs %v)", i, got[i], want[i], got, want)
		}
	}
	if want := []string{"inc", "inc"}; len(rec.Updates()) != 2 {
		t.Fatalf("expected the recording itself to have %v, got %v", want, rec.Updates())
	}
}
