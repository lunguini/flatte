package flatest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type counter struct{ n int }

func counterApp() flatte.App[counter] {
	return flatte.App[counter]{
		State: &counter{},
		Handle: func(s *counter, ev flatte.Event, fx flatte.Effects[counter]) {
			if k, ok := ev.(flatte.KeyEvent); ok && k.Rune == '+' {
				s.n++
			}
		},
		View: func(s *counter, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "n=" + strconv.Itoa(s.n)}
		},
	}
}

func TestDriverSendUpdatesStateAndFrame(t *testing.T) {
	d := Start(counterApp(), 40)
	if got := d.Frame().Content; got != "n=0" {
		t.Fatalf("initial frame = %q, want n=0", got)
	}
	f := d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '+'})
	if f.Content != "n=1" {
		t.Fatalf("frame after + = %q, want n=1", f.Content)
	}
	if d.State().n != 1 {
		t.Fatalf("state.n = %d, want 1", d.State().n)
	}
}

func TestDriverQuitReflectsEffectQuit(t *testing.T) {
	app := counterApp()
	app.Handle = func(s *counter, ev flatte.Event, fx flatte.Effects[counter]) {
		if k, ok := ev.(flatte.KeyEvent); ok && k.Rune == 'q' {
			fx.Quit()
		}
	}
	d := Start(app, 40)
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'})
	if !d.Quit() {
		t.Fatal("Driver.Quit() = false after fx.Quit()")
	}
}

func TestDriverDeliversInitialResize(t *testing.T) {
	var sawWidth int
	app := counterApp()
	app.Handle = func(s *counter, ev flatte.Event, fx flatte.Effects[counter]) {
		if r, ok := ev.(flatte.ResizeEvent); ok {
			sawWidth = r.Width
		}
	}
	Start(app, 55)
	if sawWidth != 55 {
		t.Fatalf("initial ResizeEvent width = %d, want 55", sawWidth)
	}
}

func TestDriverSharedSessionKeepsSecondaryWindowFrames(t *testing.T) {
	type state struct{ focused string }
	s := &state{}
	d := Start(flatte.App[state]{
		State: s,
		Handle: func(s *state, ev flatte.Event, _ flatte.Effects[state]) {
			if w, ok := ev.(flatte.WindowEvent); ok && w.Kind == flatte.WindowFocused {
				s.focused = w.Window
			}
		},
		View: func(_ *state, ctx flatte.RenderContext) flatte.Frame {
			if ctx.Window != "" {
				return flatte.Frame{Content: "secondary"}
			}
			return flatte.Frame{Content: "main"}
		},
		Windows: func(*state) []flatte.Window {
			return []flatte.Window{{ID: "secondary", Width: 20, Height: 5}}
		},
	}, 40)
	if s.focused != "secondary" {
		t.Fatalf("focused window = %q, want secondary", s.focused)
	}
	frame, ok := d.WindowFrame("secondary")
	if !ok || frame.Content != "secondary" {
		t.Fatalf("WindowFrame = (%+v, %t), want secondary frame", frame, ok)
	}
}

func TestDriverFrameRunsCoreLayoutSettlement(t *testing.T) {
	type state struct {
		show   bool
		layout bool
	}
	s := &state{}
	d := Start(flatte.App[state]{
		State: s,
		Handle: func(s *state, ev flatte.Event, _ flatte.Effects[state]) {
			if _, ok := ev.(flatte.LayoutEvent); ok {
				s.layout = true
			}
		},
		View: func(s *state, _ flatte.RenderContext) flatte.Frame {
			if !s.show {
				return flatte.Frame{Content: "hidden"}
			}
			text := "layout-pending"
			if s.layout {
				text = "layout-delivered"
			}
			return flatte.Frame{Root: layout.Label{NodeBase: layout.NodeBase{ID: "label"}, Text: text}}
		},
	}, 40)
	s.show = true
	frame := d.Frame()
	if !s.layout || !strings.Contains(frame.Content, "layout-delivered") {
		t.Fatalf("Frame = %q, layout delivered=%t; want settled layout", frame.Content, s.layout)
	}
}

func TestDriverResizeUpdatesHandleAndViewDimensions(t *testing.T) {
	var handleW, handleH, viewW, viewH int
	app := flatte.App[counter]{
		State: &counter{},
		Handle: func(_ *counter, ev flatte.Event, _ flatte.Effects[counter]) {
			if r, ok := ev.(flatte.ResizeEvent); ok {
				handleW, handleH = r.Width, r.Height
			}
		},
		View: func(_ *counter, ctx flatte.RenderContext) flatte.Frame {
			viewW, viewH = ctx.Width, ctx.Height
			return flatte.Frame{Content: "x"}
		},
	}
	d := StartWith(app, Options{Width: 30, Height: 12, Graphical: true, Capabilities: flatte.Capabilities{Drop: true}})
	if !d.fx.Capabilities().Drop {
		t.Fatal("Effects do not report the driver's capabilities")
	}
	d.Resize(50, 18)
	if handleW != 50 || handleH != 18 || viewW != 50 || viewH != 18 {
		t.Fatalf("Handle=%dx%d View=%dx%d; want all dimensions 50x18", handleW, handleH, viewW, viewH)
	}
}

func TestDriverRecordsNotifications(t *testing.T) {
	type st struct{}
	app := flatte.App[st]{
		State: &st{},
		Handle: func(s *st, ev flatte.Event, fx flatte.Effects[st]) {
			if k, ok := ev.(flatte.KeyEvent); ok && k.Rune == 'n' {
				fx.Notify(flatte.Notification{Title: "Saved", Body: "notes.txt"})
			}
		},
		View: func(*st, flatte.RenderContext) flatte.Frame { return flatte.Frame{Content: "x"} },
	}
	d := Start(app, 20)
	if len(d.Notifications()) != 0 {
		t.Fatal("notified before any key")
	}
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'n'})
	if got := d.Notifications(); len(got) != 1 || got[0] != (flatte.Notification{Title: "Saved", Body: "notes.txt"}) {
		t.Fatalf("notifications %v", got)
	}
}
