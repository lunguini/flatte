package flatte

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/lunguini/flatte/flatui/layout"
)

type terminalRuntime struct {
	cfg            runConfig
	in             io.Reader
	out            io.Writer
	sink           *outputSink
	terminalEnv    []string
	rawFile        *os.File
	originalTerm   *term.State
	rawEntered     bool
	runCtx         context.Context
	cancelRun      context.CancelFunc
	session        *Session
	themeCleanup   bool
	sessionCleanup bool
	inputStarted   bool
	resize         <-chan os.Signal
	stopResize     func()
	renderOut      *bytes.Buffer
	renderer       *uv.TerminalRenderer
	rendererReady  bool
	kittyImages    *kittyImageState
	lastFrame      Frame
	drew           bool
	cursorShown    bool
	cursorShape    int
	cursorColor    string
	screen         uv.ScreenBuffer
	screenWidth    int
	screenHeight   int
	forceRepaint   bool
	pending        []action
	pipe           *terminalInputPipeline
	inputStopped   bool
	inputCloseErr  error
}

// setupTerminalOptions prepares the terminal, renderer, and session for a run.
func setupTerminalOptions[S any](r *terminalRuntime, ctx context.Context, app App[S]) error {
	r.in, r.out = r.cfg.input, r.cfg.output
	r.sink = &outputSink{w: r.out}
	r.terminalEnv = os.Environ()

	if file, ok := r.in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		r.rawFile = file
	}
	if err := r.enterRaw(); err != nil {
		return err
	}
	r.rawEntered = true

	r.renderOut = &bytes.Buffer{}
	r.renderer = r.newRenderer()
	r.kittyImages = newKittyImageState(kittyImagesEnabled(r.terminalEnv) && terminalOutputIsTTY(r.out))
	r.cursorShape = -1
	r.enterRenderer()
	r.rendererReady = true

	r.runCtx, r.cancelRun = context.WithCancel(ctx)
	r.session = newSession(r.runCtx, app)
	if r.cfg.theme != nil {
		r.session.UseTheme(*r.cfg.theme)
		layout.UseCellTheme(&r.session.theme.Dark)
	}
	r.themeCleanup = true
	r.session.setActionSink(func(a action) { r.pending = append(r.pending, a) })
	r.session.cell.dialogs.home, _ = os.UserHomeDir()
	r.session.UseHostServices(HostServices{
		Capabilities: terminalCapabilities(r.cfg),
		ShowDialog:   r.session.cell.dialogs.show,
		CellSurface:  true,
		PublishMedia: r.cfg.publishMedia,
	})
	r.session.useCellPolicy(sessionPolicy{defaultQuit: r.cfg.defaultQuit, quitRequests: r.cfg.quitRequests, inline: r.cfg.inline})
	return nil
}

// enterRaw puts a real terminal into raw mode while preserving its original state.
func (r *terminalRuntime) enterRaw() error {
	if r.rawFile == nil {
		return nil
	}
	state, err := term.MakeRaw(int(r.rawFile.Fd()))
	if err != nil {
		return err
	}
	if r.originalTerm == nil {
		r.originalTerm = state
	}
	return nil
}

// restoreRaw returns the terminal to the state captured by its first raw-mode entry.
func (r *terminalRuntime) restoreRaw() {
	if r.rawFile != nil && r.originalTerm != nil {
		_ = term.Restore(int(r.rawFile.Fd()), r.originalTerm)
	}
}

func (r *terminalRuntime) newRenderer() *uv.TerminalRenderer {
	return newTerminalRenderer(r.renderOut, r.out, r.terminalEnv, r.cfg.inline)
}

// enterRenderer applies terminal modes through the renderer so its screen model stays aligned.
func (r *terminalRuntime) enterRenderer() {
	if !r.cfg.inline {
		r.renderer.EnterAltScreen()
	}
	r.cursorShape = -1
	r.cursorColor = ""
	_, _ = r.renderer.WriteString("\x1b[?25l")
	_, _ = r.renderer.WriteString(setModes(r.cfg))
	r.cursorShown = false
}

// startInputPipeline starts terminal reads and resize notifications for the event loop.
func (r *terminalRuntime) startInputPipeline() {
	r.pipe = newTerminalInputPipeline(r.runCtx, r.in, r.recordInputClose)
	r.inputStarted = true
	r.resize, r.stopResize = notifyResize()
	r.sessionCleanup = true
}

func (r *terminalRuntime) recordInputClose(err error) {
	if err != nil && r.inputCloseErr == nil {
		r.inputCloseErr = err
	}
}

// runTerminalEventLoop sends the initial resize, paints the first frame, and enters the loop.
func (r *terminalRuntime) runTerminalEventLoop() error {
	width, height := terminalSize(r.out)
	initialResize := ResizeEvent{Width: width, Height: height}
	started := r.session.Step(StepInput{Start: &initialResize})
	if started.Quit {
		return nil
	}

	r.processActions()
	if r.sink.err != nil {
		return r.sink.err
	}
	if r.session.quittingNow() {
		return nil
	}
	r.render()
	if r.sink.err != nil {
		return r.sink.err
	}
	if r.session.quittingNow() {
		return nil
	}
	return r.eventLoop()
}

// eventLoop handles input, resize, media, and state updates until the session quits.
func (r *terminalRuntime) eventLoop() error {
	for {
		select {
		case <-r.runCtx.Done():
			return nil
		case result, ok := <-r.pipe.events:
			done, err := r.handleInputBatch(result, ok)
			if err != nil || done {
				return err
			}
		case <-r.resize:
			r.forceRepaint = true
			width, height := terminalSize(r.out)
			resizeEvent := ResizeEvent{Width: width, Height: height}
			step := r.session.Step(StepInput{Resize: &resizeEvent, UpdatesPending: true, RenderContext: r.eventRenderContext()})
			if step.Quit {
				return nil
			}
		case event, ok := <-r.cfg.mediaCommands:
			if !ok {
				r.cfg.mediaCommands = nil
				continue
			}
			step := r.session.Step(StepInput{
				Events: []Event{event}, UpdatesPending: true, RenderContext: r.eventRenderContext(),
			})
			if step.Quit {
				return nil
			}
		case <-r.session.Updates():
			step := r.session.Step(StepInput{UpdatesPending: true, RenderContext: r.eventRenderContext()})
			if step.Quit {
				return nil
			}
		}
		if r.session.quittingNow() {
			return nil
		}
		r.processActions()
		if r.sink.err != nil {
			return r.sink.err
		}
		if r.session.quittingNow() {
			return nil
		}
		r.render()
		if r.sink.err != nil {
			return r.sink.err
		}
		if r.session.quittingNow() {
			return nil
		}
	}
}

// handleInputBatch drains ready input until it reaches a frame boundary.
func (r *terminalRuntime) handleInputBatch(result inputResult, ok bool) (bool, error) {
	for n := 0; ; n++ {
		if !ok {
			return true, nil
		}
		if result.err != nil {
			r.pipe.finish()
			return true, result.err
		}
		if _, resize := result.event.(ResizeEvent); resize {
			r.forceRepaint = true
		}
		step := r.session.Step(StepInput{
			Events: []Event{result.event}, UpdatesPending: true, RenderContext: r.eventRenderContext(),
		})
		if step.Quit || r.session.quittingNow() {
			return true, nil
		}
		if n+1 >= maxInputBatch || len(r.pending) > 0 {
			break
		}
		select {
		case result, ok = <-r.pipe.events:
			continue
		default:
		}
		break
	}
	return false, nil
}

// teardown releases input, session, renderer, and terminal state in run order.
func (r *terminalRuntime) teardown(runErr error) error {
	if r.sessionCleanup && r.session != nil {
		r.session.close()
	}
	if r.stopResize != nil {
		r.stopResize()
	}
	if r.inputStarted {
		r.cancelRun()
		r.pipe.finalize()
	}
	if r.themeCleanup && r.session != nil && (r.cfg.theme != nil || r.session.cellThemed) {
		layout.UseCellTheme(nil)
	}
	if r.cancelRun != nil {
		r.cancelRun()
	}
	if r.rendererReady {
		runErr = r.teardownRenderer(runErr)
	}
	if r.rawEntered {
		r.restoreRaw()
	}
	return runErr
}

// teardownRenderer clears renderer state and reports output failures.
func (r *terminalRuntime) teardownRenderer(runErr error) error {
	r.drainActions()
	r.kittyImages.clear(r.renderer)
	if r.lastFrame.Title != "" {
		_, _ = r.renderer.WriteString(ansi.SetWindowTitle(""))
	}
	r.resetCursorStyle()
	_, _ = r.renderer.WriteString(resetModes(r.cfg))
	_, _ = r.renderer.WriteString("\x1b[?25h")
	if r.cfg.inline {
		if r.drew && r.screenHeight > 0 {
			r.renderer.MoveTo(0, r.screenHeight-1)
		}
		_, _ = r.renderer.WriteString("\r\n")
	} else {
		r.renderer.ExitAltScreen()
	}
	if err := r.flushRenderer(); err != nil && runErr == nil {
		runErr = err
	}
	if r.renderOut.Len() > 0 {
		data := append([]byte(nil), r.renderOut.Bytes()...)
		r.renderOut.Reset()
		_ = r.sink.write(data)
	}
	if runErr == nil {
		runErr = r.sink.err
	}
	return runErr
}

func (r *terminalRuntime) drainActions() {
	for _, a := range r.pending {
		if a.write != "" {
			_, _ = r.renderer.WriteString(a.write)
		}
	}
	r.pending = r.pending[:0]
}

// releaseTerminal returns the terminal to the shell or a subprocess before suspend or exec.
func (r *terminalRuntime) releaseTerminal() {
	r.inputStopped = r.pipe.stop()
	r.kittyImages.clear(r.renderer)
	r.resetCursorStyle()
	_, _ = r.renderer.WriteString(resetModes(r.cfg))
	_, _ = r.renderer.WriteString("\x1b[?25h")
	if r.cfg.inline {
		if r.drew && r.screenHeight > 0 {
			r.renderer.MoveTo(0, r.screenHeight-1)
		}
		_, _ = r.renderer.WriteString("\r\n")
	} else {
		r.renderer.ExitAltScreen()
	}
	if err := r.flushRenderer(); err == nil {
		_ = r.writePendingOutput(false)
	}
	r.restoreRaw()
}

// restoreTerminal re-enters terminal modes and rebuilds the frame after suspend or exec.
func (r *terminalRuntime) restoreTerminal() {
	_ = r.enterRaw()
	if r.cfg.inline {
		r.renderer = r.newRenderer()
		r.screenWidth, r.screenHeight = 0, 0
	}
	r.enterRenderer()
	if r.inputStopped {
		r.pipe = newTerminalInputPipeline(r.runCtx, r.in, r.recordInputClose)
		r.inputStopped = false
	}
	r.forceRepaint = true
	width, height := terminalSize(r.out)
	resizeEvent := ResizeEvent{Width: width, Height: height}
	r.session.Step(StepInput{Resize: &resizeEvent})
}

// insertAbove prints lines to scrollback above an inline frame, then rebuilds the frame below them.
func (r *terminalRuntime) insertAbove(str string) {
	if str == "" {
		return
	}
	r.kittyImages.clear(r.renderer)
	if err := r.flushRenderer(); err != nil {
		return
	}
	if err := r.writePendingOutput(false); err != nil {
		return
	}
	_, y := r.renderer.Position()
	var sb strings.Builder
	sb.WriteByte('\r')
	if y > 0 {
		sb.WriteString(ansi.CursorUp(y))
	}
	sb.WriteString(ansi.EraseScreenBelow)
	for _, line := range strings.Split(str, "\n") {
		sb.WriteString(line)
		sb.WriteString(ansi.EraseLineRight)
		sb.WriteString("\r\n")
	}
	_ = r.sink.write([]byte(sb.String()))
	r.renderer = r.newRenderer()
	r.screenWidth, r.screenHeight = 0, 0
	r.forceRepaint = false
}

// processActions drains writes, suspend requests, and subprocesses before drawing.
func (r *terminalRuntime) processActions() {
	for len(r.pending) > 0 {
		if r.sink.err != nil {
			return
		}
		queued := r.pending
		r.pending = nil
		for _, a := range queued {
			if r.sink.err != nil {
				return
			}
			switch {
			case a.suspend:
				r.releaseTerminal()
				if r.sink.err != nil {
					return
				}
				r.cfg.suspendProcess()
				r.restoreTerminal()
			case a.exec != nil:
				r.releaseTerminal()
				if r.sink.err != nil {
					return
				}
				cmd := a.exec.cmd
				if cmd.Stdin == nil {
					cmd.Stdin = r.in
				}
				if cmd.Stdout == nil {
					cmd.Stdout = r.out
				}
				if cmd.Stderr == nil {
					cmd.Stderr = os.Stderr
				}
				err := cmd.Run()
				r.restoreTerminal()
				a.exec.done(err)
			case a.write != "":
				_, _ = r.renderer.WriteString(a.write)
			case a.print != "":
				if r.cfg.inline && r.drew {
					r.insertAbove(a.print)
				}
			}
			if r.sink.err != nil {
				return
			}
		}
	}
}

// render draws the current frame and handles layout actions queued during the draw.
func (r *terminalRuntime) render() {
	r.drawFrame()
	if r.sink.err != nil || r.session.quittingNow() || len(r.pending) == 0 {
		return
	}
	r.processActions()
	if r.sink.err == nil && !r.session.quittingNow() {
		r.drawFrame()
	}
}

// flushRenderer sends buffered terminal output and records its first error.
func (r *terminalRuntime) flushRenderer() error {
	err := r.renderer.Flush()
	r.sink.record(err)
	return err
}

func (r *terminalRuntime) writePendingOutput(synchronized bool) error {
	if r.renderOut.Len() == 0 {
		return nil
	}
	prefix, suffix := "", ""
	if synchronized {
		prefix, suffix = "\x1b[?2026h", "\x1b[?2026l"
	}
	data := make([]byte, 0, len(prefix)+r.renderOut.Len()+len(suffix))
	data = append(data, prefix...)
	data = append(data, r.renderOut.Bytes()...)
	data = append(data, suffix...)
	r.renderOut.Reset()
	return r.sink.write(data)
}

func (r *terminalRuntime) writeFrameOutput() {
	_ = r.writePendingOutput(true)
}

func (r *terminalRuntime) terminalRenderContext() RenderContext {
	ctx := renderContextFor(r.out)
	_, ctx.Height = terminalSize(r.out)
	ctx.Capabilities = r.session.caps()
	return ctx
}

func (r *terminalRuntime) eventRenderContext() RenderContext {
	ctx := r.terminalRenderContext()
	if r.drew {
		ctx.Width, ctx.Height = r.screenWidth, r.screenHeight
	}
	return ctx
}

func (r *terminalRuntime) kittyPlacements(frame Frame, width, height int) []layout.TerminalImagePlacement {
	if !r.kittyImages.enabled || frame.CellImages {
		return nil
	}
	return kittyPlacementsForRuntime(
		r.session.cell.lastComp,
		&r.session.cell.windows,
		&r.session.cell.menus,
		&r.session.cell.dialogs,
		width,
		height,
	)
}

// drawFrame paints one session frame and flushes its terminal output.
func (r *terminalRuntime) drawFrame() {
	if r.sink.err != nil {
		return
	}
	renderCtx := r.terminalRenderContext()
	step := r.session.Step(StepInput{FrameDue: true, RenderContext: renderCtx})
	if step.Quit || !step.Paint {
		return
	}
	frame := step.Frame
	if r.drew && !r.forceRepaint && framesEqual(frame, r.lastFrame) && r.screenWidth == renderCtx.Width {
		r.kittyImages.update(r.kittyPlacements(frame, renderCtx.Width, renderCtx.Height), r.renderer)
		if err := r.flushRenderer(); err == nil {
			r.writeFrameOutput()
		}
		return
	}
	if frame.Title != r.lastFrame.Title {
		_, _ = r.renderer.WriteString(ansi.SetWindowTitle(frame.Title))
	}
	styled := uv.NewStyledString(frame.Content)
	height := styled.Height()
	if !r.cfg.inline {
		_, terminalHeight := terminalSize(r.out)
		height = max(height, terminalHeight)
	}
	if r.screenWidth != renderCtx.Width || r.screenHeight != height {
		r.screen = uv.NewScreenBuffer(renderCtx.Width, height)
		r.renderer.Resize(renderCtx.Width, height)
		r.screenWidth, r.screenHeight = renderCtx.Width, height
	}
	if r.forceRepaint {
		r.renderer.Erase()
		r.forceRepaint = false
	}
	if r.cursorShown && frame.Cursor == nil {
		r.resetCursorStyle()
		_, _ = r.renderer.WriteString("\x1b[?25l")
		r.cursorShown = false
	}
	r.screen.Clear()
	styled.Draw(r.screen, r.screen.Bounds())
	images := r.kittyPlacements(frame, renderCtx.Width, renderCtx.Height)
	clearUnderKittyImages(r.screen, images)
	r.renderer.Render(r.screen.RenderBuffer)
	if frame.Cursor != nil {
		r.applyCursorStyle(frame.Cursor.Style)
		r.renderer.MoveTo(frame.Cursor.X, frame.Cursor.Y)
		if !r.cursorShown {
			_, _ = r.renderer.WriteString("\x1b[?25h")
			r.cursorShown = true
		}
	}
	r.kittyImages.update(images, r.renderer)
	if err := r.flushRenderer(); err != nil {
		return
	}
	r.lastFrame, r.drew = frame, true
	r.writeFrameOutput()
}

func (r *terminalRuntime) applyCursorStyle(style *CursorStyle) {
	if code, ok := cursorShapeCode(style); ok {
		if r.cursorShape != code {
			_, _ = fmt.Fprintf(r.renderer, "\x1b[%d q", code)
			r.cursorShape = code
		}
	} else if r.cursorShape != -1 {
		_, _ = r.renderer.WriteString("\x1b[0 q")
		r.cursorShape = -1
	}
	if token := cursorColorToken(style); token != "" {
		if r.cursorColor != token {
			_, _ = r.renderer.WriteString("\x1b]12;" + token + "\x07")
			r.cursorColor = token
		}
	} else if r.cursorColor != "" {
		_, _ = r.renderer.WriteString("\x1b]112\x07")
		r.cursorColor = ""
	}
}

func (r *terminalRuntime) resetCursorStyle() {
	r.applyCursorStyle(nil)
}

// terminalInputPipeline keeps an uncancelable reader attached until its read ends.
type terminalInputPipeline struct {
	events     <-chan inputResult
	done       <-chan struct{}
	cancelRead func() bool
	cancel     context.CancelFunc
	close      func() error
	closeOnce  sync.Once
	closeErr   error
	record     func(error)
	stopped    bool
}

// newTerminalInputPipeline connects a cancellable reader to the runtime event loop.
func newTerminalInputPipeline(ctx context.Context, in io.Reader, record func(error)) *terminalInputPipeline {
	inputCtx, cancelCtx := context.WithCancel(ctx)
	eventSource := in
	cancelRead := func() bool { return false }
	var cancelable cancelReader
	if candidate, err := cancelReaderFactory(in); err == nil {
		cancelable = candidate
		eventSource = cancelable
		cancelRead = cancelable.Cancel
	}
	events, done := readInput(inputCtx, eventSource)
	p := &terminalInputPipeline{events: events, done: done, cancelRead: cancelRead, cancel: cancelCtx, record: record}
	p.close = func() error {
		p.closeOnce.Do(func() {
			if cancelable != nil {
				p.closeErr = cancelable.Close()
			}
		})
		return p.closeErr
	}
	return p
}

func (p *terminalInputPipeline) recordClose() {
	if err := p.close(); err != nil && p.record != nil {
		p.record(err)
	}
}

// finish joins a reader that has already returned an error.
func (p *terminalInputPipeline) finish() {
	if p.stopped {
		return
	}
	p.cancel()
	<-p.done
	p.recordClose()
	p.stopped = true
}

// stop joins the reader when the source supports cancellation.
func (p *terminalInputPipeline) stop() bool {
	if p.stopped {
		return true
	}
	select {
	case <-p.done:
		p.cancel()
		p.recordClose()
		p.stopped = true
		return true
	default:
	}
	if !p.cancelRead() {
		return false
	}
	p.cancel()
	<-p.done
	p.recordClose()
	p.stopped = true
	return true
}

// finalize stops delivery and closes an uncancelable reader when it completes.
func (p *terminalInputPipeline) finalize() {
	if p.stop() {
		return
	}
	p.cancel()
	go func() {
		<-p.done
		_ = p.close()
	}()
}
