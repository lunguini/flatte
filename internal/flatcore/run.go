package flatcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type App[S any] struct {
	State  *S
	Init   func(*S, Effects[S])
	Handle func(*S, Event, Effects[S])
	View   func(*S, RenderContext) Frame
	Tracer Tracer
}

// action is a one-shot terminal capability request, queued by Effects
// methods on the loop goroutine and drained by the loop before the next
// flush.
type action struct {
	write   string      // raw escape sequence to emit (clipboard OSC52 etc.)
	suspend bool        // release the terminal, suspend the process, restore
	exec    *execAction // release the terminal, run the command, restore
}

type execAction struct {
	cmd  *exec.Cmd
	done func(error) // delivers cmd.Run's error back to the app as a named update
}

type Effects[S any] struct {
	Context  context.Context
	Updates  chan<- StateUpdate[S]
	quit     func()
	enqueue  func(action)
	latest   *latestRegistry
	dispatch func(func())
	clock    Clock
}

// NewEffects builds an Effects value with an observable quit callback.
// Run uses it internally; tests use it to assert quit requests.
func NewEffects[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func()) Effects[S] {
	return Effects[S]{Context: ctx, Updates: updates, quit: quit, latest: newLatestRegistry()}
}

// NewHarnessEffects builds an Effects wired for deterministic testing:
// dispatch controls how async bodies are spawned (nil = real goroutine)
// and clock controls time-based effects (nil = real clock). For flatest;
// not app API.
func NewHarnessEffects[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func(), dispatch func(func()), clock Clock) Effects[S] {
	fx := NewEffects(ctx, updates, quit)
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

// SetClipboard writes text to the system clipboard via OSC52 on the next
// flush. Loop-goroutine-only, like Quit. Terminals without OSC52 support
// ignore it. Safe on a zero Effects value.
func (fx Effects[S]) SetClipboard(text string) {
	if fx.enqueue != nil {
		fx.enqueue(action{write: ansi.SetSystemClipboard(text)})
	}
}

// ReadClipboard asks the terminal for its clipboard content via OSC52.
// A supporting terminal answers with a ClipboardEvent; unsupported
// terminals never answer — treat the event as optional and do not wait
// for it. Loop-goroutine-only, like Quit. Safe on a zero Effects value.
func (fx Effects[S]) ReadClipboard() {
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

// Option configures Run behaviour.
type Option func(*runConfig)

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
func WithMouse(mode MouseMode) Option { return func(c *runConfig) { c.mouse = mode } }

// WithReportFocus enables focus reporting; terminal focus changes arrive
// as FocusEvent. Some terminals and multiplexers need configuration to
// report focus (tmux: focus-events).
func WithReportFocus() Option { return func(c *runConfig) { c.reportFocus = true } }

// WithInline renders below the shell prompt instead of in the alternate
// screen: the frame occupies exactly its own lines, the terminal's
// scrollback stays intact, and on exit the final frame remains visible
// with the prompt landing below it.
func WithInline() Option { return func(c *runConfig) { c.inline = true } }

func Run[S any](ctx context.Context, app App[S], opts ...Option) error {
	if app.State == nil {
		panic("flatcore: App.State is nil")
	}
	if app.View == nil {
		panic("flatcore: App.View is nil")
	}

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
	in, out := cfg.input, cfg.output

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
	renderer := uv.NewTerminalRenderer(renderOut, os.Environ())
	var lastFrame Frame
	drew := false
	cursorShown := false
	var screen uv.ScreenBuffer
	var screenWidth, screenHeight int
	forceRepaint := false
	var pending []action

	// enterRenderer applies the renderer-side entry state: alt screen
	// (unless inline), cursor hidden, terminal modes. Alt-screen entry must
	// go through the renderer, not a raw write: EnterAltScreen also flips
	// the renderer's fullscreen + absolute-cursor flags, or its inline-mode
	// cursor/scroll optimizations desync the screen (frames drift one row
	// off). A fresh renderer's default state (relative cursor, not
	// fullscreen) IS inline mode, so WithInline simply leaves it alone.
	// Reused at startup and after a suspend/exec hands the terminal back.
	enterRenderer := func() {
		if !cfg.inline {
			renderer.EnterAltScreen()
		}
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
		if err := renderer.Flush(); err == nil && renderOut.Len() > 0 {
			_, _ = out.Write(renderOut.Bytes())
			renderOut.Reset()
		}
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The input pipeline is restartable: suspend and exec stop it before
	// handing the terminal away (the subprocess must own stdin) and start
	// a fresh incarnation afterwards. stop blocks until no goroutine can
	// touch the input source anymore (a Close racing a blocked read is a
	// data race).
	type inputPipeline struct {
		events <-chan inputResult
		stop   func()
	}
	startInput := func() inputPipeline {
		inputCtx, cancelCtx := context.WithCancel(runCtx)
		eventSource := io.Reader(in)
		cancelRead := func() {}
		if cancelable, err := uv.NewCancelReader(in); err == nil {
			eventSource = cancelable
			cancelRead = func() { _ = cancelable.Cancel() }
		}
		events, done := readInput(inputCtx, eventSource)
		return inputPipeline{events: events, stop: func() {
			cancelCtx()
			cancelRead()
			<-done
		}}
	}
	pipe := startInput()
	defer func() {
		cancel()
		pipe.stop()
	}()
	resize, stopResize := notifyResize()
	defer stopResize()
	updates := make(chan StateUpdate[S], 64)
	quitRequested := false
	effects := NewEffects(runCtx, updates, func() { quitRequested = true })
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

	writeFrameOutput := func() {
		if renderOut.Len() == 0 {
			return // nothing to write, not even markers
		}
		fmt.Fprintf(out, "\x1b[?2026h%s\x1b[?2026l", renderOut.Bytes())
		renderOut.Reset()
	}

	draw := func() {
		renderCtx := RenderContextFor(out)
		frame := app.View(app.State, renderCtx)
		if drew && !forceRepaint && framesEqual(frame, lastFrame) && screenWidth == renderCtx.Width {
			// Identical frame: only queued one-shot actions (if any) go out.
			if err := renderer.Flush(); err == nil {
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
			_, _ = renderer.WriteString("\x1b[?25l")
			cursorShown = false
		}
		screen.Clear()
		styled.Draw(screen, screen.Bounds())
		renderer.Render(screen.RenderBuffer)
		if frame.Cursor != nil {
			// MoveTo must come after Render: rendering moves the cursor.
			renderer.MoveTo(frame.Cursor.X, frame.Cursor.Y)
			if !cursorShown {
				_, _ = renderer.WriteString("\x1b[?25h")
				cursorShown = true
			}
		}
		if err := renderer.Flush(); err != nil {
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
		pipe.stop()
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
		if err := renderer.Flush(); err == nil && renderOut.Len() > 0 {
			_, _ = out.Write(renderOut.Bytes())
			renderOut.Reset()
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
			renderer = uv.NewTerminalRenderer(renderOut, os.Environ())
			screenWidth, screenHeight = 0, 0
		}
		enterRenderer()
		pipe = startInput()
		forceRepaint = true
		width, height := terminalSize(out)
		resizeEvent := Event(ResizeEvent{Width: width, Height: height})
		app.Tracer.Event(resizeEvent)
		if app.Handle != nil {
			app.Handle(app.State, resizeEvent, effects)
		}
	}

	// processActions drains the one-shot queue before each draw. Suspend
	// runs here, not in drainActions: it re-enters the loop machinery
	// (input restart, repaint) and must never run mid-render. The outer
	// loop re-checks because the post-restore resize Handle may enqueue.
	processActions := func() {
		for len(pending) > 0 {
			queued := pending
			pending = nil
			for _, a := range queued {
				switch {
				case a.suspend:
					releaseTerminal()
					cfg.suspendProcess()
					restoreTerminal()
				case a.exec != nil:
					releaseTerminal()
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
				}
			}
		}
	}

	processActions() // Init and the initial resize Handle may have enqueued
	if quitRequested {
		return nil
	}
	draw()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case result, ok := <-pipe.events:
			if !ok {
				return nil
			}
			if result.err != nil {
				return result.err
			}
			if key, isKey := result.event.(KeyEvent); isKey && key.Key == KeyCtrlC && cfg.defaultQuit {
				return nil
			}
			app.Tracer.Event(result.event)
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
		if quitRequested {
			return nil
		}
		draw()
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
		defer close(done)
		defer close(results)
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
