//go:build darwin

package window

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"

	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
	flattehost "github.com/lunguini/flatte/host"
)

func (MediaControls) ConfigureRun(config *flattehost.RunConfig) {
	config.UseHost(mediaControlsHost{})
}

type mediaControlsHost struct{}

func (mediaControlsHost) Run(ctx context.Context, session flattehost.Session) error {
	if !flattehost.Snapshot(session).HasNowPlaying {
		return runSessionTerminal(ctx, session)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := loadFrameworks(); err != nil {
		return err
	}
	if pthreadMainNP() == 0 {
		return errNotMainThread
	}
	var runErr error
	withPool(func() { runErr = runTerminalMain(ctx, session) })
	return runErr
}

func runSessionTerminal(ctx context.Context, session flattehost.Session, options ...flatte.Option) error {
	return flattehost.RunTerminal(ctx, session, options...)
}

// runTerminalMain pumps AppKit's main queue while the terminal loop runs on a
// worker goroutine. AppKit and MediaPlayer remain isolated in this module.
func runTerminalMain(ctx context.Context, session flattehost.Session, options ...flatte.Option) error {
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
	go func() {
		err := flattehost.RunTerminal(ctx, session, append(options, flattehost.WithHostMedia(commands, publish))...)
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
