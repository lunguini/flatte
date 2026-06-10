package flatcore

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

type App[S any] struct {
	State  *S
	Init   func(*S, Effects[S])
	Handle func(*S, Event, Effects[S])
	View   func(*S, RenderContext) string
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

	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := readInput(runCtx, bufio.NewReader(in))
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

	renderer := NewDiffRenderer()
	draw := func() {
		renderCtx := RenderContextFor(out)
		var buf bytes.Buffer
		renderer.Draw(&buf, app.View(app.State, renderCtx), renderCtx)
		if buf.Len() == 0 {
			return // identical frame: write nothing, not even markers
		}
		fmt.Fprintf(out, "\x1b[?2026h%s\x1b[?2026l", buf.Bytes())
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
			if input.event.Key == KeyCtrlC && cfg.defaultQuit {
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
			resizeEvent := Event{Key: KeyResize}
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

func readInput(ctx context.Context, reader *bufio.Reader) <-chan inputResult {
	results := make(chan inputResult)
	go func() {
		defer close(results)
		for {
			event, err := readEvent(reader)
			result := inputResult{event: event, err: err}
			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return results
}
