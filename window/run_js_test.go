//go:build js && wasm

package window

import (
	"context"
	_ "embed"
	"fmt"
	"sync/atomic"
	"syscall/js"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/hostconformance"
)

//go:embed testdata/flat-mono.ttf
var canvasTestFont []byte

func TestBrowserWindowSupported(t *testing.T) {
	if !Supported() {
		t.Fatal("Supported() = false on js/wasm, want canvas support")
	}
}

func TestCanvasFontsLoadBeforeCanvasStarts(t *testing.T) {
	doc := js.Global().Get("document")
	if err := loadCanvasFonts(doc, Dark(), [][]byte{canvasTestFont}); err != nil {
		t.Fatalf("loadCanvasFonts: %v", err)
	}
	if got := doc.Get("loadedFonts").Length(); got == 0 {
		t.Fatal("FontFace was not added to document.fonts")
	}
}

func TestCanvasHostConformance(t *testing.T) {
	want := hostconformance.Expectations{
		Capabilities: flatte.Capabilities{Alert: true, Notifications: true, Clipboard: true, Drop: true, Mouse: true},
		AlertButton:  0, ExecError: "unsupported", ClipboardRead: true,
		ClipboardValue: hostconformance.ClipboardText, CtrlCDelivered: true,
	}
	if err := hostconformance.Verify(canvasConformanceAdapter{t: t}, want); err != nil {
		t.Fatal(err)
	}
}

type canvasConformanceAdapter struct{ t *testing.T }

func (a canvasConformanceAdapter) Start(app flatte.App[hostconformance.State], state *hostconformance.State, observer *hostconformance.Observer) (hostconformance.Driver, error) {
	a.t.Helper()
	dom := js.Global().Get("__flatteTest")
	if dom.IsNull() || dom.IsUndefined() {
		return nil, fmt.Errorf("browser DOM test shim is not installed")
	}
	dom.Set("hostExit", false)
	dom.Set("inputRemoved", false)
	observer.OnExit = func() { dom.Set("hostExit", true) }
	ctx, cancel := context.WithCancel(context.Background())
	d := &canvasConformanceDriver{dom: dom, state: state, cancel: cancel, done: make(chan error, 1)}
	go func() { d.done <- flatte.Run(ctx, app, Options{}) }()
	return d, nil
}

type canvasConformanceDriver struct {
	dom      js.Value
	state    *hostconformance.State
	cancel   context.CancelFunc
	done     chan error
	finished atomic.Bool
}

func (d *canvasConformanceDriver) Send(ev flatte.KeyEvent) error {
	input := d.dom.Get("input")
	if input.IsNull() || input.IsUndefined() {
		return fmt.Errorf("canvas host has no hidden input")
	}
	name, values := "", map[string]any{}
	switch ev.Key {
	case flatte.KeyCtrlC:
		name, values["ctrlKey"] = "c", true
	case flatte.KeyCharacter:
		input.Set("value", string(ev.Rune))
		input.Call("__emit", "input", map[string]any{})
		return nil
	case flatte.KeyEnter:
		name = "Enter"
	case flatte.KeyTab:
		name = "Tab"
	case flatte.KeyEscape:
		name = "Escape"
	case flatte.KeyBackspace:
		name = "Backspace"
	case flatte.KeyDelete:
		name = "Delete"
	case flatte.KeyUp:
		name = "ArrowUp"
	case flatte.KeyDown:
		name = "ArrowDown"
	case flatte.KeyLeft:
		name = "ArrowLeft"
	case flatte.KeyRight:
		name = "ArrowRight"
	default:
		return fmt.Errorf("canvas adapter cannot encode key %+v", ev)
	}
	if ev.Mod&flatte.ModShift != 0 {
		values["shiftKey"] = true
	}
	if ev.Mod&flatte.ModAlt != 0 {
		values["altKey"] = true
	}
	if ev.Mod&flatte.ModCtrl != 0 {
		values["ctrlKey"] = true
	}
	if ev.Mod&flatte.ModSuper != 0 {
		values["metaKey"] = true
	}
	values["key"] = name
	input.Call("__emit", "keydown", js.ValueOf(values))
	return nil
}

func (d *canvasConformanceDriver) AnswerAlert(int) error { return nil }

func (d *canvasConformanceDriver) Wait(ready func(hostconformance.Snapshot) bool) error {
	return hostconformance.WaitUntil(5*time.Second, func() bool {
		if flush := d.dom.Get("flushAnimationFrame"); flush.Type() == js.TypeFunction {
			flush.Invoke()
		}
		d.pollDone()
		return ready(d.state.Read())
	})
}

func (d *canvasConformanceDriver) Finished() bool { d.pollDone(); return d.finished.Load() }
func (d *canvasConformanceDriver) ClipboardWriteObserved() bool {
	return d.dom.Get("clipboardText").String() == hostconformance.ClipboardText
}

func (d *canvasConformanceDriver) Close() {
	d.cancel()
	_ = hostconformance.WaitUntil(5*time.Second, func() bool { d.pollDone(); return d.finished.Load() })
}

func (d *canvasConformanceDriver) pollDone() {
	select {
	case <-d.done:
		d.finished.Store(true)
	default:
	}
}
