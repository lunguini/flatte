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
	started := session.Step(StepInput{Start: &ResizeEvent{Width: 40, Height: 12}})
	if started.Quit {
		return fmt.Errorf("session quit during startup")
	}
	select {
	case <-session.Updates():
	case <-time.After(time.Second):
		return fmt.Errorf("async update was not queued")
	}
	updated := session.Step(StepInput{UpdatesPending: true})
	if !updated.Dirty {
		return fmt.Errorf("update step did not report dirty state")
	}
	if got := session.State().Capabilities; got != want {
		return fmt.Errorf("session capabilities = %+v, want %+v", got, want)
	}
	frame := session.Step(StepInput{FrameDue: true, RenderContext: RenderContext{Width: 40, Height: 12, Graphical: true}})
	if frame.Frame.Content != "complete" {
		return fmt.Errorf("View content = %q, want complete", frame.Frame.Content)
	}
	quit := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyCharacter, Rune: 'q'}}})
	if !quit.Quit {
		return fmt.Errorf("Quit did not reach the session")
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
	session := newSession(context.Background(), app)
	session.UseHostServices(HostServices{CellSurface: true})
	session.useCellPolicy(sessionPolicy{defaultQuit: true})
	session.Step(StepInput{Start: &ResizeEvent{Width: 40, Height: 12}})
	renderContext := RenderContext{Width: 40, Height: 12}
	initial := session.Step(StepInput{FrameDue: true, RenderContext: renderContext})
	if !initial.Paint || initial.Frame.Content != "menu=0 updates=0" {
		t.Fatalf("initial step = %+v, want initial frame", initial)
	}

	stepped := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyCharacter, Rune: 'x'}}, FrameDue: true, RenderContext: renderContext})
	if !stepped.Paint || stepped.Frame.Content != "menu=1 updates=0" || s.menuEvents != 1 {
		t.Fatalf("menu step = %+v, menuEvents=%d", stepped, s.menuEvents)
	}
	session.enqueue(func() {
		s.updates++
		session.enqueue(func() { s.updates++ })
	})
	session.enqueue(func() { s.updates++ })
	updated := session.Step(StepInput{UpdatesPending: true, FrameDue: true, RenderContext: renderContext})
	if !updated.Paint || updated.Frame.Content != "menu=1 updates=2" {
		t.Fatalf("update step = %+v, state=%+v", updated, *s)
	}
	settled := session.Step(StepInput{UpdatesPending: true, FrameDue: true, RenderContext: renderContext})
	if !settled.Paint || settled.Frame.Content != "menu=1 updates=3" {
		t.Fatalf("follow-up update step = %+v, state=%+v", settled, *s)
	}
	select {
	case <-session.Updates():
		t.Fatal("update notification remained after the pending batch was applied")
	default:
	}
	quit := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyCtrlC}}, RenderContext: renderContext})
	if !quit.Quit || s.ctrlC != 0 {
		t.Fatalf("Ctrl+C step = %+v, ctrlC deliveries=%d", quit, s.ctrlC)
	}
}

func TestSessionStepDefersPaintUntilFrameDue(t *testing.T) {
	type state struct{ events int }
	s := &state{}
	session := NewSession(context.Background(), App[state]{
		State: s,
		Handle: func(s *state, ev Event, _ Effects[state]) {
			if _, ok := ev.(KeyEvent); ok {
				s.events++
			}
		},
		View: func(s *state, _ RenderContext) Frame { return Frame{Content: fmt.Sprintf("events=%d", s.events)} },
	})
	defer session.Cancel()
	session.UseHostServices(HostServices{CellSurface: true})
	session.Step(StepInput{Start: &ResizeEvent{Width: 40, Height: 12}})
	ctx := RenderContext{Width: 40, Height: 12}
	step := session.Step(StepInput{Events: []Event{KeyEvent{Key: KeyEnter}}, RenderContext: ctx})
	if step.Paint || !step.Dirty || s.events != 1 {
		t.Fatalf("event step = %+v, state=%+v; want dirty without painting", step, *s)
	}
	frame := session.Step(StepInput{FrameDue: true, RenderContext: ctx})
	if !frame.Paint || frame.Dirty || frame.Frame.Content != "events=1" {
		t.Fatalf("frame step = %+v, state=%+v", frame, *s)
	}
}

func TestHostStateIsAValueSnapshotAndTargetQueriesStayOnSession(t *testing.T) {
	type state struct {
		menus   []Menu
		windows []Window
		item    *NowPlaying
		context []MenuItem
	}
	s := &state{
		menus:   []Menu{{Title: "File", Items: []MenuItem{{ID: "save", Label: "Save", Keys: []KeyEvent{{Rune: 's'}}}}}},
		windows: []Window{{ID: "secondary"}},
		item:    &NowPlaying{Title: "Song"},
		context: []MenuItem{{ID: "open", Label: "Open"}},
	}
	session := NewSession(context.Background(), App[state]{
		State:      s,
		Menus:      func(s *state) []Menu { return s.menus },
		Windows:    func(s *state) []Window { return s.windows },
		NowPlaying: func(s *state) *NowPlaying { return s.item },
		ContextMenu: func(s *state, target string) []MenuItem {
			if target == "row" {
				return s.context
			}
			return nil
		},
		View: func(*state, RenderContext) Frame { return Frame{Content: "main"} },
	})
	defer session.Cancel()
	session.UseHostServices(HostServices{CellSurface: true})
	snapshot := session.State()
	if len(snapshot.Menus) != 1 || snapshot.Menus[0].Items[0].Label != "Save" || len(snapshot.Windows) != 1 || snapshot.NowPlaying == nil || snapshot.NowPlaying.Title != "Song" {
		t.Fatalf("state snapshot = %+v", snapshot)
	}
	if !session.UsesNowPlaying() {
		t.Fatal("session did not report its NowPlaying provider")
	}
	contextItems := session.ContextMenu("row")
	if len(contextItems) != 1 || contextItems[0].Label != "Open" {
		t.Fatalf("context menu = %+v", contextItems)
	}
	snapshot.Menus[0].Items[0].Keys[0].Rune = 'x'
	snapshot.Menus[0].Items[0].Label = "changed"
	snapshot.Windows[0].ID = "changed"
	snapshot.NowPlaying.Title = "changed"
	contextItems[0].Label = "changed"
	if s.menus[0].Items[0].Label != "Save" || s.menus[0].Items[0].Keys[0].Rune != 's' || s.windows[0].ID != "secondary" || s.item.Title != "Song" || s.context[0].Label != "Open" {
		t.Fatal("host snapshot or target query exposed app-owned mutable values")
	}
	start := ResizeEvent{Width: 40, Height: 12}
	session.Step(StepInput{Start: &start})
	session.Step(StepInput{FrameDue: true, RenderContext: RenderContext{Width: 40, Height: 12}})
	if frame, ok := session.WindowFrame("secondary"); !ok || frame.Content != "main" {
		t.Fatalf("secondary frame = (%+v, %t)", frame, ok)
	}
}

func TestCellDialogResizeRoutingPreservesHostMenuPolicy(t *testing.T) {
	newCell := func(routeResize bool) *sessionCell {
		cell := &sessionCell{dialogResizeRoutesThroughMenus: routeResize}
		cell.menus.open([]MenuItem{{ID: "menu", Label: "Menu"}}, 0, 0, 20, 8, nil)
		cell.dialogs.show(Dialog{Alert: &AlertDialog{Title: "Confirm"}}, func(DialogResult) {})
		return cell
	}
	ctx := RenderContext{Width: 20, Height: 8}
	resize := ResizeEvent{Width: 21, Height: 9}

	terminal := newCell(false)
	if got := routeCellEvent(terminal, App[struct{}]{}, resize, ctx); len(got) != 1 || got[0] != resize || !terminal.menus.isOpen() {
		t.Fatalf("terminal dialog resize route = %v, menu open=%t; want resize delivered and menu retained", got, terminal.menus.isOpen())
	}

	browser := newCell(true)
	if got := routeCellEvent(browser, App[struct{}]{}, resize, ctx); len(got) != 1 || got[0] != resize || browser.menus.isOpen() {
		t.Fatalf("browser dialog resize route = %v, menu open=%t; want resize delivered and menu closed", got, browser.menus.isOpen())
	}
}
