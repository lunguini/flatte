//go:build darwin && !wasm

package appkit

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/sessionaccess"
)

var errNotMainThread = errors.New("window: Run must be called from the main goroutine")

var (
	postMu       sync.Mutex
	posted       []func()
	postCallback uintptr
	postOnce     sync.Once
)

// RunTerminalMedia keeps the terminal driver on a worker goroutine while the
// main thread pumps AppKit and MediaPlayer callbacks. It is used by the
// terminal media option; window hosts use NewMedia directly on their UI loop.
func RunTerminalMedia(ctx context.Context, session *flatte.Session) error {
	if !session.UsesNowPlaying() {
		return sessionaccess.RunTerminal(ctx, session, sessionaccess.Media{})
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := load(); err != nil {
		return err
	}
	if pthreadMainNP() == 0 {
		return errNotMainThread
	}
	if err := BeginRun(); err != nil {
		return err
	}
	defer EndRun()
	var runErr error
	withPool(func() { runErr = runTerminalMain(ctx, session) })
	return runErr
}

func runTerminalMain(ctx context.Context, session *flatte.Session) error {
	commands := make(chan flatte.MediaEvent, 16)
	active := atomic.Bool{}
	active.Store(true)
	defer active.Store(false)
	nsApp := class("NSApplication").Send(sel("sharedApplication"))
	previousPolicy := objc.Send[int](nsApp, sel("activationPolicy"))
	nsApp.Send(sel("setActivationPolicy:"), 1) // accessory: no Dock icon, no app window
	defer nsApp.Send(sel("setActivationPolicy:"), previousPolicy)
	media, err := NewMedia(func(command flatte.MediaCommand) {
		select {
		case commands <- flatte.MediaEvent{Command: command}:
		default: // A full queue drops a duplicate command rather than blocking AppKit.
		}
	})
	if err != nil {
		return err
	}
	defer media.Close()

	var stopped atomic.Bool
	stop := func() {
		if stopped.Swap(true) {
			return
		}
		nsApp.Send(sel("stop:"), objc.ID(0))
		wake(nsApp)
	}
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
				media.Update(snapshot)
			}
		})
	}

	done := make(chan error, 1)
	var commandSource <-chan flatte.MediaEvent = commands
	go func() {
		err := sessionaccess.RunTerminal(ctx, session, sessionaccess.Media{Commands: commandSource, Publish: publish})
		done <- err
		post(stop)
	}()
	nsApp.Send(sel("run"))
	return <-done
}

func post(f func()) {
	postOnce.Do(func() {
		postCallback = purego.NewCallback(func(uintptr) { runPosted() })
	})
	postMu.Lock()
	posted = append(posted, f)
	postMu.Unlock()
	dispatchAsyncF(dispatchMainQueue, 0, postCallback)
}

func runPosted() {
	postMu.Lock()
	fs := posted
	posted = nil
	postMu.Unlock()
	for _, f := range fs {
		withPool(f)
	}
}

func wake(nsApp objc.ID) {
	ev := class("NSEvent").Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
		uint(15), cgPoint{}, uint(0), 0.0, 0, objc.ID(0), int16(0), 0, 0)
	nsApp.Send(sel("postEvent:atStart:"), ev, true)
}
