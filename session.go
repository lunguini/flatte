package flatte

import (
	"context"
	"maps"
	"sync"

	"github.com/lunguini/flatte/internal/sessionaccess"
)

// Host runs an App through a platform-specific driver selected by Run.
type Host interface {
	Run(context.Context, *Session) error
}

// HostServices installs optional capabilities and services before a Session's
// first step. Apps use Effects for these operations.
type HostServices struct {
	Capabilities  Capabilities
	SetClipboard  func(string)
	ReadClipboard func()
	ShowDialog    func(Dialog, func(DialogResult))
	Notify        func(Notification)
	Desktop       func(DesktopCommand)
	// CellSurface enables the shared cell input, layout, dialog, menu, and
	// secondary-window rules. Graphical hosts leave it false.
	CellSurface bool
	// PublishMedia receives the app's current media snapshot after a painted
	// frame. A nil snapshot clears the host's published item.
	PublishMedia func(*NowPlaying)
}

type sessionPolicy struct {
	defaultQuit                    bool
	inline                         bool
	dialogResizeRoutesThroughMenus bool
}

// Session is the non-generic, minimal host driver handle for one App[S].
// Application state and generic Effects remain private to this package.
type Session struct {
	mu            sync.Mutex
	pending       []func()
	updates       chan struct{}
	quitting      bool
	started       bool
	exitOnce      sync.Once
	cancel        context.CancelFunc
	start         func(ResizeEvent)
	handle        func(Event)
	trace         func(Event)
	view          func(RenderContext) Frame
	services      func(HostServices)
	step          func(StepInput) StepResult
	actionSink    func(func(action))
	caps          func() Capabilities
	onExit        func()
	menus         func() []Menu
	contextMenu   func(string) []MenuItem
	windows       func() []Window
	nowPlaying    func() *NowPlaying
	hasNowPlaying bool
	runTerminal   func(context.Context, ...Option) error
	cellSurface   bool
	defaultQuit   bool
	inline        bool
	dirty         bool
	publishMedia  func(*NowPlaying)
	cell          *sessionCell
}

// NewSession wraps an app for an offscreen host. Run constructs sessions
// itself for hosts selected through an Option.
func NewSession[S any](ctx context.Context, app App[S]) *Session { return newSession(ctx, app) }

func newSession[S any](ctx context.Context, app App[S]) *Session {
	if app.State == nil {
		panic("flat: App.State is nil")
	}
	if app.View == nil {
		panic("flat: App.View is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	if app.Tracer == nil {
		app.Tracer = NoopTracer{}
	}
	s := &Session{updates: make(chan struct{}, 1), cancel: cancel, cell: &sessionCell{}}
	fx := newEffects[S](runCtx, nil, func() {
		s.mu.Lock()
		s.quitting = true
		s.wake()
		s.mu.Unlock()
	})
	s.actionSink = func(enqueue func(action)) { fx.enqueue = enqueue }
	fx.sendUpdate = func(sendCtx context.Context, update StateUpdate[S]) bool {
		if sendCtx != nil {
			select {
			case <-sendCtx.Done():
				return false
			default:
			}
		}
		s.enqueue(func() { ApplyUpdate(app.State, app.Tracer, update) })
		return true
	}
	s.start = func(size ResizeEvent) {
		s.mu.Lock()
		if s.started {
			s.mu.Unlock()
			return
		}
		s.started = true
		s.mu.Unlock()
		if app.Init != nil {
			app.Init(app.State, fx)
		}
		if !s.quittingNow() {
			s.handle(size)
		}
	}
	s.handle = func(ev Event) {
		handlerEvent := ev
		if layout, ok := ev.(LayoutEvent); ok {
			handlerEvent = LayoutEvent{
				Rects:      maps.Clone(layout.Rects),
				PointRects: maps.Clone(layout.PointRects),
				Window:     layout.Window,
			}
		}
		s.trace(ev)
		if app.Handle != nil {
			app.Handle(app.State, handlerEvent, fx)
		}
	}
	s.trace = func(ev Event) { app.Tracer.Event(ev) }
	s.view = func(ctx RenderContext) Frame { return app.View(app.State, ctx) }
	s.services = func(host HostServices) {
		fx = withHostCapabilities(fx, host.Capabilities)
		fx = withHostClipboard(fx, host.SetClipboard, host.ReadClipboard)
		if host.ShowDialog != nil {
			fx = withHostDialogs(fx, host.ShowDialog)
		}
		if host.Notify != nil {
			fx = withHostNotifications(fx, host.Notify)
		}
		if host.Desktop != nil {
			fx = withHostDesktop(fx, host.Desktop)
		}
		s.cellSurface = host.CellSurface
		s.publishMedia = host.PublishMedia
	}
	s.caps = func() Capabilities { return fx.Capabilities() }
	s.onExit = func() {
		if app.OnExit != nil {
			app.OnExit(app.State)
		}
	}
	s.menus = func() []Menu {
		if app.Menus == nil {
			return nil
		}
		return app.Menus(app.State)
	}
	s.contextMenu = func(target string) []MenuItem {
		if app.ContextMenu == nil {
			return nil
		}
		return app.ContextMenu(app.State, target)
	}
	s.windows = func() []Window {
		if app.Windows == nil {
			return nil
		}
		return app.Windows(app.State)
	}
	s.hasNowPlaying = app.NowPlaying != nil
	s.nowPlaying = func() *NowPlaying {
		if app.NowPlaying == nil {
			return nil
		}
		return app.NowPlaying(app.State)
	}
	s.step = func(input StepInput) StepResult { return stepSession(s, app, s.cell, input) }
	sessionaccess.Register(s, sessionaccess.Access{
		SetAsync: func(dispatch func(func()), clock sessionaccess.Clock) {
			fx.dispatch, fx.clock = dispatch, clock
		},
		ConfigureTests: func(config sessionaccess.HarnessEffects) {
			if config.Context != nil {
				fx.ctx = config.Context
			}
			if config.Updates != nil {
				updates, ok := config.Updates.(chan<- StateUpdate[S])
				if !ok {
					panic("flatte: invalid harness update channel")
				}
				fx.updates = updates
			}
			fx.sendUpdate = nil
			if config.SendUpdate != nil {
				fx.updates = nil
				fx.sendUpdate = func(ctx context.Context, update StateUpdate[S]) bool {
					return config.SendUpdate(ctx, update)
				}
			}
			fx.quit, fx.dispatch, fx.clock = config.Quit, config.Dispatch, config.Clock
			if caps, ok := config.Capabilities.(Capabilities); ok {
				fx.caps = caps
			}
			fx.setClipboard, fx.readClipboard = config.SetClipboard, config.ReadClipboard
			fx.dialogs = nil
			if config.ShowDialog != nil {
				fx.dialogs = func(dialog Dialog, done func(DialogResult)) {
					config.ShowDialog(dialog, func(result any) { done(result.(DialogResult)) })
				}
			}
			fx.notify = nil
			if config.Notify != nil {
				fx.notify = func(notification Notification) { config.Notify(notification) }
			}
			fx.desktop = nil
			if config.Desktop != nil {
				fx.desktop = func(command DesktopCommand) { config.Desktop(command) }
			}
		},
		Close: s.close,
		RunTerminal: func(ctx context.Context, media sessionaccess.Media) error {
			if s.runTerminal == nil {
				return ErrUnsupported
			}
			if media.Commands == nil && media.Publish == nil {
				return s.runTerminal(ctx)
			}
			option := optionFunc(func(config *runConfig) {
				config.mediaCommands = media.Commands.(<-chan MediaEvent)
				config.publishMedia = media.Publish.(func(*NowPlaying))
			})
			return s.runTerminal(ctx, option)
		},
	})
	return s
}

func (s *Session) enqueue(update func()) {
	s.mu.Lock()
	s.pending = append(s.pending, update)
	s.wake()
	s.mu.Unlock()
}

func (s *Session) wake() {
	select {
	case s.updates <- struct{}{}:
	default:
	}
}

// Updates returns a coalesced notification channel. Hosts select on it and
func (s *Session) Updates() <-chan struct{} { return s.updates }

func (s *Session) setActionSink(enqueue func(action)) { s.actionSink(enqueue) }

func (s *Session) quittingNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// UseHostServices installs a host's capabilities and optional services. Call
// it before the first StepInput.Start.
func (s *Session) UseHostServices(services HostServices) { s.services(services) }

func (s *Session) useCellPolicy(policy sessionPolicy) {
	s.defaultQuit = policy.defaultQuit
	s.inline = policy.inline
	s.cell.dialogResizeRoutesThroughMenus = policy.dialogResizeRoutesThroughMenus
}

// close calls the app's cleanup hook once and cancels Effects work. Run owns
// this call for selected hosts.
func (s *Session) close() {
	s.exitOnce.Do(func() {
		defer s.cancel()
		defer sessionaccess.Unregister(s)
		if s.onExit != nil {
			s.onExit()
		}
	})
}

// Cancel stops Effects work without calling the app's OnExit hook.
func (s *Session) Cancel() {
	s.cancel()
	sessionaccess.Unregister(s)
}

func runWithHost[S any](ctx context.Context, app App[S], config RunConfig) (bool, error) {
	if config.err != nil {
		return true, config.err
	}
	host := config.selectedHost()
	if host == nil {
		return false, nil
	}
	session := newSession(ctx, app)
	session.runTerminal = func(runCtx context.Context, options ...Option) error {
		runConfig := applyRunOptions(config.config, options...).config
		terminalApp := app
		terminalApp.OnExit = nil // the outer Run owns the session's OnExit call
		return runTerminalConfig(runCtx, terminalApp, runConfig)
	}
	defer session.close()
	return true, host.Run(ctx, session)
}
