package flatte

import (
	"context"
	"maps"
	"sync"
)

// Host is a host-plumbing interface for a platform driver selected by an
// Option. Application code should call Run instead of implementing Host.
type Host interface {
	Run(context.Context, *Session) error
}

// HostServices is the set of optional services a host provides to a Session.
// It is host plumbing; applications use Effects.
type HostServices struct {
	Capabilities  Capabilities
	SetClipboard  func(string)
	ReadClipboard func()
	ShowDialog    func(Dialog, func(DialogResult))
	Notify        func(Notification)
	Desktop       func(DesktopCommand)
	// CellSurface enables the shared cell input, layout, dialog, menu, and
	// secondary-window rules. Native graphical hosts leave it false.
	CellSurface bool
	// DefaultQuit makes Ctrl+C end the session before it reaches App.Handle.
	DefaultQuit bool
	// Inline makes cell root frames use their measured content height.
	Inline bool
	// DialogResizeRoutesThroughMenus preserves a host where a resize that
	// passes through an open cell dialog also reaches open cell menus.
	DialogResizeRoutesThroughMenus bool
	// PublishMedia receives the app's current media snapshot after a painted
	// frame. A nil snapshot clears the host's published item.
	PublishMedia func(*NowPlaying)
}

// Session is the non-generic, host-facing wrapper for one App[S]. Hosts drive
// its lifecycle, events, updates, and views without seeing application state
// or generic Effects. Its methods are host plumbing, not application API.
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

// NewSession wraps an app for use by a Host. Run normally constructs sessions
// itself; this constructor is also useful to a host's offscreen test adapter.
func NewSession[S any](ctx context.Context, app App[S]) *Session {
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
		if !s.Quitting() {
			s.Handle(size)
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
		fx = WithHostCapabilities(fx, host.Capabilities)
		fx = WithHostClipboard(fx, host.SetClipboard, host.ReadClipboard)
		if host.ShowDialog != nil {
			fx = WithHostDialogs(fx, host.ShowDialog)
		}
		if host.Notify != nil {
			fx = WithHostNotifications(fx, host.Notify)
		}
		if host.Desktop != nil {
			fx = WithHostDesktop(fx, host.Desktop)
		}
		s.cellSurface = host.CellSurface
		s.defaultQuit = host.DefaultQuit
		s.inline = host.Inline
		s.cell.dialogResizeRoutesThroughMenus = host.DialogResizeRoutesThroughMenus
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

// Start runs Init, then sends the first ResizeEvent unless Init requested
// Quit. Calling Start more than once has no effect.
func (s *Session) Start(size ResizeEvent) { s.start(size) }

// Handle delivers an event to the app and tracer.
func (s *Session) Handle(ev Event) { s.handle(ev) }

// ApplyPendingUpdates applies queued async updates on the host's loop
// goroutine and returns the number applied.
func (s *Session) ApplyPendingUpdates() int {
	count := 0
	for {
		s.mu.Lock()
		pending := s.pending
		s.pending = nil
		s.mu.Unlock()
		if len(pending) == 0 {
			return count
		}
		for _, update := range pending {
			update()
			count++
		}
	}
}

// Updates returns a coalesced notification channel. Hosts select on it and
// call ApplyPendingUpdates on their loop goroutine.
func (s *Session) Updates() <-chan struct{} { return s.updates }

func (s *Session) setActionSink(enqueue func(action)) { s.actionSink(enqueue) }

// View evaluates the app's frame for the supplied host render context.
func (s *Session) View(ctx RenderContext) Frame { return s.view(ctx) }

// Quitting reports whether Effects.Quit has been called.
func (s *Session) Quitting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quitting
}

// UseHostServices installs the host's capabilities and optional callbacks.
// Call it before Start.
func (s *Session) UseHostServices(services HostServices) { s.services(services) }

// Capabilities reports the services installed through UseHostServices.
func (s *Session) Capabilities() Capabilities { return s.caps() }

// OnExit calls the app's cleanup hook once and cancels Effects work. Run owns
// this call for selected hosts; direct host tests may call it explicitly.
func (s *Session) OnExit() {
	s.exitOnce.Do(func() {
		defer s.cancel()
		if s.onExit != nil {
			s.onExit()
		}
	})
}

// Cancel stops Effects work without calling the app's OnExit hook. Snapshot
// uses it after rendering a frame.
func (s *Session) Cancel() { s.cancel() }

// Menus returns the app's current menu bar.
func (s *Session) Menus() []Menu { return s.menus() }

// ContextMenu returns the app's current context menu for target.
func (s *Session) ContextMenu(target string) []MenuItem { return s.contextMenu(target) }

// Windows returns the app's current secondary-window declarations.
func (s *Session) Windows() []Window { return s.windows() }

// HasNowPlaying reports whether the app provides a NowPlaying callback.
func (s *Session) HasNowPlaying() bool { return s.hasNowPlaying }

// NowPlaying returns the app's current media snapshot, if any.
func (s *Session) NowPlaying() *NowPlaying { return s.nowPlaying() }

// RunTerminal asks the built-in terminal host to drive the wrapped app. It is
// a host-plumbing hook used by window.MediaControls to preserve the terminal
// loop while installing native media services.
func (s *Session) RunTerminal(ctx context.Context, options ...Option) error {
	if s.runTerminal == nil {
		return ErrUnsupported
	}
	return s.runTerminal(ctx, options...)
}

func runWithHost[S any](ctx context.Context, app App[S], config RunConfig) (bool, error) {
	if config.err != nil {
		return true, config.err
	}
	if config.host == nil {
		return false, nil
	}
	session := NewSession(ctx, app)
	session.runTerminal = func(runCtx context.Context, options ...Option) error {
		runConfig := applyRunOptions(config.config, options...).config
		terminalApp := app
		terminalApp.OnExit = nil // the outer Run owns the session's OnExit call
		return runTerminalConfig(runCtx, terminalApp, runConfig)
	}
	defer session.OnExit()
	return true, config.host.Run(ctx, session)
}
