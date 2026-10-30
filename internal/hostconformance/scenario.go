// Package hostconformance contains one application scenario shared by the
// terminal, browser, and window host conformance tests. Hosts provide only
// event delivery, settling, alert answers, and lifecycle observation.
package hostconformance

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/lunguini/flatte"
)

const ClipboardText = "host conformance clipboard"

// Snapshot is the app-owned result of the common host scenario.
type Snapshot struct {
	Order         []string
	Keys          []string
	Initialized   bool
	Async         bool
	Width, Height int
	AlertDone     bool
	AlertButton   int
	AlertError    string
	ExecDone      bool
	ExecError     string
	Clipboard     string
	CtrlC         int
	QuitRequested bool
	Exited        bool
	Capabilities  flatte.Capabilities
}

// State belongs to the scenario app. Its mutex only lets a host adapter
// inspect completed loop transitions while the host is still running.
type State struct {
	mu sync.Mutex
	v  Snapshot
}

// Read returns a copy of the scenario state.
func (s *State) Read() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.v
	v.Order = append([]string(nil), v.Order...)
	v.Keys = append([]string(nil), v.Keys...)
	return v
}

func (s *State) change(f func(*Snapshot)) {
	s.mu.Lock()
	f(&s.v)
	s.mu.Unlock()
}

// Observer captures the RenderContext capabilities and allows executable
// browser smokes to mark OnExit in the DOM shim.
type Observer struct {
	mu         sync.Mutex
	renderCaps flatte.Capabilities
	hasRender  bool
	OnExit     func()
}

// RenderCapabilities returns the capabilities seen by View, once rendered.
func (o *Observer) RenderCapabilities() (flatte.Capabilities, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.renderCaps, o.hasRender
}

func (o *Observer) rendered(c flatte.Capabilities) {
	o.mu.Lock()
	o.renderCaps, o.hasRender = c, true
	o.mu.Unlock()
}

// New returns the app and observer used by every host adapter. The scenario
// records Init/Resize ordering, one async fold, Alert and Exec answers,
// clipboard round-tripping, Ctrl+C delivery, Quit, and OnExit.
func New() (flatte.App[State], *State, *Observer) {
	s := &State{v: Snapshot{AlertButton: -2}}
	o := &Observer{}
	app := flatte.App[State]{
		State: s,
		Init: func(s *State, fx flatte.Effects[State]) {
			s.change(func(v *Snapshot) {
				v.Initialized = true
				v.Order = append(v.Order, "init")
				v.Capabilities = fx.Capabilities()
			})
			flatte.Go(fx, "host-conformance-async", func(context.Context) (bool, error) {
				return true, nil
			}, func(s *State, ok bool, _ error) {
				s.change(func(v *Snapshot) { v.Async = ok })
			})
		},
		Handle: func(s *State, ev flatte.Event, fx flatte.Effects[State]) {
			switch ev := ev.(type) {
			case flatte.ResizeEvent:
				s.change(func(v *Snapshot) {
					v.Order = append(v.Order, "resize")
					v.Width, v.Height = ev.Width, ev.Height
				})
			case flatte.ClipboardEvent:
				s.change(func(v *Snapshot) { v.Clipboard = ev.Text })
			case flatte.KeyEvent:
				s.change(func(v *Snapshot) { v.Keys = append(v.Keys, fmt.Sprintf("%+v", ev)) })
				if ev.Key == flatte.KeyCtrlC {
					s.change(func(v *Snapshot) { v.CtrlC++ })
					return
				}
				if ev.Key != flatte.KeyCharacter {
					return
				}
				switch ev.Rune {
				case 'a':
					flatte.Alert(fx, "host-conformance-alert", flatte.AlertDialog{
						Title: "Host conformance", Message: "Choose a result", Buttons: []string{"No", "Yes"},
					}, func(s *State, button int, err error) {
						s.change(func(v *Snapshot) {
							v.AlertDone, v.AlertButton, v.AlertError = true, button, errorName(err)
						})
					})
				case 'e':
					flatte.Exec(fx, "host-conformance-exec", exec.Command("go", "version"), func(s *State, err error) {
						s.change(func(v *Snapshot) { v.ExecDone, v.ExecError = true, errorName(err) })
					})
				case 'c':
					if ev.Mod != 0 {
						return
					}
					fx.SetClipboard(ClipboardText)
					fx.ReadClipboard()
				case 'q':
					s.change(func(v *Snapshot) { v.QuitRequested = true })
					fx.Quit()
				}
			}
		},
		View: func(s *State, ctx flatte.RenderContext) flatte.Frame {
			o.rendered(ctx.Capabilities)
			v := s.Read()
			return flatte.Frame{Content: fmt.Sprintf(
				"async=%t alert=%t/%d/%s exec=%t/%s clipboard=%s ctrlc=%d initialized=%t order=%s size=%d,%d fxcaps=%s ctxcaps=%s",
				v.Async, v.AlertDone, v.AlertButton, v.AlertError, v.ExecDone, v.ExecError,
				v.Clipboard, v.CtrlC, v.Initialized, strings.Join(v.Order, ","), v.Width, v.Height,
				capabilitySummary(v.Capabilities), capabilitySummary(ctx.Capabilities),
			)}
		},
		OnExit: func(s *State) {
			s.change(func(v *Snapshot) { v.Exited = true })
			if o.OnExit != nil {
				o.OnExit()
			}
		},
	}
	return app, s, o
}

func capabilitySummary(c flatte.Capabilities) string {
	return fmt.Sprintf("exec:%t,alert:%t,clipboard:%t,mouse:%t,quit:%t", c.Exec, c.Alert, c.Clipboard, c.Mouse, c.Quit)
}

func errorName(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, flatte.ErrUnsupported):
		return "unsupported"
	case errors.Is(err, flatte.ErrNoDialogs):
		return "no-dialogs"
	default:
		return err.Error()
	}
}
