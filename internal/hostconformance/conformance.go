package hostconformance

import (
	"fmt"
	"reflect"
	"time"

	"github.com/lunguini/flatte"
)

// Driver is the small surface a host adapter needs to expose to the shared
// conformance checks. Wait must keep driving queued host work while waiting.
type Driver interface {
	Send(flatte.KeyEvent) error
	AnswerAlert(button int) error
	Wait(func(Snapshot) bool) error
	Finished() bool
	ClipboardWriteObserved() bool
	Close()
}

// Adapter starts the common app through one host's existing test seam.
type Adapter interface {
	Start(flatte.App[State], *State, *Observer) (Driver, error)
}

// Expectations captures deliberate host differences. Capabilities must be
// identical in Effects and RenderContext for each host.
type Expectations struct {
	Capabilities   flatte.Capabilities
	AlertButton    int
	AlertError     string
	ExecError      string
	ClipboardRead  bool
	ClipboardValue string
	CtrlCDelivered bool
	CtrlCExitsRun  bool
}

// Verify runs the same lifecycle, async, dialog, effect, clipboard, Quit,
// and Ctrl+C cases against an adapter. It returns a concrete failure message
// so tests in the root and nested window modules can share this suite.
func Verify(adapter Adapter, want Expectations) error {
	app, state, observer := New()
	d, err := adapter.Start(app, state, observer)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer d.Close()

	if err := d.Wait(func(s Snapshot) bool { return s.Async && s.Width > 0 }); err != nil {
		return fmt.Errorf("initial async update and resize: %w", err)
	}
	got := state.Read()
	if !got.Initialized || !reflect.DeepEqual(got.Order, []string{"init", "resize"}) {
		return fmt.Errorf("startup order = %v, initialized=%t; want Init then first ResizeEvent", got.Order, got.Initialized)
	}
	if got.Capabilities != want.Capabilities {
		return fmt.Errorf("Effects capabilities = %+v, want %+v", got.Capabilities, want.Capabilities)
	}
	if caps, ok := observer.RenderCapabilities(); !ok || caps != want.Capabilities {
		return fmt.Errorf("RenderContext capabilities = %+v (observed=%t), want %+v", caps, ok, want.Capabilities)
	}

	if err := d.Send(key('a')); err != nil {
		return fmt.Errorf("send Alert request: %w", err)
	}
	if err := d.AnswerAlert(want.AlertButton); err != nil {
		return fmt.Errorf("answer Alert: %w", err)
	}
	if err := d.Wait(func(s Snapshot) bool { return s.AlertDone }); err != nil {
		return fmt.Errorf("Alert fold (state=%+v): %w", state.Read(), err)
	}
	got = state.Read()
	if got.AlertButton != want.AlertButton || got.AlertError != want.AlertError {
		return fmt.Errorf("Alert result button=%d error=%q, want button=%d error=%q", got.AlertButton, got.AlertError, want.AlertButton, want.AlertError)
	}

	if err := d.Send(key('e')); err != nil {
		return fmt.Errorf("send Exec request: %w", err)
	}
	if err := d.Wait(func(s Snapshot) bool { return s.ExecDone }); err != nil {
		return fmt.Errorf("Exec fold (state=%+v): %w", state.Read(), err)
	}
	if got = state.Read(); got.ExecError != want.ExecError {
		return fmt.Errorf("Exec error = %q, want %q", got.ExecError, want.ExecError)
	}

	if err := d.Send(key('c')); err != nil {
		return fmt.Errorf("send clipboard request: %w", err)
	}
	if err := d.Wait(func(s Snapshot) bool {
		if want.ClipboardRead {
			return s.Clipboard == want.ClipboardValue
		}
		return d.ClipboardWriteObserved()
	}); err != nil {
		return fmt.Errorf("clipboard round trip: %w", err)
	}
	if !d.ClipboardWriteObserved() {
		return fmt.Errorf("clipboard write was not observed")
	}

	if err := d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}); err != nil {
		return fmt.Errorf("send Quit request: %w", err)
	}
	if err := d.Wait(func(s Snapshot) bool { return s.Exited }); err != nil {
		return fmt.Errorf("Quit and OnExit: %w", err)
	}
	if !d.Finished() {
		return fmt.Errorf("Quit did not end the host run")
	}
	if got = state.Read(); !got.QuitRequested || !got.Exited {
		return fmt.Errorf("Quit state = requested:%t exited:%t", got.QuitRequested, got.Exited)
	}

	ctrlApp, ctrlState, ctrlObserver := New()
	ctrl, err := adapter.Start(ctrlApp, ctrlState, ctrlObserver)
	if err != nil {
		return fmt.Errorf("start Ctrl+C case: %w", err)
	}
	if err := ctrl.Wait(func(s Snapshot) bool { return s.Async }); err != nil {
		ctrl.Close()
		return fmt.Errorf("Ctrl+C case startup: %w", err)
	}
	if err := ctrl.Send(flatte.KeyEvent{Key: flatte.KeyCtrlC}); err != nil {
		ctrl.Close()
		return fmt.Errorf("send Ctrl+C: %w", err)
	}
	if want.CtrlCExitsRun {
		if err := ctrl.Wait(func(s Snapshot) bool { return s.Exited && ctrl.Finished() }); err != nil {
			ctrl.Close()
			return fmt.Errorf("default Ctrl+C exit: %w", err)
		}
		if ctrlState.Read().CtrlC != 0 || !ctrl.Finished() {
			ctrl.Close()
			return fmt.Errorf("default Ctrl+C delivered=%d finished=%t, want undelivered and finished", ctrlState.Read().CtrlC, ctrl.Finished())
		}
	} else {
		if err := ctrl.Wait(func(s Snapshot) bool { return s.CtrlC == 1 }); err != nil {
			ctrl.Close()
			return fmt.Errorf("Ctrl+C delivery: %w", err)
		}
		if ctrl.Finished() {
			ctrl.Close()
			return fmt.Errorf("Ctrl+C unexpectedly ended the host run")
		}
		ctrl.Close()
		if err := ctrl.Wait(func(s Snapshot) bool { return s.Exited }); err != nil {
			return fmt.Errorf("OnExit after Ctrl+C case cleanup: %w", err)
		}
	}
	if got := ctrlState.Read().CtrlC == 1; got != want.CtrlCDelivered {
		return fmt.Errorf("Ctrl+C delivered = %t, want %t", got, want.CtrlCDelivered)
	}
	return nil
}

func key(r rune) flatte.KeyEvent { return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r} }

// WaitUntil is a utility for adapters that poll a host seam without a
// testing.T dependency.
func WaitUntil(timeout time.Duration, ready func() bool) error {
	deadline := time.Now().Add(timeout)
	for !ready() {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}
		time.Sleep(time.Millisecond)
	}
	return nil
}
