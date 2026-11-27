package flatte

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/theme"
)

type App[S any] struct {
	State  *S
	Init   func(*S, Effects[S])
	Handle func(*S, Event, Effects[S])
	View   func(*S, RenderContext) Frame
	Tracer Tracer
	// OnExit is called after the loop ends, on every exit path (clean quit,
	// context cancel, signal). Use it to persist state before the process
	// exits. It fires before terminal restoration.
	OnExit func(*S)
	// Menus, when set, is the menu bar for the state, read like View after
	// every change: a window shows it natively, and every host delivers a
	// key matching an item's Keys as its MenuEvent (a terminal shows no
	// bar).
	Menus func(*S) []Menu
	// ContextMenu, when set, is the menu for a right-click on the ID'd node
	// target ("" outside any): a window shows it as a native popup, a
	// terminal draws it over the frame. Return nil for no menu (the click
	// is then delivered as usual); a shown menu consumes the click, and the
	// choice arrives as a MenuEvent with Target set.
	ContextMenu func(s *S, target string) []MenuItem
	// Windows, when set, declares the app's secondary windows for the
	// state, read like View after every change: a window appears when
	// declared and closes when no longer declared. View draws each (see
	// RenderContext.Window). A desktop host opens real windows; a terminal
	// draws them over the main frame.
	Windows func(*S) []Window
	// NowPlaying reports the current media item and the commands available
	// for it. A native host publishes it to the system media controls; nil
	// opts out. The callback is read on the app's state loop after changes.
	NowPlaying func(*S) *NowPlaying
	// Panic, when set, catches a panic in the app's code instead of
	// letting it end the process: in Init, Handle, View, Menus, ContextMenu,
	// Windows, NowPlaying, a fold or other update as it applies, and the
	// off-loop work of Go, Latest, Stream and their Scope forms. Panic then
	// runs on the loop with what happened (show it, log it), and the app
	// goes on from its state as the panic left it. A View that panics
	// shows the last frame it drew (reported once until View draws
	// again); work that panics folds nothing. A panic in Panic itself is
	// printed to stderr and dropped. Nil: panics end the process, as
	// before.
	Panic func(*S, Panic, Effects[S])
}

// action is a one-shot terminal capability request, queued by Effects
// methods on the loop goroutine and drained by the loop before the next
// flush.
type action struct {
	write   string      // raw escape sequence to emit (clipboard OSC52 etc.)
	print   string      // content to insert into scrollback above the inline frame
	suspend bool        // release the terminal, suspend the process, restore
	exec    *execAction // release the terminal, run the command, restore
}

type execAction struct {
	cmd  *exec.Cmd
	done func(error) // delivers cmd.Run's error back to the app as a named update
}

type Effects[S any] struct {
	ctx     context.Context
	updates chan<- StateUpdate[S]
	// sendUpdate is used by deterministic harnesses that need an unbounded
	// in-memory queue. Production effects always use updates; keeping this
	// seam private prevents applications from replacing the loop transport.
	sendUpdate func(context.Context, StateUpdate[S]) bool
	quit       func()
	enqueue    func(action)
	latest     *latestRegistry
	dispatch   func(func())
	clock      Clock
	// setClipboard and readClipboard replace the terminal's OSC52 clipboard
	// for a host with its own clipboard service.
	setClipboard  func(string)
	readClipboard func()
	// dialogs shows dialogs; nil: the host has none.
	dialogs func(Dialog, func(DialogResult))
	// notify posts notifications; nil: terminal
	// escape sequences.
	notify func(Notification)
	// beep plays the alert sound; nil: the terminal's bell.
	beep func()
	// desktop routes background utility commands to a native host.
	desktop func(DesktopCommand)
	// caps is what the host supports.
	caps Capabilities
	// catch recovers panics in off-loop work for App.Panic (nil: none).
	catch *catcher
}

// Capabilities reports what the current host supports; the zero value on
// a zero Effects value or a harness that did not set any.
func (fx Effects[S]) Capabilities() Capabilities { return fx.caps }

func withHostCapabilities[S any](fx Effects[S], c Capabilities) Effects[S] {
	fx.caps = c
	return fx
}

// terminalCapabilities is what Run's terminal host supports under cfg.
func terminalCapabilities(cfg runConfig) Capabilities {
	return Capabilities{
		Exec: true, Suspend: true, Print: true,
		Alert: true, OpenFile: true, SaveFile: true,
		Notifications: true, Clipboard: true,
		Mouse: cfg.mouse != MouseModeNone,
		Quit:  true,
	}
}

// cancelReaderFactory is a seam for the input lifecycle tests. Keeping the
// substrate constructor behind this small interface also makes ownership
// explicit: every cancel reader created here is closed after its read loop
// has joined.
type cancelReader interface {
	io.ReadCloser
	Cancel() bool
}

var cancelReaderFactory = func(in io.Reader) (cancelReader, error) {
	return uv.NewCancelReader(in)
}

// outputSink is the one place where Run writes bytes to the real terminal
// sink. It latches the first failure so later best-effort cleanup writes do
// not hide the error that stopped normal work.
type outputSink struct {
	w   io.Writer
	err error
}

func (s *outputSink) write(p []byte) error {
	n, err := s.w.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	s.record(err)
	return err
}

func (s *outputSink) record(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func newEffects[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func()) Effects[S] {
	return Effects[S]{ctx: ctx, updates: updates, quit: quit, latest: newLatestRegistry()}
}

func withHostClipboard[S any](fx Effects[S], set func(text string), read func()) Effects[S] {
	fx.setClipboard, fx.readClipboard = set, read
	return fx
}

// spawn runs f on a goroutine by default, or through the injected dispatch
// (tests defer it to a controlled queue).
func (fx Effects[S]) spawn(in string, f func()) {
	if fx.catch != nil {
		g := f
		f = func() { fx.catch.run("work "+in, g) }
	}
	if fx.dispatch != nil {
		fx.dispatch(f)
		return
	}
	go f()
}

// Quit requests a clean exit of the Run loop. Safe on a zero Effects value.
// Call it from Init, Handle, or a fold — they all run on the loop goroutine;
// calling it from an app-spawned goroutine is a data race.
func (fx Effects[S]) Quit() {
	if fx.quit != nil {
		fx.quit()
	}
}

// SetClipboard writes text to the system clipboard: via OSC52 on the next
// flush in a terminal, directly in a window. Loop-goroutine-only, like
// Quit. Terminals without OSC52 support ignore it. Safe on a zero Effects
// value.
func (fx Effects[S]) SetClipboard(text string) {
	if fx.setClipboard != nil {
		fx.setClipboard(text)
		return
	}
	if fx.enqueue != nil {
		fx.enqueue(action{write: ansi.SetSystemClipboard(text)})
	}
}

// ReadClipboard asks for the clipboard content: the terminal via OSC52, or
// the window's pasteboard. A window or supporting terminal answers with a
// ClipboardEvent; unsupported terminals never answer — treat the event as
// optional and do not wait for it. Loop-goroutine-only, like Quit. Safe on
// a zero Effects value.
func (fx Effects[S]) ReadClipboard() {
	if fx.readClipboard != nil {
		fx.readClipboard()
		return
	}
	if fx.enqueue != nil {
		fx.enqueue(action{write: ansi.RequestSystemClipboard})
	}
}

// Suspend releases the terminal (cooked mode, main screen, cursor
// visible), suspends the process like the shell's Ctrl-Z would
// (SIGTSTP to the process group), and on resume (fg/SIGCONT) restores
// the terminal and repaints. On platforms without job control it is a
// release/restore round trip. The framework never binds a key to this —
// apps decide what (if anything) triggers it. Loop-goroutine-only, like
// Quit. No-op on window and browser hosts and on a zero Effects value.
func (fx Effects[S]) Suspend() {
	if fx.enqueue != nil {
		fx.enqueue(action{suspend: true})
	}
}

// Print writes s into the terminal's scrollback above the live frame, then
// repaints the frame below it — the Claude-Code "message stream + pinned input"
// model: emitted lines flow into the real terminal's history (which the user
// scrolls with the terminal/mouse) while the frame stays put at the bottom.
// Requires inline rendering (WithInline); in alt-screen mode it is a no-op,
// because the prepended lines would be overwritten by the next frame. The
// content is the app's to format (no trailing newline is added). Newlines in s
// produce multiple scrollback lines. Loop-goroutine-only, like Quit. Safe on a
// zero Effects value. No-op on window and browser hosts.
func (fx Effects[S]) Print(s string) {
	if fx.enqueue != nil {
		fx.enqueue(action{print: s})
	}
}

// Printf is Print with fmt.Sprintf formatting.
func (fx Effects[S]) Printf(format string, args ...any) {
	fx.Print(fmt.Sprintf(format, args...))
}

// Option configures Run behavior. Options are host-plumbing values; the
// standard With* functions are the application-facing constructors.
type Option interface {
	ConfigureRun(*RunConfig)
}

// RunConfig is opaque Run configuration supplied to Option implementations.
// Its exported methods are for host plumbing; apps should use With* options.
type RunConfig struct {
	config runConfig
	host   Host
	// defaultHost is selected only when no host-installing option is present.
	// Optional services such as terminal media controls use this slot so a
	// window host can take precedence without creating a host conflict.
	defaultHost Host
	err         error
}

// ErrMultipleHosts reports that more than one host-installing option was
// supplied to Run.
var ErrMultipleHosts = errors.New("flatte: multiple host options selected")

// UseHost selects a host driver for Run. Supplying multiple host-installing
// options makes Run return ErrMultipleHosts.
func (c *RunConfig) UseHost(host Host) {
	if c.host != nil {
		c.err = ErrMultipleHosts
		return
	}
	c.host = host
}

// UseFallbackHost selects a host only when no ordinary host option selects
// one. Repeating the same optional host is harmless; ordinary hosts still
// conflict with one another through UseHost.
func (c *RunConfig) UseFallbackHost(host Host) {
	if c.host == nil && c.defaultHost == nil {
		c.defaultHost = host
	}
}

func (c RunConfig) selectedHost() Host {
	if c.host != nil {
		return c.host
	}
	return c.defaultHost
}

type optionFunc func(*runConfig)

func (o optionFunc) ConfigureRun(c *RunConfig) { o(&c.config) }

func applyRunOptions(defaults runConfig, options ...Option) RunConfig {
	c := RunConfig{config: defaults}
	for _, option := range options {
		if option != nil {
			option.ConfigureRun(&c)
		}
	}
	return c
}

func nativeRunConfig() runConfig {
	return runConfig{
		input:          os.Stdin,
		output:         os.Stdout,
		defaultQuit:    true,
		bracketedPaste: true,
		suspendProcess: suspendProcess,
	}
}

// ErrInlineMouseUnsupported reports an invalid combination of options.
// Inline frames do not have a stable terminal origin, so mouse coordinates
// cannot be translated to frame-relative positions. Use alternate-screen
// rendering for mouse-enabled apps.
var ErrInlineMouseUnsupported = errors.New("flat: WithInline cannot be combined with mouse reporting")

// MouseMode selects which mouse events the terminal reports.
type MouseMode int

const (
	// MouseModeNone reports no mouse events (the default).
	MouseModeNone MouseMode = iota
	// MouseModeCellMotion reports clicks, releases, wheel, and drag motion.
	MouseModeCellMotion
	// MouseModeAllMotion additionally reports motion with no button held.
	MouseModeAllMotion
)

type runConfig struct {
	mediaCommands <-chan MediaEvent
	publishMedia  func(*NowPlaying)
	input         io.Reader
	output        io.Writer
	defaultQuit   bool
	quitRequests  bool
	// preciseScrolling: see WithPreciseScrolling.
	preciseScrolling bool
	bracketedPaste   bool
	mouse            MouseMode
	reportFocus      bool
	inline           bool
	suspendProcess   func() // test seam; defaults to the platform suspend
	theme            *theme.Theme
}

// WithTheme gives the app a design system: a native window draws layout's
// roles in it (following the system's light or dark appearance), a
// terminal draws its cells in true color from its dark palette, and
// RenderContext carries it to View. Without it, a window uses
// theme.Default() and a terminal its own colors.
func WithTheme(t theme.Theme) Option {
	return optionFunc(func(c *runConfig) { c.theme = &t })
}

// withSuspendProcess overrides the process-suspension call. Test seam:
// the real one stops the whole process group, which would stop the test
// runner too.
func withSuspendProcess(fn func()) Option {
	return optionFunc(func(c *runConfig) { c.suspendProcess = fn })
}

// withHostMedia connects a terminal run to a system media host. Commands are
// handled on the state loop. publish is called synchronously on that loop with
// an owned snapshot (or nil), and must return promptly. It receives nil again
// at shutdown. Closing commands disconnects it without ending Run.
func withHostMedia(commands <-chan MediaEvent, publish func(*NowPlaying)) Option {
	return optionFunc(func(c *runConfig) { c.mediaCommands, c.publishMedia = commands, publish })
}

// WithInput sets the event source. Default: os.Stdin.
func WithInput(in io.Reader) Option { return optionFunc(func(c *runConfig) { c.input = in }) }

// WithOutput sets the render sink. Default: os.Stdout.
func WithOutput(out io.Writer) Option { return optionFunc(func(c *runConfig) { c.output = out }) }

// WithoutDefaultQuit delivers Ctrl-C to the app instead of exiting the loop.
// The app must call fx.Quit(), close the input, or cancel the context to exit.
func WithoutDefaultQuit() Option { return optionFunc(func(c *runConfig) { c.defaultQuit = false }) }

// WithoutBracketedPaste disables bracketed paste mode. It is on by
// default: without it a paste arrives as a flood of individual key
// events instead of one PasteEvent.
func WithoutBracketedPaste() Option {
	return optionFunc(func(c *runConfig) { c.bracketedPaste = false })
}

// WithMouse enables terminal mouse reporting; events arrive as MouseEvent.
// MouseModeNone remains compatible with WithInline; reporting modes do not.
func WithMouse(mode MouseMode) Option { return optionFunc(func(c *runConfig) { c.mouse = mode }) }

// WithReportFocus enables focus reporting; terminal focus changes arrive
// as FocusEvent. Some terminals and multiplexers need configuration to
// report focus (tmux: focus-events).
func WithReportFocus() Option { return optionFunc(func(c *runConfig) { c.reportFocus = true }) }

// WithInline renders below the shell prompt instead of in the alternate
// screen: the frame occupies exactly its own lines, the terminal's
// scrollback stays intact, and on exit the final frame remains visible
// with the prompt landing below it. Mouse reporting cannot be enabled because
// an inline frame has no stable terminal origin; Run returns
// ErrInlineMouseUnsupported for that combination.
func WithInline() Option { return optionFunc(func(c *runConfig) { c.inline = true }) }

func runTerminalConfig[S any](ctx context.Context, app App[S], cfg runConfig) (runErr error) {
	if app.State == nil {
		panic("flat: App.State is nil")
	}
	if app.View == nil {
		panic("flat: App.View is nil")
	}
	if cfg.inline && cfg.mouse != MouseModeNone {
		return ErrInlineMouseUnsupported
	}

	run := &terminalRuntime{cfg: cfg}
	defer func() {
		if runErr == nil && run.inputCloseErr != nil {
			runErr = run.inputCloseErr
		}
	}()
	if cfg.publishMedia != nil {
		defer cfg.publishMedia(nil)
	}
	defer func() { runErr = run.teardown(runErr) }()

	if err := setupTerminalOptions(run, ctx, app); err != nil {
		return err
	}
	run.startInputPipeline()
	return run.runTerminalEventLoop()
}

// inputBuffer is how many translated input events wait for the loop.
const inputBuffer = 64

// maxInputBatch bounds the input events handled between two draws, so a
// stream of input cannot hold the screen still.
const maxInputBatch = 256

// setModes returns the terminal-mode escapes for the configured
// capabilities; resetModes returns their inverses. Mouse reset clears
// all mouse modes regardless of which one was set.
func setModes(cfg runConfig) string {
	var modes string
	if cfg.bracketedPaste {
		modes += ansi.SetModeBracketedPaste
	}
	if cfg.reportFocus {
		modes += ansi.SetModeFocusEvent
	}
	switch cfg.mouse {
	case MouseModeCellMotion:
		modes += ansi.SetModeMouseButtonEvent + ansi.SetModeMouseExtSgr
	case MouseModeAllMotion:
		modes += ansi.SetModeMouseAnyEvent + ansi.SetModeMouseExtSgr
	}
	return modes
}

func resetModes(cfg runConfig) string {
	var modes string
	if cfg.bracketedPaste {
		modes += ansi.ResetModeBracketedPaste
	}
	if cfg.reportFocus {
		modes += ansi.ResetModeFocusEvent
	}
	if cfg.mouse != MouseModeNone {
		modes += ansi.ResetModeMouseButtonEvent +
			ansi.ResetModeMouseAnyEvent +
			ansi.ResetModeMouseExtSgr
	}
	return modes
}

var detectRendererColorProfile = colorprofile.Detect

func newTerminalRenderer(renderBuffer, output io.Writer, env []string, inline bool) *uv.TerminalRenderer {
	renderer := uv.NewTerminalRenderer(renderBuffer, env)
	renderer.SetColorProfile(detectRendererColorProfile(output, env))
	if inline {
		renderer.SetRelativeCursor(true)
	}
	return renderer
}

func cursorShapeCode(style *CursorStyle) (int, bool) {
	if style == nil {
		return 0, false
	}
	switch style.Shape {
	case CursorShapeBlock:
		if style.Blink {
			return 1, true
		}
		return 2, true
	case CursorShapeUnderline:
		if style.Blink {
			return 3, true
		}
		return 4, true
	case CursorShapeBar:
		if style.Blink {
			return 5, true
		}
		return 6, true
	default:
		return 0, false
	}
}

func cursorColorToken(style *CursorStyle) string {
	if style == nil || style.Color == nil {
		return ""
	}
	r, g, b, _ := style.Color.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

type inputResult struct {
	event Event
	err   error
}

// readInput streams substrate events from the reader and translates them
// onto the closed event set. The results channel closes when the stream
// ends (input close maps to a clean end, terminal read errors are delivered
// as the final result). The done channel closes only once no goroutine can
// touch the reader anymore — Run waits on it before returning.
func readInput(ctx context.Context, in io.Reader) (<-chan inputResult, <-chan struct{}) {
	// Buffered, so input read while a frame is drawn waits ready for the
	// loop to take in one batch (see maxInputBatch).
	results := make(chan inputResult, inputBuffer)
	done := make(chan struct{})
	rawEvents := make(chan uv.Event)
	reader := uv.NewTerminalReader(in, os.Getenv("TERM"))
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- reader.StreamEvents(ctx, rawEvents)
	}()
	go func() {
		// Publish completion before closing results. A caller that observes
		// input EOF can then close an already-finished CancelReader even when
		// its Cancel method correctly reports that no read remains pending.
		defer close(results)
		defer close(done)
		for {
			select {
			case raw := <-rawEvents:
				event, ok := translateEvent(raw)
				if !ok {
					continue
				}
				select {
				case results <- inputResult{event: event}:
				case <-ctx.Done():
					drainStream(rawEvents, streamErr)
					return
				}
			case err := <-streamErr:
				if err != nil && !errors.Is(err, context.Canceled) {
					select {
					case results <- inputResult{err: err}:
					case <-ctx.Done():
					}
				}
				return
			case <-ctx.Done():
				drainStream(rawEvents, streamErr)
				return
			}
		}
	}()
	return results, done
}

// drainStream discards remaining substrate events until StreamEvents
// returns. On cancellation StreamEvents flushes pending events into its
// channel before exiting, so someone must keep receiving or it never
// finishes — and its internal goroutine keeps the input reader pinned.
func drainStream(rawEvents <-chan uv.Event, streamErr <-chan error) {
	for {
		select {
		case <-rawEvents:
		case <-streamErr: //nolint:errcheck // the stream's end is all it waits for
			return
		}
	}
}
