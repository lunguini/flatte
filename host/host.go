// Package host provides the public host-driver vocabulary and compatibility
// adapters around the Flatte runtime. The root package owns Run and the shared
// step core; this package imports root, so the core cannot depend back on it.
package host

import (
	"context"
	"io"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type (
	Host       = flatte.Host
	Session    = *flatte.Session
	Services   = flatte.HostServices
	State      = flatte.HostState
	StepInput  = flatte.StepInput
	StepResult = flatte.StepResult
	Option     = flatte.Option
	RunConfig  = flatte.RunConfig
	MenuHost   = flatte.MenuHost
	WindowHost = flatte.WindowHost
)

// NewSession creates the non-generic session used by a host driver.
func NewSession[S any](ctx context.Context, app flatte.App[S]) Session {
	return flatte.NewSession(ctx, app)
}

// RunTerminal delegates to the root's terminal-only composite-host hook.
func RunTerminal(ctx context.Context, session Session, options ...Option) error {
	return flatte.RunTerminal(ctx, session, options...)
}

// CloseSession finishes an offscreen session and calls its OnExit hook.
func CloseSession(session Session) { flatte.CloseSession(session) }

// Snapshot returns the host-specific app data for native presentation and
// deterministic test helpers. It is optional for drivers that only need Step.
func Snapshot(session Session) State {
	return session.State()
}

// WithHostCapabilities installs the host's capability set on Effects.
func WithHostCapabilities[S any](fx flatte.Effects[S], c flatte.Capabilities) flatte.Effects[S] {
	return flatte.WithHostCapabilities(fx, c)
}

// WithHostClipboard routes clipboard effects to the host's clipboard.
func WithHostClipboard[S any](fx flatte.Effects[S], set func(string), read func()) flatte.Effects[S] {
	return flatte.WithHostClipboard(fx, set, read)
}

// WithHostDialogs routes dialog effects to the host.
func WithHostDialogs[S any](fx flatte.Effects[S], show func(flatte.Dialog, func(flatte.DialogResult))) flatte.Effects[S] {
	return flatte.WithHostDialogs(fx, show)
}

// WithHostNotifications routes notification effects to the host.
func WithHostNotifications[S any](fx flatte.Effects[S], show func(flatte.Notification)) flatte.Effects[S] {
	return flatte.WithHostNotifications(fx, show)
}

// WithHostDesktop routes desktop commands to the host.
func WithHostDesktop[S any](fx flatte.Effects[S], command func(flatte.DesktopCommand)) flatte.Effects[S] {
	return flatte.WithHostDesktop(fx, command)
}

// WithHostMedia connects a terminal driver to a native media host.
func WithHostMedia(commands <-chan flatte.MediaEvent, publish func(*flatte.NowPlaying)) Option {
	return flatte.WithHostMedia(commands, publish)
}

// NewHarnessEffects constructs Effects for deterministic or offscreen hosts.
func NewHarnessEffects[S any](ctx context.Context, updates chan<- flatte.StateUpdate[S], quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return flatte.NewHarnessEffects(ctx, updates, quit, dispatch, clock)
}

// NewHarnessEffectsQueue constructs Effects backed by a host-owned update queue.
func NewHarnessEffectsQueue[S any](ctx context.Context, enqueue func(flatte.StateUpdate[S]) bool, quit func(), dispatch func(func()), clock flatte.Clock) flatte.Effects[S] {
	return flatte.NewHarnessEffectsQueue(ctx, enqueue, quit, dispatch, clock)
}

func MenuShortcut(menus []flatte.Menu, key flatte.KeyEvent) (flatte.MenuItem, bool) {
	return flatte.MenuShortcut(menus, key)
}

func RouteInput[S any](menus *MenuHost, windows *WindowHost, app flatte.App[S], event flatte.Event, width, height int) []flatte.Event {
	return flatte.RouteInput(menus, windows, app, event, width, height)
}

func RouteMenus[S any](menus *MenuHost, app flatte.App[S], event flatte.Event, width, height int) (flatte.Event, bool) {
	return flatte.RouteMenus(menus, app, event, width, height)
}

func SyncWindows[S any](windows *WindowHost, app flatte.App[S], width, height int) []flatte.Event {
	return flatte.SyncWindows(windows, app, width, height)
}

func PaintWindows[S any](windows *WindowHost, app flatte.App[S], base flatte.RenderContext, frame flatte.Frame, handle func(flatte.LayoutEvent)) flatte.Frame {
	return flatte.PaintWindows(windows, app, base, frame, handle)
}

func SettleFrame(view func() flatte.Frame, handle func(flatte.LayoutEvent), last map[string]layout.Rect, width, height int) (flatte.Frame, layout.Composition, map[string]layout.Rect) {
	return flatte.SettleFrame(view, handle, last, width, height)
}

func RenderContextFor(out io.Writer) flatte.RenderContext { return flatte.RenderContextFor(out) }
