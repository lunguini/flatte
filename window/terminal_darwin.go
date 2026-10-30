//go:build darwin

package window

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
)

// runTerminal pumps AppKit's main queue while the terminal loop runs on a
// worker goroutine. AppKit and MediaPlayer remain isolated in this module.
func runTerminal[S any](ctx context.Context, app flatte.App[S], opts ...flatte.Option) error {
	if err := loadFrameworks(); err != nil {
		return err
	}
	if pthreadMainNP() == 0 {
		return errNotMainThread
	}
	var runErr error
	withPool(func() { runErr = runTerminalMain(ctx, app, opts...) })
	return runErr
}

func runTerminalMain[S any](ctx context.Context, app flatte.App[S], opts ...flatte.Option) error {
	if running {
		return errors.New("window: a window or terminal host is already running")
	}
	running = true
	defer func() { running = false }()

	commands := make(chan flatte.MediaEvent, 16)
	active := atomic.Bool{}
	active.Store(true)
	defer active.Store(false)
	nsApp := class("NSApplication").Send(sel("sharedApplication"))
	previousPolicy := objc.Send[int](nsApp, sel("activationPolicy"))
	nsApp.Send(sel("setActivationPolicy:"), 1) // accessory: no Dock icon, no app window
	defer nsApp.Send(sel("setActivationPolicy:"), previousPolicy)
	media, err := newNativeMedia(func(command flatte.MediaCommand) {
		// Native callbacks must never wait for the terminal loop. A full queue
		// drops this duplicate control request rather than wedging AppKit.
		select {
		case commands <- flatte.MediaEvent{Command: command}:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer media.close()
	var stopped bool
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		nsApp.Send(sel("stop:"), objc.ID(0))
		wake(nsApp)
	}

	// Snapshot publication happens on the terminal loop; marshal the copied
	// value to the main thread before touching MediaPlayer/AppKit objects.
	publish := func(item *flatte.NowPlaying) {
		if !active.Load() {
			return
		}
		var snapshot *flatte.NowPlaying
		if item != nil {
			copy := *item
			snapshot = &copy
		}
		post(func() {
			if active.Load() {
				media.update(snapshot)
			}
		})
	}

	done := make(chan error, 1)
	terminalOpts := append(append([]flatte.Option(nil), opts...), flatte.WithHostMedia(commands, publish))
	go func() {
		err := flatte.Run(ctx, app, terminalOpts...)
		done <- err
		post(func() { stop() })
	}()

	// Run is entered even if flatte.Run exits immediately: its deferred
	// publish(nil) queues a main-thread update and the completion callback
	// below will stop the pump once AppKit starts dispatching.
	nsApp.Send(sel("run"))
	err = <-done
	return err
}
