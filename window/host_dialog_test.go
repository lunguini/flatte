package window

import (
	"context"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type fakeDialogs struct {
	shown  []flatte.Dialog
	answer flatte.DialogResult
}

func (f *fakeDialogs) run(d flatte.Dialog) flatte.DialogResult {
	f.shown = append(f.shown, d)
	return f.answer
}

type dialogState struct {
	paths  []string
	button int
	drops  []flatte.DropEvent
}

func dialogHost(t *testing.T) (*host[dialogState], *dialogState, *fakeBackend) {
	s := &dialogState{}
	app := flatte.App[dialogState]{
		State: s,
		Handle: func(s *dialogState, ev flatte.Event, fx flatte.Effects[dialogState]) {
			switch e := ev.(type) {
			case flatte.KeyEvent:
				switch e.Rune {
				case 'o':
					flatte.OpenFile(fx, "open", flatte.OpenDialog{Title: "Open"}, func(s *dialogState, p []string, _ error) { s.paths = p })
				case 'a':
					flatte.Alert(fx, "alert", flatte.AlertDialog{Title: "Sure?"}, func(s *dialogState, b int) { s.button = b })
				}
			case flatte.DropEvent:
				s.drops = append(s.drops, e)
			}
		},
		View: func(*dialogState, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Col{Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "top", H: layout.Fixed(2)}, Text: "top"},
				layout.Label{NodeBase: layout.NodeBase{ID: "zone", H: layout.Grow(1)}, Text: "drop here"},
			}}}
		},
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() {}, func() {})
	t.Cleanup(h.cancel)
	h.start(400, 200)
	h.render()
	return h, s, b
}

func TestHostRunsDialogsAfterTheHandler(t *testing.T) {
	h, s, b := dialogHost(t)
	d := &fakeDialogs{answer: flatte.DialogResult{Paths: []string{"/tmp/a.txt"}}}
	h.dialogs = d
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'o'})
	if len(d.shown) != 0 {
		t.Fatal("the dialog ran inside the handler")
	}
	b.drain(t, 1) // the posted dialog runs, answers, and queues the fold
	if len(d.shown) != 1 || d.shown[0].Open == nil || d.shown[0].Open.Title != "Open" {
		t.Fatalf("shown %+v", d.shown)
	}
	b.drain(t, 1) // the update arrives through the host's queue
	if len(s.paths) != 1 || s.paths[0] != "/tmp/a.txt" {
		t.Fatalf("paths %q", s.paths)
	}
	// Without a backend, dialogs answer as cancelled.
	h.dialogs = nil
	s.button = 7
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'a'})
	b.drain(t, 2)
	if s.button != -1 {
		t.Fatalf("button %d", s.button)
	}
}

func TestHostDropIsHitTested(t *testing.T) {
	h, s, _ := dialogHost(t)
	zone := h.scene.Rects["zone"]
	h.drop(zone.X+20, zone.Y+20, []string{"/a", "/b"}, "")
	h.drop(-5, -5, nil, "text")
	if len(s.drops) != 2 {
		t.Fatalf("drops %+v", s.drops)
	}
	d := s.drops[0]
	if d.Target != "zone" || len(d.Paths) != 2 || d.X-d.TargetRect.X != 2 || d.Y-d.TargetRect.Y != 1 {
		t.Errorf("drop %+v", d)
	}
	if s.drops[1].Target != "" || s.drops[1].Text != "text" {
		t.Errorf("outside drop %+v", s.drops[1])
	}
}
