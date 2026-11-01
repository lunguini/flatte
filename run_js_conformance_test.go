//go:build js && wasm

package flatte_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"syscall/js"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/hostconformance"
)

func TestBrowserHostConformance(t *testing.T) {
	want := hostconformance.Expectations{
		Capabilities: flatte.Capabilities{
			Alert: true, Notifications: true, Clipboard: true, Mouse: true,
		},
		AlertButton:    1,
		ExecError:      "unsupported",
		ClipboardRead:  true,
		ClipboardValue: hostconformance.ClipboardText,
		CtrlCDelivered: true,
		CtrlCExitsRun:  false,
	}
	if err := hostconformance.Verify(browserConformanceAdapter{t: t}, want); err != nil {
		t.Fatal(err)
	}
}

type browserConformanceAdapter struct{ t *testing.T }

func (a browserConformanceAdapter) Start(app flatte.App[hostconformance.State], state *hostconformance.State, observer *hostconformance.Observer) (hostconformance.Driver, error) {
	a.t.Helper()
	dom := js.Global().Get("__flatteTest")
	if dom.IsNull() || dom.IsUndefined() {
		return nil, fmt.Errorf("browser DOM test shim is not installed")
	}
	dom.Set("hostExit", false)
	observer.OnExit = func() { dom.Set("hostExit", true) }
	ctx, cancel := context.WithCancel(context.Background())
	d := &browserConformanceDriver{dom: dom, state: state, cancel: cancel, done: make(chan error, 1)}
	go func() {
		d.done <- flatte.Run(ctx, app, flatte.WithMouse(flatte.MouseModeCellMotion))
	}()
	return d, nil
}

type browserConformanceDriver struct {
	dom      js.Value
	state    *hostconformance.State
	cancel   context.CancelFunc
	done     chan error
	finished atomic.Bool
}

func (d *browserConformanceDriver) Send(ev flatte.KeyEvent) error {
	input := d.dom.Get("input")
	if input.IsNull() || input.IsUndefined() {
		return fmt.Errorf("browser host has no input element")
	}
	key := ""
	values := map[string]any{}
	switch ev.Key {
	case flatte.KeyCtrlC:
		key = "c"
		values["ctrlKey"] = true
	case flatte.KeyEnter:
		key = "Enter"
	case flatte.KeyTab:
		key = "Tab"
	case flatte.KeyEscape:
		key = "Escape"
	case flatte.KeyCharacter:
		input.Set("value", string(ev.Rune))
		input.Call("__emit", "input", map[string]any{})
		return nil
	default:
		return fmt.Errorf("browser adapter cannot encode key %+v", ev)
	}
	values["key"] = key
	input.Call("__emit", "keydown", values)
	return nil
}

func (d *browserConformanceDriver) AnswerAlert(button int) error {
	if button == 1 {
		if err := d.Send(flatte.KeyEvent{Key: flatte.KeyTab}); err != nil {
			return err
		}
	}
	return d.Send(flatte.KeyEvent{Key: flatte.KeyEnter})
}

func (d *browserConformanceDriver) Wait(ready func(hostconformance.Snapshot) bool) error {
	return hostconformance.WaitUntil(5*time.Second, func() bool {
		d.pollDone()
		return ready(d.state.Read())
	})
}

func (d *browserConformanceDriver) Finished() bool {
	d.pollDone()
	return d.finished.Load()
}

func (d *browserConformanceDriver) ClipboardWriteObserved() bool {
	return d.dom.Get("clipboardText").String() == hostconformance.ClipboardText
}

func (d *browserConformanceDriver) Close() {
	d.cancel()
	if !d.Finished() {
		select {
		case <-d.done:
			d.finished.Store(true)
		case <-time.After(5 * time.Second):
		}
	}
}

func (d *browserConformanceDriver) pollDone() {
	select {
	case <-d.done:
		d.finished.Store(true)
	default:
	}
}
