package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/sessionaccess"
)

func TestQuitRequestedStopsWithoutTheOption(t *testing.T) {
	s := &testState{content: "x"}
	h, b := newTestHost(t, s)
	h.start(100, 50)
	h.quitRequested()
	if b.stops != 1 {
		t.Fatalf("stops = %d, want 1", b.stops)
	}
	for _, ev := range s.events {
		if _, ok := ev.(flatte.QuitRequest); ok {
			t.Fatal("app got a QuitRequest it did not ask for")
		}
	}
}

func TestQuitRequestedAsksTheAppWithTheOption(t *testing.T) {
	quit := false
	s := &testState{content: "x"}
	app := testApp(s)
	handle := app.Handle
	app.Handle = func(st *testState, ev flatte.Event, fx flatte.Effects[testState]) {
		handle(st, ev, fx)
		if _, ok := ev.(flatte.QuitRequest); ok && quit {
			fx.Quit()
		}
	}
	h, b := newTestHostWithApp(t, app)
	sessionaccess.SetQuitRequests(h.session, true)
	h.start(100, 50)

	h.quitRequested() // declined: the app keeps running
	if b.stops != 0 {
		t.Fatalf("declined request stopped the host (stops = %d)", b.stops)
	}
	got := 0
	for _, ev := range s.events {
		if _, ok := ev.(flatte.QuitRequest); ok {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("QuitRequests delivered = %d, want 1", got)
	}
	quit = true
	h.quitRequested() // accepted
	if b.stops != 1 {
		t.Fatalf("accepted request: stops = %d, want 1", b.stops)
	}
}
