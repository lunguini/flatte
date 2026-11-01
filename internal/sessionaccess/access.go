// Package sessionaccess bridges root runtime details to packages inside the
// module. It is unavailable to external modules by Go's internal-package
// rules.
package sessionaccess

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

type Clock interface {
	Tick(context.Context, time.Duration, func(time.Time))
}

// HarnessEffects contains test-only Effects overrides. Updates and service
// values are deliberately untyped here; the root package restores their
// concrete generic types when applying the configuration.
type HarnessEffects struct {
	Context       context.Context
	Updates       any
	SendUpdate    func(context.Context, any) bool
	Quit          func()
	Dispatch      func(func())
	Clock         Clock
	Capabilities  any
	SetClipboard  func(string)
	ReadClipboard func()
	ShowDialog    func(any, func(any))
	Notify        func(any)
	Desktop       func(any)
}

type Media struct {
	Commands any
	Publish  any
}

type Access struct {
	SetAsync       func(dispatch func(func()), clock Clock)
	ConfigureTests func(HarnessEffects)
	Close          func()
	RunTerminal    func(context.Context, Media) error
}

type Helpers struct {
	SettleFrame func(view, handle any, last map[string]layout.Rect, width, height int) (any, layout.Composition, map[string]layout.Rect)
	MediaOption func(commands, publish any) any
}

var sessions sync.Map // map[any]Access; keys are *flatte.Session values.
var helperMu sync.RWMutex
var helpers Helpers

func Register(session any, access Access) { sessions.Store(session, access) }

func RegisterHelpers(value Helpers) {
	helperMu.Lock()
	helpers = value
	helperMu.Unlock()
}

func SettleFrame(view, handle any, last map[string]layout.Rect, width, height int) (any, layout.Composition, map[string]layout.Rect) {
	helperMu.RLock()
	settle := helpers.SettleFrame
	helperMu.RUnlock()
	if settle == nil {
		panic("sessionaccess: root frame helper was not registered")
	}
	return settle(view, handle, last, width, height)
}

func MediaOption(commands, publish any) any {
	helperMu.RLock()
	makeOption := helpers.MediaOption
	helperMu.RUnlock()
	if makeOption == nil {
		panic("sessionaccess: root media option was not registered")
	}
	return makeOption(commands, publish)
}

func Unregister(session any) { sessions.Delete(session) }

func For(session any) (Access, bool) {
	access, ok := sessions.Load(session)
	if !ok {
		return Access{}, false
	}
	return access.(Access), true
}

// Close calls the runtime-owned app cleanup hook for an offscreen session.
func Close(session any) {
	if access, ok := For(session); ok && access.Close != nil {
		access.Close()
	}
}

// RunTerminal invokes the terminal loop attached by root Run to a selected
// composite host.
func RunTerminal(ctx context.Context, session any, media Media) error {
	if access, ok := For(session); ok && access.RunTerminal != nil {
		return access.RunTerminal(ctx, media)
	}
	return errors.New("sessionaccess: terminal driver is unavailable")
}
