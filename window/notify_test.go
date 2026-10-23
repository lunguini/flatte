package window

import (
	"testing"

	"github.com/lunguini/flatte"
)

type fakeNotifier struct{ posted []flatte.Notification }

func (f *fakeNotifier) notify(n flatte.Notification) { f.posted = append(f.posted, n) }

func TestHostPostsNotifications(t *testing.T) {
	s := &testState{content: "x"}
	h, _ := newTestHost(t, s)
	h.start(80, 48)
	h.fx.Notify(flatte.Notification{Body: "dropped: no notifier"})
	f := &fakeNotifier{}
	h.notes = f
	h.fx.Notify(flatte.Notification{Title: "Now playing", Body: "Glass"})
	if len(f.posted) != 1 || f.posted[0] != (flatte.Notification{Title: "Now playing", Body: "Glass"}) {
		t.Fatalf("posted %v", f.posted)
	}
}

func TestHostReportsActivationAsFocus(t *testing.T) {
	s := &testState{content: "x"}
	h, _ := newTestHost(t, s)
	h.start(80, 48)
	h.active = true
	s.events = nil
	h.activated(true) // unchanged: nothing
	h.activated(false)
	h.activated(false)
	h.activated(true)
	want := []flatte.Event{flatte.FocusEvent{Focused: false}, flatte.FocusEvent{Focused: true}}
	if len(s.events) != len(want) || s.events[0] != want[0] || s.events[1] != want[1] {
		t.Fatalf("events %+v, want %+v", s.events, want)
	}
}
