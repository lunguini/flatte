package flatte

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/lunguini/flatte/flatui/layout"
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
	// for a host with its own (WithHostClipboard).
	setClipboard  func(string)
	readClipboard func()
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

// NewHarnessEffects builds an Effects for a host other than Run: dispatch
// controls how async bodies are spawned (nil = real goroutine) and clock
// controls time-based effects (nil = real clock). flatest uses it for
// deterministic tests and the window host for real ones; it is host
// plumbing, not application API. Terminal-only effects (Print, Suspend,
// Exec, clipboard) are no-ops on it.
func NewHarnessEffects[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func(), dispatch func(func()), clock Clock) Effects[S] {
	fx := newEffects[S](ctx, updates, quit)
	fx.dispatch = dispatch
	fx.clock = clock
	return fx
}

// WithHostClipboard returns fx with SetClipboard and ReadClipboard routed to
// set and read: a host with a system clipboard of its own (a window) writes
// it and answers read with a ClipboardEvent. Like NewHarnessEffects it is
// host plumbing, not application API.
func WithHostClipboard[S any](fx Effects[S], set func(text string), read func()) Effects[S] {
	fx.setClipboard, fx.readClipboard = set, read
	return fx
}

// NewHarnessEffectsQueue builds a deterministic test Effects value backed by
// an enqueue callback instead of a channel. It is intended for synchronous
// harnesses such as flatest, where a burst of updates must not block a fake
// clock or a controlled Stream worker on an arbitrary channel capacity.
func NewHarnessEffectsQueue[S any](ctx context.Context, enqueue func(StateUpdate[S]) bool, quit func(), dispatch func(func()), clock Clock) Effects[S] {
	fx := newEffects[S](ctx, nil, quit)
	fx.sendUpdate = func(sendCtx context.Context, update StateUpdate[S]) bool {
		if sendCtx == nil {
			sendCtx = context.Background()
		}
		select {
		case <-sendCtx.Done():
			return false
		default:
		}
		if enqueue == nil {
			return false
		}
		return enqueue(update)
	}
	fx.dispatch = dispatch
	fx.clock = clock
	return fx
}

// spawn runs f on a goroutine by default, or through the injected dispatch
// (tests defer it to a controlled queue).
func (fx Effects[S]) spawn(f func()) {
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
// Quit. Safe on a zero Effects value.
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
// zero Effects value.
func (fx Effects[S]) Print(s string) {
	if fx.enqueue != nil {
		fx.enqueue(action{print: s})
	}
}

// Printf is Print with fmt.Sprintf formatting.
func (fx Effects[S]) Printf(format string, args ...any) {
	fx.Print(fmt.Sprintf(format, args...))
}

// Option configures Run behaviour.
type Option func(*runConfig)

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
	input          io.Reader
	output         io.Writer
	defaultQuit    bool
	bracketedPaste bool
	mouse          MouseMode
	reportFocus    bool
	inline         bool
	suspendProcess func() // test seam; defaults to the platform suspend
}

// withSuspendProcess overrides the process-suspension call. Test seam:
// the real one stops the whole process group, which would stop the test
// runner too.
func withSuspendProcess(fn func()) Option {
	return func(c *runConfig) { c.suspendProcess = fn }
}

// WithInput sets the event source. Default: os.Stdin.
func WithInput(in io.Reader) Option { return func(c *runConfig) { c.input = in } }

// WithOutput sets the render sink. Default: os.Stdout.
func WithOutput(out io.Writer) Option { return func(c *runConfig) { c.output = out } }

// WithoutDefaultQuit delivers Ctrl-C to the app instead of exiting the loop.
// The app must call fx.Quit(), close the input, or cancel the context to exit.
func WithoutDefaultQuit() Option { return func(c *runConfig) { c.defaultQuit = false } }

// WithoutBracketedPaste disables bracketed paste mode. It is on by
// default: without it a paste arrives as a flood of individual key
// events instead of one PasteEvent.
func WithoutBracketedPaste() Option { return func(c *runConfig) { c.bracketedPaste = false } }

// WithMouse enables terminal mouse reporting; events arrive as MouseEvent.
// MouseModeNone remains compatible with WithInline; reporting modes do not.
func WithMouse(mode MouseMode) Option { return func(c *runConfig) { c.mouse = mode } }

// WithReportFocus enables focus reporting; terminal focus changes arrive
// as FocusEvent. Some terminals and multiplexers need configuration to
// report focus (tmux: focus-events).
func WithReportFocus() Option { return func(c *runConfig) { c.reportFocus = true } }

// WithInline renders below the shell prompt instead of in the alternate
// screen: the frame occupies exactly its own lines, the terminal's
// scrollback stays intact, and on exit the final frame remains visible
// with the prompt landing below it. Mouse reporting cannot be enabled because
// an inline frame has no stable terminal origin; Run returns
// ErrInlineMouseUnsupported for that combination.
func WithInline() Option { return func(c *runConfig) { c.inline = true } }

// Run owns the terminal loop until the app quits, input ends, or ctx is
// cancelled. Those three shutdown paths are clean and return nil after OnExit
// and terminal restoration; cancellation is intentionally not surfaced as
// context.Canceled. Input, rendering, output, and cleanup failures are returned.
func Run[S any](ctx context.Context, app App[S], opts ...Option) (runErr error) {
	if app.State == nil {
		panic("flat: App.State is nil")
	}
	if app.View == nil {
		panic("flat: App.View is nil")
	}
	var inputCloseErr error
	// Input cleanup happens before renderer cleanup, but a close failure is a
	// cleanup error. Let an earlier run or output error win, and only expose it
	// when the run otherwise completed successfully.
	defer func() {
		if runErr == nil && inputCloseErr != nil {
			runErr = inputCloseErr
		}
	}()

	cfg := runConfig{
		input:          os.Stdin,
		output:         os.Stdout,
		defaultQuit:    true,
		bracketedPaste: true,
		suspendProcess: suspendProcess,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.inline && cfg.mouse != MouseModeNone {
		return ErrInlineMouseUnsupported
	}
	in, out := cfg.input, cfg.output
	sink := &outputSink{w: out}

	// Raw mode enter/restore are reusable: suspend and exec hand the
	// terminal back to the shell or a subprocess and re-enter afterwards.
	// The state captured by the FIRST MakeRaw is the terminal's original
	// state and is what every restore returns to.
	var rawFile *os.File
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		rawFile = file
	}
	var originalTermState *term.State
	enterRaw := func() error {
		if rawFile == nil {
			return nil
		}
		state, err := term.MakeRaw(int(rawFile.Fd()))
		if err != nil {
			return err
		}
		if originalTermState == nil {
			originalTermState = state
		}
		return nil
	}
	restoreRaw := func() {
		if rawFile != nil && originalTermState != nil {
			_ = term.Restore(int(rawFile.Fd()), originalTermState)
		}
	}
	if err := enterRaw(); err != nil {
		return err
	}
	defer restoreRaw()

	renderOut := &bytes.Buffer{}
	newRenderer := func() *uv.TerminalRenderer {
		return newTerminalRenderer(renderOut, out, os.Environ(), cfg.inline)
	}
	renderer := newRenderer()
	flushRenderer := func() error {
		err := renderer.Flush()
		sink.record(err)
		return err
	}
	var lastFrame Frame
	// lastComp is the composition of the frame on screen; mouse events are
	// hit-tested against it so Target matches what the user clicked.
	var lastComp layout.Composition
	// menus routes menu shortcuts and draws context menus (see MenuHost).
	var menus MenuHost
	// lastLayout is the rect set last delivered as a LayoutEvent.
	var lastLayout map[string]layout.Rect
	drew := false
	cursorShown := false
	currentCursorShape := -1
	currentCursorColor := ""
	var screen uv.ScreenBuffer
	var screenWidth, screenHeight int
	forceRepaint := false
	var pending []action
	applyCursorStyle := func(style *CursorStyle) {
		if code, ok := cursorShapeCode(style); ok {
			if currentCursorShape != code {
				_, _ = renderer.WriteString(fmt.Sprintf("\x1b[%d q", code))
				currentCursorShape = code
			}
		} else if currentCursorShape != -1 {
			_, _ = renderer.WriteString("\x1b[0 q")
			currentCursorShape = -1
		}
		if token := cursorColorToken(style); token != "" {
			if currentCursorColor != token {
				_, _ = renderer.WriteString("\x1b]12;" + token + "\x07")
				currentCursorColor = token
			}
		} else if currentCursorColor != "" {
			_, _ = renderer.WriteString("\x1b]112\x07")
			currentCursorColor = ""
		}
	}
	resetCursorStyle := func() {
		applyCursorStyle(nil)
	}

	// enterRenderer applies the renderer-side entry state: alt screen
	// (unless inline), cursor hidden, terminal modes. Alt-screen entry must
	// go through the renderer, not a raw write: EnterAltScreen also flips
	// the renderer's fullscreen + absolute-cursor flags, or its inline-mode
	// cursor/scroll optimizations desync the screen (frames drift one row
	// off). Inline renderers explicitly enable relative cursor movement so
	// fresh paints start at the current terminal row instead of absolute home.
	// Reused at startup and after a suspend/exec hands the terminal back.
	enterRenderer := func() {
		if !cfg.inline {
			renderer.EnterAltScreen()
		}
		currentCursorShape = -1
		currentCursorColor = ""
		_, _ = renderer.WriteString("\x1b[?25l") // hide cursor (terminals may reset it on alt-screen entry)
		_, _ = renderer.WriteString(setModes(cfg))
		cursorShown = false
	}
	enterRenderer()
	// drainActions emits only the write-type actions (clipboard OSC52). It
	// runs in the cleanup defer so a write enqueued on the quit iteration
	// still reaches the terminal. Suspend/exec actions are NOT run here —
	// they re-enter the loop machinery (input restart, repaint) and only
	// make sense while the loop is live; processActions handles those.
	drainActions := func() {
		for _, a := range pending {
			if a.write != "" {
				_, _ = renderer.WriteString(a.write)
			}
		}
		pending = pending[:0]
	}
	defer func() {
		drainActions() // actions enqueued on the quit iteration still emit
		if lastFrame.Title != "" {
			_, _ = renderer.WriteString(ansi.SetWindowTitle(""))
		}
		resetCursorStyle()
		_, _ = renderer.WriteString(resetModes(cfg))
		_, _ = renderer.WriteString("\x1b[?25h")
		if cfg.inline {
			// The final frame stays in the terminal; park the cursor below
			// it so the shell prompt does not overwrite the last line.
			if drew && screenHeight > 0 {
				renderer.MoveTo(0, screenHeight-1)
			}
			_, _ = renderer.WriteString("\r\n")
		} else {
			renderer.ExitAltScreen()
		}
		if err := flushRenderer(); err != nil && runErr == nil {
			runErr = err
		}
		if renderOut.Len() > 0 {
			data := append([]byte(nil), renderOut.Bytes()...)
			renderOut.Reset()
			_ = sink.write(data)
		}
		if runErr == nil {
			runErr = sink.err
		}
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The input pipeline is restartable: suspend and exec stop it before
	// handing the terminal away (the subprocess must own stdin) and start
	// a fresh incarnation afterwards. stop blocks until no goroutine can
	// touch the input source anymore (a Close racing a blocked read is a
	// data race) — but only when the read was actually interrupted.
	//
	// Not every input source has an interruptible read. The substrate falls
	// back to an uncancelable reader when it cannot do better (a pipe on
	// Windows, notably, which is what the tests and any piped stdin use),
	// and there Cancel reports false and leaves the pending Read parked in
	// the syscall until the source sends or closes. Such a pipeline cannot
	// be torn down at all: cancelling its context would stop event delivery
	// while the goroutine stayed parked on the source, and a replacement
	// pipeline would then race the parked one for the same bytes — input
	// after a resume would go to a reader nobody is listening to.
	//
	// So stop reports whether it actually tore the pipeline down, and the
	// caller restarts only what really stopped. An uncancelable pipeline is
	// left running across the release/restore cycle and keeps delivering to
	// the same channel. The cost is that a subprocess does not exclusively
	// own stdin during exec/suspend on such a source — unavoidable without
	// hanging, and not reachable on a real console, where the substrate has
	// a cancelable reader and Cancel reports true.
	type inputPipeline struct {
		events   <-chan inputResult
		stop     func() bool
		finish   func()
		finalize func()
	}
	startInput := func() inputPipeline {
		inputCtx, cancelCtx := context.WithCancel(runCtx)
		eventSource := io.Reader(in)
		cancelRead := func() bool { return false }
		var cancelable cancelReader
		if candidate, err := cancelReaderFactory(in); err == nil {
			cancelable = candidate
			eventSource = cancelable
			cancelRead = cancelable.Cancel
		}
		events, done := readInput(inputCtx, eventSource)
		stopped := false
		var closeOnce sync.Once
		var closeErr error
		closeCancelable := func() error {
			closeOnce.Do(func() {
				if cancelable != nil {
					closeErr = cancelable.Close()
				}
			})
			return closeErr
		}
		recordClose := func() {
			if err := closeCancelable(); err != nil && inputCloseErr == nil {
				inputCloseErr = err
			}
		}
		finish := func() {
			if stopped {
				return
			}
			cancelCtx()
			<-done
			recordClose()
			stopped = true
		}
		var stop func() bool
		stop = func() bool {
			if stopped {
				return true
			}
			// A reader that reached EOF on its own is already safe to close;
			// Cancel may legitimately return false once no read is pending.
			select {
			case <-done:
				cancelCtx()
				recordClose()
				stopped = true
				return true
			default:
			}
			// Cancel first: it is the only way to learn whether this
			// pipeline can be stopped, and the answer decides whether the
			// context may be cancelled at all. The interrupted read makes
			// the stream fail, and cancelCtx releases the goroutine if it
			// is mid-send with that error.
			if !cancelRead() {
				return false
			}
			cancelCtx()
			<-done
			recordClose()
			stopped = true
			return true
		}
		finalize := func() {
			if stop() {
				return
			}
			// Final Run shutdown differs from suspend/exec: this pipeline will
			// never be reused. Stop delivering events now, then release the
			// CancelReader asynchronously once its uncancelable read eventually
			// completes. A late close error cannot be returned after Run exits.
			cancelCtx()
			go func() {
				<-done
				_ = closeCancelable()
			}()
		}
		return inputPipeline{events: events, stop: stop, finish: finish, finalize: finalize}
	}
	pipe := startInput()
	// inputStopped tracks whether the last stop actually tore the pipeline
	// down, so restoreTerminal knows whether to start a fresh one or keep
	// using the surviving one.
	inputStopped := false
	defer func() {
		cancel()
		pipe.finalize()
	}()
	resize, stopResize := notifyResize()
	defer stopResize()
	defer func() {
		if app.OnExit != nil {
			app.OnExit(app.State)
		}
	}()
	updates := make(chan StateUpdate[S], 64)
	quitRequested := false
	effects := newEffects(runCtx, updates, func() { quitRequested = true })
	effects.enqueue = func(a action) { pending = append(pending, a) }
	if app.Init != nil {
		app.Init(app.State, effects)
	}
	if quitRequested {
		return nil
	}
	if app.Tracer == nil {
		app.Tracer = NoopTracer{}
	}

	width, height := terminalSize(out)
	initialSize := Event(ResizeEvent{Width: width, Height: height})
	app.Tracer.Event(initialSize)
	if app.Handle != nil {
		app.Handle(app.State, initialSize, effects)
	}
	if quitRequested {
		return nil
	}

	writePendingOutput := func(synchronized bool) error {
		if renderOut.Len() == 0 {
			return nil // nothing to write, not even markers
		}
		prefix, suffix := "", ""
		if synchronized {
			prefix, suffix = "\x1b[?2026h", "\x1b[?2026l"
		}
		data := make([]byte, 0, len(prefix)+renderOut.Len()+len(suffix))
		data = append(data, prefix...)
		data = append(data, renderOut.Bytes()...)
		data = append(data, suffix...)
		renderOut.Reset()
		return sink.write(data)
	}
	writeFrameOutput := func() {
		_ = writePendingOutput(true)
	}

	draw := func() {
		if sink.err != nil {
			return // terminal output failed; defer handles best-effort cleanup
		}
		renderCtx := RenderContextFor(out)
		_, terminalHeight := terminalSize(out)
		var handleLayout func(LayoutEvent)
		if app.Handle != nil {
			handleLayout = func(ev LayoutEvent) {
				if quitRequested {
					return
				}
				app.Tracer.Event(ev)
				app.Handle(app.State, ev, effects)
			}
		}
		frame, comp, delivered := settleFrame(
			func() Frame { return app.View(app.State, renderCtx) },
			handleLayout, lastLayout, renderCtx.Width,
			func(root layout.Node) int { return rootHeight(root, terminalHeight, cfg.inline) },
		)
		lastLayout = delivered
		if quitRequested {
			return // a LayoutEvent handler quit: paint nothing more
		}
		lastComp = comp
		frame = menus.Paint(frame)
		if drew && !forceRepaint && framesEqual(frame, lastFrame) && screenWidth == renderCtx.Width {
			// Identical frame: only queued one-shot actions (if any) go out.
			if err := flushRenderer(); err == nil {
				writeFrameOutput()
			}
			return
		}
		if frame.Title != lastFrame.Title {
			_, _ = renderer.WriteString(ansi.SetWindowTitle(frame.Title))
		}
		styled := uv.NewStyledString(frame.Content)
		// Alt-screen frames fill the terminal (so a shrunk frame clears the
		// rows it vacated); inline frames own exactly their own lines and
		// must not pad to the terminal height, or they would scroll the
		// shell's scrollback away.
		height := styled.Height()
		if !cfg.inline {
			_, terminalHeight := terminalSize(out)
			height = max(height, terminalHeight)
		}
		if screenWidth != renderCtx.Width || screenHeight != height {
			screen = uv.NewScreenBuffer(renderCtx.Width, height)
			renderer.Resize(renderCtx.Width, height)
			screenWidth, screenHeight = renderCtx.Width, height
		}
		if forceRepaint {
			// Terminals can scroll or clobber alt-screen content during a
			// resize, voiding the renderer's belief about what is on screen:
			// repaint everything from home.
			renderer.Erase()
			forceRepaint = false
		}
		if cursorShown && frame.Cursor == nil {
			// Hide before the diff writes so the cursor never lingers on
			// stale cells.
			resetCursorStyle()
			_, _ = renderer.WriteString("\x1b[?25l")
			cursorShown = false
		}
		screen.Clear()
		styled.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		if frame.Cursor != nil {
			applyCursorStyle(frame.Cursor.Style)
			// MoveTo must come after Render: rendering moves the cursor.
			renderer.MoveTo(frame.Cursor.X, frame.Cursor.Y)
			if !cursorShown {
				_, _ = renderer.WriteString("\x1b[?25h")
				cursorShown = true
			}
		}
		if err := flushRenderer(); err != nil {
			return
		}
		lastFrame, drew = frame, true
		writeFrameOutput()
	}

	// releaseTerminal hands the terminal back (cooked mode, main screen or
	// below the inline frame, cursor visible) and fully stops the input
	// pipeline so a subprocess or the shell owns stdin. restoreTerminal is
	// its inverse plus a forced repaint and a fresh ResizeEvent — the
	// terminal may have been resized while we were away, and no SIGWINCH
	// was delivered to us.
	releaseTerminal := func() {
		inputStopped = pipe.stop()
		resetCursorStyle()
		_, _ = renderer.WriteString(resetModes(cfg))
		_, _ = renderer.WriteString("\x1b[?25h")
		if cfg.inline {
			// Leave the frame in place; park the cursor below it so the
			// shell or subprocess starts on a fresh line.
			if drew && screenHeight > 0 {
				renderer.MoveTo(0, screenHeight-1)
			}
			_, _ = renderer.WriteString("\r\n")
		} else {
			renderer.ExitAltScreen()
		}
		if err := flushRenderer(); err == nil {
			_ = writePendingOutput(false)
		}
		restoreRaw()
	}
	restoreTerminal := func() {
		_ = enterRaw()
		if cfg.inline {
			// Foreign output (a subprocess, or the shell during suspend) has
			// moved the real cursor, so the renderer's screen model is now
			// stale. Recreate it: a fresh inline renderer draws the frame at
			// the current cursor, below whatever was printed. forceRepaint +
			// the zeroed buffer dimensions rebuild everything from scratch.
			renderer = newRenderer()
			screenWidth, screenHeight = 0, 0
		}
		enterRenderer()
		// Only a pipeline that really stopped needs replacing. A surviving
		// one is still reading the same source into the same channel; a
		// second reader on top of it would steal the bytes.
		if inputStopped {
			pipe = startInput()
			inputStopped = false
		}
		forceRepaint = true
		width, height := terminalSize(out)
		resizeEvent := Event(ResizeEvent{Width: width, Height: height})
		app.Tracer.Event(resizeEvent)
		if app.Handle != nil {
			app.Handle(app.State, resizeEvent, effects)
		}
	}

	// insertAbove emits unmanaged lines into the terminal's scrollback above the
	// inline frame — the mechanism behind fx.Print. It is deliberately
	// position-independent: it does NOT assume the frame sits at the bottom of the
	// viewport. (It usually does not — a freshly-started inline app renders below
	// the shell prompt with empty rows beneath it. The earlier scroll-and-insert
	// approaches — uv's PrependString and Bubble Tea v2's cursedRenderer.insertAbove
	// — both rely on a line feed at the frame's bottom row actually scrolling the
	// screen, which only happens when that row IS the screen's bottom; otherwise
	// the frame just walks down the screen on every print, corrupting it.)
	//
	// Instead, mirror Bubble Tea v1's standard renderer and flat's own
	// exec/suspend recovery: move to the frame's top-left, erase it (and anything
	// below) from the screen, write the unmanaged lines there as plain output, then
	// rebuild the frame fresh below them. Earlier scrollback above the frame is
	// untouched; the lines flow up into history naturally as the frame is repainted
	// lower each time and the viewport eventually scrolls. Recreating the renderer
	// with zeroed dimensions follows the same fresh-render principle as
	// restoreTerminal, and it is the position-independent way to keep print correct
	// wherever the frame sits — uv's diff model cannot relocate the frame on its own.
	insertAbove := func(str string) {
		if str == "" {
			return
		}
		// Flush any buffered renderer output first so the inserted lines land in
		// order relative to a pending frame.
		if err := flushRenderer(); err != nil {
			return
		}
		if err := writePendingOutput(false); err != nil {
			return
		}
		_, y := renderer.Position()
		var sb strings.Builder
		sb.WriteByte('\r')
		if y > 0 {
			sb.WriteString(ansi.CursorUp(y)) // to the frame's top row
		}
		sb.WriteString(ansi.EraseScreenBelow) // wipe the old frame and rows below it
		for _, line := range strings.Split(str, "\n") {
			sb.WriteString(line)
			sb.WriteString(ansi.EraseLineRight)
			sb.WriteString("\r\n")
		}
		_ = sink.write([]byte(sb.String()))
		// Rebuild the frame fresh starting at the current cursor row (below the
		// inserted lines). A fresh inline renderer treats that row as its origin.
		// Zero dimensions force the next draw through the full render path; do not
		// set forceRepaint, because renderer.Erase would erase the line we just
		// emitted before repainting the frame.
		renderer = newRenderer()
		screenWidth, screenHeight = 0, 0
		forceRepaint = false
	}

	// processActions drains the one-shot queue before each draw. Suspend
	// runs here, not in drainActions: it re-enters the loop machinery
	// (input restart, repaint) and must never run mid-render. The outer
	// loop re-checks because the post-restore resize Handle may enqueue.
	processActions := func() {
		for len(pending) > 0 {
			if sink.err != nil {
				return
			}
			queued := pending
			pending = nil
			for _, a := range queued {
				if sink.err != nil {
					return
				}
				switch {
				case a.suspend:
					releaseTerminal()
					if sink.err != nil {
						return
					}
					cfg.suspendProcess()
					restoreTerminal()
				case a.exec != nil:
					releaseTerminal()
					if sink.err != nil {
						return
					}
					cmd := a.exec.cmd
					if cmd.Stdin == nil {
						cmd.Stdin = in
					}
					if cmd.Stdout == nil {
						cmd.Stdout = out
					}
					if cmd.Stderr == nil {
						cmd.Stderr = os.Stderr
					}
					err := cmd.Run() // blocking: the TUI is paused while it runs
					restoreTerminal()
					a.exec.done(err)
				case a.write != "":
					_, _ = renderer.WriteString(a.write)
				case a.print != "":
					// Emit into scrollback above the inline frame and repaint the
					// frame below. insertAbove recreates the renderer and zeroes the
					// dimensions so the next draw rebuilds the frame at its new
					// position without erasing the printed line. Only meaningful
					// inline and after a frame exists; in alt-screen the lines would
					// be overwritten, so skip.
					if cfg.inline && drew {
						insertAbove(a.print)
					}
				}
				if sink.err != nil {
					return
				}
			}
		}
	}

	// render draws, then runs any actions a LayoutEvent handler queued while
	// the frame settled and redraws once, so they are not stranded until an
	// unrelated later event. A further queue from that redraw waits for the
	// next loop turn, keeping render bounded.
	render := func() {
		draw()
		if sink.err != nil || quitRequested || len(pending) == 0 {
			return
		}
		processActions()
		if sink.err == nil && !quitRequested {
			draw()
		}
	}

	processActions() // Init and the initial resize Handle may have enqueued
	if sink.err != nil {
		return sink.err
	}
	if quitRequested {
		return nil
	}
	render()
	if sink.err != nil {
		return sink.err
	}
	if quitRequested {
		return nil
	}
	for {
		select {
		case <-runCtx.Done():
			return nil
		case result, ok := <-pipe.events:
			if !ok {
				return nil
			}
			if result.err != nil {
				// The terminal reader has already returned, but the dispatcher
				// may still be unwinding after its unbuffered error send. Join it
				// before closing the owned CancelReader.
				pipe.finish()
				return result.err
			}
			if _, ok := result.event.(ResizeEvent); ok {
				// A resize can arrive through the input stream (notably the
				// Windows console path), rather than SIGWINCH. Both paths
				// invalidate the renderer's screen model and require a full
				// repaint, even when the frame content is unchanged.
				forceRepaint = true
			}
			if mouse, ok := result.event.(MouseEvent); ok {
				mouse.Target, mouse.TargetRect = lastComp.HitTest(mouse.X, mouse.Y)
				mouse.Caret, mouse.HasCaret = lastComp.CaretAt(mouse.X, mouse.Y)
				result.event = mouse
			}
			event, deliver := RouteMenus(&menus, app, result.event, screenWidth, screenHeight)
			if !deliver {
				render()
				if sink.err != nil {
					return sink.err
				}
				continue
			}
			result.event = event
			// Trace before any exit decision so the recorder sees the
			// exit-causing event too (incl. a default-quit Ctrl-C).
			app.Tracer.Event(result.event)
			if key, isKey := result.event.(KeyEvent); isKey && key.Key == KeyCtrlC && cfg.defaultQuit {
				return nil
			}
			if app.Handle != nil {
				app.Handle(app.State, result.event, effects)
			}
			// quitRequested flips synchronously inside Init, Handle, or a
			// fold — all on the loop goroutine. The updates path is checked
			// after drainUpdates below.
			if quitRequested {
				return nil
			}
		case <-resize:
			forceRepaint = true
			width, height := terminalSize(out)
			resizeEvent := Event(ResizeEvent{Width: width, Height: height})
			app.Tracer.Event(resizeEvent)
			if app.Handle != nil {
				app.Handle(app.State, resizeEvent, effects)
			}
			if quitRequested {
				return nil
			}
		case update := <-updates:
			ApplyUpdate(app.State, app.Tracer, update)
		}
		drainUpdates(app, updates)
		if quitRequested {
			return nil
		}
		processActions()
		if sink.err != nil {
			return sink.err
		}
		if quitRequested {
			return nil
		}
		render()
		if sink.err != nil {
			return sink.err
		}
		if quitRequested {
			return nil
		}
	}
}

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

// drainUpdates applies at most len(updates) pending updates without blocking,
// so a burst of async results produces one redraw instead of one per update.
// Capping at the snapshot length bounds latency to one frame's worth of work
// even if a fast producer keeps the channel continuously non-empty.
func drainUpdates[S any](app App[S], updates <-chan StateUpdate[S]) {
	for n := len(updates); n > 0; n-- {
		select {
		case update := <-updates:
			ApplyUpdate(app.State, app.Tracer, update)
		default:
			return
		}
	}
}

var detectRendererColorProfile = colorprofile.Detect

func newTerminalRenderer(renderBuffer io.Writer, output io.Writer, env []string, inline bool) *uv.TerminalRenderer {
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
	results := make(chan inputResult)
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
		case <-streamErr:
			return
		}
	}
}
