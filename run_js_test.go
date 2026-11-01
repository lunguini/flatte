//go:build js && wasm

package flatte

import (
	"context"
	"fmt"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

type browserRunTestState struct {
	async   bool
	width   int
	text    string
	focused bool
	exited  bool
}

func TestBrowserRunLifecycleInputRenderingAndCleanup(t *testing.T) {
	testDOM := jsTestDOM(t)
	state := &browserRunTestState{}
	graphical := true
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[browserRunTestState]{
			State: state,
			Init: func(_ *browserRunTestState, fx Effects[browserRunTestState]) {
				Go(fx, "boot", func(context.Context) (bool, error) { return true, nil }, func(s *browserRunTestState, value bool, _ error) {
					s.async = value
				})
			},
			Handle: func(s *browserRunTestState, ev Event, _ Effects[browserRunTestState]) {
				switch ev := ev.(type) {
				case ResizeEvent:
					s.width = ev.Width
				case KeyEvent:
					if ev.Key == KeyCharacter {
						s.text += string(ev.Rune)
					} else if ev.Key == KeyRight {
						s.text += "→"
					}
				case PasteEvent:
					s.text += "[" + ev.Text + "]"
				case FocusEvent:
					s.focused = ev.Focused
				case MouseEvent:
					if ev.Action == MousePress {
						s.text += fmt.Sprintf("{%d,%d}", ev.X, ev.Y)
					}
				}
			},
			View: func(s *browserRunTestState, renderContext RenderContext) Frame {
				graphical = renderContext.Graphical
				return Frame{
					Content: fmt.Sprintf("\x1b[1;38;2;1;2;3m<safe>\x1b[0m async=%t width=%d focus=%t text=%s", s.async, s.width, s.focused, s.text),
					Title:   "browser-run",
					Cursor:  &Cursor{X: 2, Y: 1, Style: &CursorStyle{Shape: CursorShapeBar, Blink: true}},
				}
			},
			OnExit: func(s *browserRunTestState) { s.exited = true },
		}, WithMouse(MouseModeCellMotion), WithReportFocus())
	}()

	waitJS(t, func() bool {
		return strings.Contains(testDOM.Get("text").String(), "async=true")
	})
	if got := testDOM.Get("title").String(); got != "browser-run" {
		t.Fatalf("document title = %q", got)
	}
	if got := testDOM.Get("text").String(); !strings.Contains(got, "<safe>") {
		t.Fatalf("rendered text lost literal markup: %q", got)
	}
	if !testDOM.Get("cursorVisible").Bool() {
		t.Fatal("Frame.Cursor was not rendered")
	}
	if graphical {
		t.Fatal("browser RenderContext.Graphical = true, want false")
	}
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "focus=true") })

	input := testDOM.Get("input")
	input.Call("blur")
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "focus=false") })
	input.Call("focus")
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "focus=true") })
	input.Call("__emit", "keydown", map[string]any{"key": "ArrowRight"})
	input.Set("value", "é")
	input.Call("__emit", "input", map[string]any{})
	input.Call("__emit", "compositionstart", map[string]any{})
	input.Call("__emit", "compositionend", map[string]any{"data": "界"})
	input.Call("__emit", "paste", map[string]any{"text": "paste"})
	testDOM.Get("surface").Call("__emit", "pointerdown", map[string]any{"clientX": 24, "clientY": 24})
	waitJS(t, func() bool {
		return strings.Contains(testDOM.Get("text").String(), "text=→é界[paste]{2,1}")
	})

	testDOM.Get("surface").Set("clientWidth", 640)
	testDOM.Call("emitWindow", "resize", map[string]any{})
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "width=78") })

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
	if !state.exited || !testDOM.Get("inputRemoved").Bool() || testDOM.Get("listenerCount").Int() != 0 {
		t.Fatalf("cleanup: exited=%v inputRemoved=%v listeners=%d", state.exited, testDOM.Get("inputRemoved").Bool(), testDOM.Get("listenerCount").Int())
	}
}

func TestBrowserRunAlertUsesCellDialogAndFileDialogsAreUnsupported(t *testing.T) {
	testDOM := jsTestDOM(t)
	state := &struct {
		button            int
		alertErr, openErr error
	}{button: -2}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[struct {
			button            int
			alertErr, openErr error
		}]{
			State: state,
			Init: func(s *struct {
				button            int
				alertErr, openErr error
			}, fx Effects[struct {
				button            int
				alertErr, openErr error
			}]) {
				Alert(fx, "alert", AlertDialog{Title: "Confirm", Message: "Save changes?", Buttons: []string{"Yes", "No"}}, func(s *struct {
					button            int
					alertErr, openErr error
				}, button int, err error) {
					s.button, s.alertErr = button, err
				})
				OpenFile(fx, "open", OpenDialog{}, func(s *struct {
					button            int
					alertErr, openErr error
				}, _ []string, err error) {
					s.openErr = err
				})
			},
			View: func(*struct {
				button            int
				alertErr, openErr error
			}, RenderContext) Frame {
				return Frame{Content: "app"}
			},
		})
	}()
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "Save changes?") })
	if state.openErr != ErrNoDialogs {
		t.Fatalf("OpenFile error = %v, want ErrNoDialogs", state.openErr)
	}
	testDOM.Get("input").Call("__emit", "keydown", map[string]any{"key": "Enter"})
	waitJS(t, func() bool { return state.alertErr == nil && state.button == 0 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBrowserRunRoutesCommandAndControlShortcutsAndPaste(t *testing.T) {
	testDOM := jsTestDOM(t)
	var keys []KeyEvent
	var menus []string
	var pasted string
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[int]{
			State: new(int),
			Menus: func(*int) []Menu {
				return []Menu{{Items: []MenuItem{
					{ID: "command-save", Keys: []KeyEvent{{Key: KeyCharacter, Rune: 's', Mod: ModSuper}}},
					{ID: "control-save", Keys: []KeyEvent{{Key: KeyCharacter, Rune: 's', Mod: ModCtrl}}},
				}}}
			},
			Handle: func(_ *int, ev Event, _ Effects[int]) {
				switch ev := ev.(type) {
				case KeyEvent:
					keys = append(keys, ev)
				case MenuEvent:
					menus = append(menus, ev.ID)
				case PasteEvent:
					pasted += ev.Text
				}
			},
			View: func(_ *int, _ RenderContext) Frame {
				return Frame{Content: fmt.Sprintf("keys=%v menus=%s paste=%s", keys, strings.Join(menus, ","), pasted)}
			},
		})
	}()
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "menus=") })

	input := testDOM.Get("input")
	keydown := func(values map[string]any) bool {
		t.Helper()
		return input.Call("__emit", "keydown", values).Get("defaultPrevented").Bool()
	}
	if !keydown(map[string]any{"key": "s", "metaKey": true}) {
		t.Fatal("Cmd+S: browser default was not prevented")
	}
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "menus=command-save") })
	if !keydown(map[string]any{"key": "s", "ctrlKey": true}) {
		t.Fatal("Ctrl+S: browser default was not prevented")
	}
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "command-save,control-save") })
	if !keydown(map[string]any{"key": "c", "ctrlKey": true}) {
		t.Fatal("Ctrl+C: default was not prevented")
	}
	if keydown(map[string]any{"key": "v", "metaKey": true}) {
		t.Fatal("Cmd+V: browser paste default was prevented")
	}
	input.Call("__emit", "paste", map[string]any{"text": "pasted"})
	input.Call("__emit", "keydown", map[string]any{"key": "ArrowLeft", "metaKey": true})
	waitJS(t, func() bool {
		return strings.Contains(testDOM.Get("text").String(), "paste=pasted")
	})

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != (KeyEvent{Key: KeyCtrlC, Mod: ModCtrl}) || keys[1] != (KeyEvent{Key: KeyLeft, Mod: ModSuper}) {
		t.Fatalf("delivered keys = %v, want Ctrl+C and Cmd+Left", keys)
	}
	if fmt.Sprint(menus) != "[command-save control-save]" || pasted != "pasted" {
		t.Fatalf("delivered menus=%v paste=%q", menus, pasted)
	}
}

func TestBrowserRunSettlesLayoutRoutesContextMenusAndBatchesWheel(t *testing.T) {
	testDOM := jsTestDOM(t)
	type state struct {
		layout  bool
		chosen  string
		wheel   int
		target  string
		renders int
	}
	s := &state{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[state]{
			State: s,
			ContextMenu: func(_ *state, target string) []MenuItem {
				if target == "hit" {
					return []MenuItem{{ID: "select", Label: "Select"}}
				}
				return nil
			},
			Handle: func(s *state, ev Event, _ Effects[state]) {
				switch ev := ev.(type) {
				case LayoutEvent:
					s.layout = ev.Rects["hit"].W > 0
				case MenuEvent:
					s.chosen = ev.ID + ":" + ev.Target
				case MouseEvent:
					if ev.Button == MouseWheelDown || ev.Button == MouseWheelUp {
						s.wheel++
						s.target = ev.Target
					}
				}
			},
			View: func(s *state, _ RenderContext) Frame {
				s.renders++
				return Frame{Root: layout.Col{Children: []layout.Node{
					layout.Label{NodeBase: layout.NodeBase{ID: "hit"}, Text: fmt.Sprintf("hit layout=%t chosen=%s wheel=%d target=%s", s.layout, s.chosen, s.wheel, s.target)},
				}}}
			},
		}, WithMouse(MouseModeCellMotion))
	}()
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "layout=true") })
	if got := testDOM.Get("text").String(); !strings.Contains(got, "hit layout=true") {
		t.Fatalf("Frame.Root was not composed into text cells: %q", got)
	}

	surface := testDOM.Get("surface")
	surface.Call("__emit", "pointerdown", map[string]any{"button": 2, "clientX": 16, "clientY": 8})
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "Select") })
	// The popup's first item is one row below its border at this position.
	surface.Call("__emit", "pointerdown", map[string]any{"button": 0, "clientX": 24, "clientY": 24})
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "chosen=select:hit") })

	before := s.renders
	surface.Call("__emit", "wheel", map[string]any{"deltaY": 16, "clientX": 24, "clientY": 8})
	surface.Call("__emit", "wheel", map[string]any{"deltaY": 16, "clientX": 24, "clientY": 8})
	if testDOM.Get("pendingAnimationFrames").Int() != 1 {
		t.Fatalf("queued animation frames = %d, want one", testDOM.Get("pendingAnimationFrames").Int())
	}
	testDOM.Call("flushAnimationFrame")
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "wheel=2") })
	if s.renders != before+1 {
		t.Fatalf("wheel frame rendered %d times, want one", s.renders-before)
	}
	if s.target != "hit" {
		t.Fatalf("wheel target = %q, want hit", s.target)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBrowserRunClipboardAndNotificationsDegradeByPermission(t *testing.T) {
	testDOM := jsTestDOM(t)
	testDOM.Call("setNotificationPermission", "granted")
	type state struct{ clipboard string }
	s := &state{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, App[state]{
			State: s,
			Init: func(_ *state, fx Effects[state]) {
				fx.SetClipboard("browser clipboard")
				fx.ReadClipboard()
				fx.Notify(Notification{Title: "Ready", Body: "clipboard checked"})
			},
			Handle: func(s *state, ev Event, fx Effects[state]) {
				switch ev := ev.(type) {
				case ClipboardEvent:
					s.clipboard = ev.Text
				case KeyEvent:
					if ev.Rune == 'n' && ev.Mod == ModCtrl {
						fx.Notify(Notification{Title: "Denied", Body: "must not appear"})
					}
				}
			},
			View: func(s *state, _ RenderContext) Frame { return Frame{Content: "clipboard=" + s.clipboard} },
		})
	}()
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "clipboard=browser clipboard") })
	if got := testDOM.Get("clipboardText").String(); got != "browser clipboard" {
		t.Fatalf("clipboard write = %q", got)
	}
	if got := testDOM.Get("notifications").Length(); got != 1 {
		t.Fatalf("notifications with permission = %d, want 1", got)
	}
	testDOM.Call("setNotificationPermission", "denied")
	testDOM.Get("input").Call("__emit", "keydown", map[string]any{"key": "n", "ctrlKey": true})
	waitJS(t, func() bool { return strings.Contains(testDOM.Get("text").String(), "clipboard=browser clipboard") })
	if got := testDOM.Get("notifications").Length(); got != 1 {
		t.Fatalf("notifications without permission = %d, want 1", got)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBrowserRunRequiresMount(t *testing.T) {
	testDOM := jsTestDOM(t)
	testDOM.Call("setMounted", false)
	defer testDOM.Call("setMounted", true)
	err := Run(context.Background(), App[int]{State: new(int), View: func(*int, RenderContext) Frame { return Frame{} }})
	if err == nil || !strings.Contains(err.Error(), "[data-flatte]") {
		t.Fatalf("Run error = %v, want missing mount", err)
	}
}

func TestBrowserRunQuitsFromInitialLayoutEvent(t *testing.T) {
	jsTestDOM(t)
	exited := false
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[int]{
			State: new(int),
			Handle: func(_ *int, ev Event, fx Effects[int]) {
				if _, ok := ev.(LayoutEvent); ok {
					fx.Quit()
				}
			},
			View: func(_ *int, _ RenderContext) Frame {
				return Frame{Root: layout.Label{NodeBase: layout.NodeBase{ID: "quit"}, Text: "quit after layout"}}
			},
			OnExit: func(*int) { exited = true },
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not honor Quit from the initial LayoutEvent")
	}
	if !exited {
		t.Fatal("OnExit was not called after the layout handler quit")
	}
}

func jsTestDOM(t *testing.T) js.Value {
	t.Helper()
	v := js.Global().Get("__flatteTest")
	if v.IsUndefined() || v.IsNull() {
		t.Fatal("browser DOM test shim is not installed")
	}
	return v
}

func waitJS(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for browser state")
		}
		time.Sleep(time.Millisecond)
	}
}
