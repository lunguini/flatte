package window

import (
	"context"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type fakeWindows struct {
	opened  []flatte.Window
	surface map[string]ui
	closed  []string
	redraws map[string]int
}

func (f *fakeWindows) openWindow(w flatte.Window, s ui) func() {
	f.opened = append(f.opened, w)
	f.surface[w.ID] = s
	return func() { f.redraws[w.ID]++ }
}

func (f *fakeWindows) closeWindow(id string) { f.closed = append(f.closed, id) }

type multiState struct {
	stats  bool
	events []flatte.Event
	views  []string
}

func multiHost(t *testing.T) (*host[multiState], *multiState, *fakeWindows) {
	s := &multiState{}
	app := flatte.App[multiState]{
		State: s,
		Handle: func(s *multiState, ev flatte.Event, _ flatte.Effects[multiState]) {
			s.events = append(s.events, ev)
			if w, ok := ev.(flatte.WindowEvent); ok && w.Kind == flatte.WindowCloseRequest {
				s.stats = false
			}
		},
		View: func(s *multiState, ctx flatte.RenderContext) flatte.Frame {
			s.views = append(s.views, ctx.Window)
			if ctx.Window == "stats" {
				return flatte.Frame{Title: "Statistics", Root: layout.Col{Children: []layout.Node{
					layout.Label{NodeBase: layout.NodeBase{ID: "words"}, Text: "words: 3", Selectable: true},
				}}}
			}
			return flatte.Frame{Title: "Main", Root: layout.Label{NodeBase: layout.NodeBase{ID: "main"}, Text: "main"}}
		},
		Windows: func(s *multiState) []flatte.Window {
			if !s.stats {
				return nil
			}
			return []flatte.Window{{ID: "stats", Width: 30, Height: 8}}
		},
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() {}, func() {})
	t.Cleanup(h.cancel)
	f := &fakeWindows{surface: map[string]ui{}, redraws: map[string]int{}}
	h.windows, h.text, h.clip = f, &fakeText{}, &fakeClip{}
	h.start(400, 200)
	h.render()
	return h, s, f
}

func TestHostOpensAndClosesDeclaredWindows(t *testing.T) {
	h, s, f := multiHost(t)
	if len(f.opened) != 0 {
		t.Fatal("a window opened undeclared")
	}
	s.stats = true
	h.handle(flatte.KeyEvent{Key: flatte.KeyEnter}) // any change syncs
	if len(f.opened) != 1 || f.opened[0].ID != "stats" || f.opened[0].Width != 30 {
		t.Fatalf("opened %+v", f.opened)
	}
	h.handle(flatte.KeyEvent{Key: flatte.KeyEnter})
	if len(f.opened) != 1 {
		t.Fatal("an open window was opened again")
	}
	if f.redraws["stats"] == 0 {
		t.Error("changes do not redraw the secondary window")
	}
	// Its surface draws the window's View at its own size, with no
	// ResizeEvent, and delivers its LayoutEvents tagged.
	w := f.surface["stats"]
	s.events, s.views = nil, nil
	w.resize(30*cellMetrics.Column, 8*cellMetrics.Line)
	w.render()
	if w.frameTitle() != "Statistics" || len(s.views) == 0 || s.views[0] != "stats" {
		t.Fatalf("title %q views %v", w.frameTitle(), s.views)
	}
	for _, ev := range s.events {
		if _, ok := ev.(flatte.ResizeEvent); ok {
			t.Fatal("a secondary window delivered a ResizeEvent")
		}
	}
	if l, ok := s.events[len(s.events)-1].(flatte.LayoutEvent); !ok || l.Window != "stats" || l.Rects["words"].W != 30 {
		t.Fatalf("layout %+v", s.events)
	}
	// Mouse events in it carry the window.
	s.events = nil
	w.mouse(5, 5, flatte.MouseLeft, flatte.MousePress, 0)
	if m, ok := s.events[0].(flatte.MouseEvent); !ok || m.Window != "stats" || m.Target != "words" {
		t.Fatalf("mouse %+v", s.events)
	}
	// Focus: reported when it changes.
	s.events = nil
	h.focused("stats")
	h.focused("stats")
	h.focused("nope") // not open: ignored
	if len(s.events) != 1 || s.events[0] != (flatte.WindowEvent{Window: "stats", Kind: flatte.WindowFocused}) {
		t.Fatalf("focus %+v", s.events)
	}
	// ⌘C from the menu copies the focused window's selection.
	w.mouse(0, 1, flatte.MouseLeft, flatte.MousePress, 0)
	w.mouse(5, 1, flatte.MouseLeft, flatte.MouseMotion, 0)
	h.bar = []flatte.Menu{{Items: []flatte.MenuItem{{ID: "copy", Keys: []flatte.KeyEvent{copyKey}}}}}
	h.menuChosen("copy")
	if got := h.clip.(*fakeClip).text; got != "words" {
		t.Errorf("copied %q from the focused window", got)
	}
	// The close button asks; the app stops declaring it; it closes.
	s.events = nil
	h.closeRequested("stats")
	if len(f.closed) != 1 || f.closed[0] != "stats" || len(h.open) != 0 {
		t.Fatalf("closed %v open %d", f.closed, len(h.open))
	}
	if s.events[0] != (flatte.WindowEvent{Window: "stats", Kind: flatte.WindowCloseRequest}) {
		t.Fatalf("events %+v", s.events)
	}
	h.closeRequested("stats") // no longer open: nothing
	if len(s.events) != 1 {
		t.Fatal("a close request for a closed window was delivered")
	}
}

func TestHostKeepsAWindowTheAppKeeps(t *testing.T) {
	h, s, f := multiHost(t)
	s.stats = true
	h.handle(flatte.KeyEvent{})
	// An app that keeps declaring the window keeps it open.
	app := h.app
	app.Handle = func(s *multiState, ev flatte.Event, _ flatte.Effects[multiState]) { s.events = append(s.events, ev) }
	h.app = app
	h.closeRequested("stats")
	if len(f.closed) != 0 || len(h.open) != 1 {
		t.Fatal("the window closed though the app kept it")
	}
	h.close()
	if len(f.closed) != 1 {
		t.Fatal("closing the host left the window open")
	}
}
