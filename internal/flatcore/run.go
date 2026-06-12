package flatcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

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

type Effects[S any] struct {
	Context context.Context
	Updates chan<- StateUpdate[S]
	quit    func()
	latest  *latestRegistry
}

// NewEffects builds an Effects value with an observable quit callback.
// Run uses it internally; tests use it to assert quit requests.
func NewEffects[S any](ctx context.Context, updates chan<- StateUpdate[S], quit func()) Effects[S] {
	return Effects[S]{Context: ctx, Updates: updates, quit: quit, latest: newLatestRegistry()}
}

// Quit requests a clean exit of the Run loop. Safe on a zero Effects value.
// Call it from Init, Handle, or a fold — they all run on the loop goroutine;
// calling it from an app-spawned goroutine is a data race.
func (fx Effects[S]) Quit() {
	if fx.quit != nil {
		fx.quit()
	}
}

// Option configures Run behaviour.
type Option func(*runConfig)

type runConfig struct {
	input       io.Reader
	output      io.Writer
	defaultQuit bool
}

// WithInput sets the event source. Default: os.Stdin.
func WithInput(in io.Reader) Option { return func(c *runConfig) { c.input = in } }

// WithOutput sets the render sink. Default: os.Stdout.
func WithOutput(out io.Writer) Option { return func(c *runConfig) { c.output = out } }

// WithoutDefaultQuit delivers Ctrl-C to the app instead of exiting the loop.
// The app must call fx.Quit(), close the input, or cancel the context to exit.
func WithoutDefaultQuit() Option { return func(c *runConfig) { c.defaultQuit = false } }

func Run[S any](ctx context.Context, app App[S], opts ...Option) error {
	if app.State == nil {
		panic("flatcore: App.State is nil")
	}
	if app.View == nil {
		panic("flatcore: App.View is nil")
	}

	cfg := runConfig{input: os.Stdin, output: os.Stdout, defaultQuit: true}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	in, out := cfg.input, cfg.output

	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		oldState, err := term.MakeRaw(int(file.Fd()))
		if err != nil {
			return err
		}
		defer func() {
			_ = term.Restore(int(file.Fd()), oldState)
		}()
	}

	// Alt-screen entry/exit goes through the renderer, not raw writes: it
	// must also flip the renderer's fullscreen + absolute-cursor flags, or
	// its inline-mode cursor/scroll optimizations desync the screen (frames
	// drift one row off). EnterAltScreen queues the escape; the first draw
	// flushes it together with the initial frame.
	renderOut := &bytes.Buffer{}
	renderer := uv.NewTerminalRenderer(renderOut, os.Environ())
	renderer.EnterAltScreen()
	_, _ = renderer.WriteString("\x1b[?25l") // hide cursor (terminals may reset it on alt-screen entry)
	var lastFrame Frame
	drew := false
	cursorShown := false
	defer func() {
		if lastFrame.Title != "" {
			_, _ = renderer.WriteString(ansi.SetWindowTitle(""))
		}
		_, _ = renderer.WriteString("\x1b[?25h")
		renderer.ExitAltScreen()
		if err := renderer.Flush(); err == nil && renderOut.Len() > 0 {
			_, _ = out.Write(renderOut.Bytes())
			renderOut.Reset()
		}
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventSource := io.Reader(in)
	cancelInput := func() {}
	if cancelable, err := uv.NewCancelReader(in); err == nil {
		eventSource = cancelable
		cancelInput = func() { _ = cancelable.Cancel() }
	}
	events, inputDone := readInput(runCtx, eventSource)
	defer func() {
		// Stop the input pipeline before Run returns: cancel the stream,
		// unblock any in-progress read, and wait until no reader goroutine
		// can touch the input source anymore (a Close racing a blocked
		// read is a data race).
		cancel()
		cancelInput()
		<-inputDone
	}()
	resize, stopResize := notifyResize()
	defer stopResize()
	updates := make(chan StateUpdate[S], 64)
	quitRequested := false
	effects := NewEffects(runCtx, updates, func() { quitRequested = true })
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

	var screen uv.ScreenBuffer
	var screenWidth, screenHeight int
	forceRepaint := false

	draw := func() {
		renderCtx := RenderContextFor(out)
		frame := app.View(app.State, renderCtx)
		if drew && !forceRepaint && framesEqual(frame, lastFrame) && screenWidth == renderCtx.Width {
			return // identical frame: write nothing, not even markers
		}
		if frame.Title != lastFrame.Title {
			_, _ = renderer.WriteString(ansi.SetWindowTitle(frame.Title))
		}
		styled := uv.NewStyledString(frame.Content)
		_, terminalHeight := terminalSize(out)
		height := max(styled.Height(), terminalHeight)
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
		if renderOut.Len() == 0 {
			return // uv's diff found no terminal-state change to write
		}
		fmt.Fprintf(out, "\x1b[?2026h%s\x1b[?2026l", renderOut.Bytes())
		renderOut.Reset()
	}

	draw()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case input, ok := <-events:
			if !ok {
				return nil
			}
			if input.err != nil {
				return input.err
			}
			if key, isKey := input.event.(KeyEvent); isKey && key.Key == KeyCtrlC && cfg.defaultQuit {
				return nil
			}
			app.Tracer.Event(input.event)
			if app.Handle != nil {
				app.Handle(app.State, input.event, effects)
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
		draw()
	}
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
