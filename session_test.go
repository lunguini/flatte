package flatte

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type sessionRunState struct {
	steps        []string
	async        string
	capabilities Capabilities
	exited       int
}

type sessionTestHost struct{}

func (h sessionTestHost) Run(_ context.Context, session *Session) error {
	want := Capabilities{Alert: true, Clipboard: true, Mouse: true, Quit: true}
	session.UseHostServices(HostServices{Capabilities: want})
	session.Start(ResizeEvent{Width: 40, Height: 12})
	select {
	case <-session.Updates():
	case <-time.After(time.Second):
		return fmt.Errorf("async update was not queued")
	}
	if n := session.ApplyPendingUpdates(); n != 1 {
		return fmt.Errorf("ApplyPendingUpdates applied %d updates, want 1", n)
	}
	if got := session.Capabilities(); got != want {
		return fmt.Errorf("session capabilities = %+v, want %+v", got, want)
	}
	session.Handle(KeyEvent{Key: KeyCharacter, Rune: 'q'})
	if !session.Quitting() {
		return fmt.Errorf("Quit did not reach the session")
	}
	frame := session.View(RenderContext{Width: 40, Height: 12, Graphical: true, Capabilities: session.Capabilities()})
	if frame.Content != "complete" {
		return fmt.Errorf("View content = %q, want complete", frame.Content)
	}
	return nil
}

type sessionTestOption struct{ host Host }

func (o sessionTestOption) ConfigureRun(config *RunConfig) { config.UseHost(o.host) }

func TestRunSelectsHostAndOwnsSessionLifecycle(t *testing.T) {
	s := &sessionRunState{}
	var rendered Capabilities
	app := App[sessionRunState]{
		State: s,
		Init: func(s *sessionRunState, fx Effects[sessionRunState]) {
			s.steps = append(s.steps, "init")
			Go(fx, "session-test", func(context.Context) (string, error) { return "complete", nil }, func(s *sessionRunState, value string, _ error) {
				s.async = value
			})
		},
		Handle: func(s *sessionRunState, ev Event, fx Effects[sessionRunState]) {
			switch ev := ev.(type) {
			case ResizeEvent:
				s.steps = append(s.steps, "resize")
				s.capabilities = fx.Capabilities()
			case KeyEvent:
				if ev.Rune == 'q' {
					fx.Quit()
				}
			}
		},
		View: func(s *sessionRunState, ctx RenderContext) Frame {
			rendered = ctx.Capabilities
			return Frame{Content: s.async}
		},
		OnExit: func(*sessionRunState) { s.exited++ },
	}
	if err := Run(context.Background(), app, WithMouse(MouseModeCellMotion), sessionTestOption{host: sessionTestHost{}}); err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprint(s.steps), "[init resize]"; got != want {
		t.Fatalf("startup order = %s, want %s", got, want)
	}
	if s.async != "complete" || !s.capabilities.Alert || !rendered.Alert || !rendered.Mouse || !rendered.Quit {
		t.Fatalf("session result async=%q effects=%+v render=%+v", s.async, s.capabilities, rendered)
	}
	if s.exited != 1 {
		t.Fatalf("OnExit called %d times, want once", s.exited)
	}
}

func TestSessionStepOwnsCellInputUpdatesFramesAndQuit(t *testing.T) {
	type state struct {
		menuEvents int
		updates    int
		ctrlC      int
	}
	s := &state{}
	app := App[state]{
		State: s,
		Menus: func(*state) []Menu {
			return []Menu{{Items: []MenuItem{{ID: "increment", Keys: []KeyEvent{{Key: KeyCharacter, Rune: 'x'}}}}}}
		},
		Handle: func(s *state, ev Event, _ Effects[state]) {
			switch ev := ev.(type) {
			case MenuEvent:
				if ev.ID == "increment" {
					s.menuEvents++
				}
			case KeyEvent:
				if ev.Key == KeyCtrlC {
					s.ctrlC++
				}
			}
		},
		View: func(s *state, _ RenderContext) Frame {
			return Frame{Content: fmt.Sprintf("menu=%d updates=%d", s.menuEvents, s.updates)}
		},
	}
	session := NewSession(context.Background(), app)
	session.UseHostServices(HostServices{CellSurface: true, DefaultQuit: true})
	session.Start(ResizeEvent{Width: 40, Height: 12})
	renderContext := RenderContext{Width: 40, Height: 12}
	initial := session.Step(StepInput{FrameDue: true, RenderContext: renderContext})
	if !initial.Paint || initial.Frame.Content != "menu=0 updates=0" {
		t.Fatalf("initial step = %+v, want initial frame", initial)
	}

	stepped := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyCharacter, Rune: 'x'}}, RenderContext: renderContext})
	if !stepped.Paint || stepped.Frame.Content != "menu=1 updates=0" || s.menuEvents != 1 {
		t.Fatalf("menu step = %+v, menuEvents=%d", stepped, s.menuEvents)
	}
	session.enqueue(func() {
		s.updates++
		session.enqueue(func() { s.updates++ })
	})
	session.enqueue(func() { s.updates++ })
	updated := session.Step(StepInput{UpdatesPending: true, RenderContext: renderContext})
	if !updated.Paint || updated.Frame.Content != "menu=1 updates=2" {
		t.Fatalf("update step = %+v, state=%+v", updated, *s)
	}
	settled := session.Step(StepInput{UpdatesPending: true, RenderContext: renderContext})
	if !settled.Paint || settled.Frame.Content != "menu=1 updates=3" {
		t.Fatalf("follow-up update step = %+v, state=%+v", settled, *s)
	}
	quit := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyCtrlC}}, RenderContext: renderContext})
	if !quit.Quit || s.ctrlC != 0 {
		t.Fatalf("Ctrl+C step = %+v, ctrlC deliveries=%d", quit, s.ctrlC)
	}
}
