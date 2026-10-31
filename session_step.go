package flatte

import "github.com/lunguini/flatte/flatui/layout"

// StepInput is one non-blocking turn of the shared host loop. A driver passes
// the events it received, whether async updates are pending, a frame deadline,
// and context cancellation as inputs; it never waits inside Step.
type StepInput struct {
	Events         []Event
	Resize         *ResizeEvent
	UpdatesPending bool
	FrameDue       bool
	Quit           bool
	RenderContext  RenderContext
}

// StepResult reports whether the driver should paint, the cell frame prepared
// by a cell host, and whether the loop should stop.
type StepResult struct {
	Frame Frame
	Paint bool
	Dirty bool
	Quit  bool
}

type sessionCell struct {
	menus                          MenuHost
	windows                        WindowHost
	dialogs                        termDialogs
	dialogResizeRoutesThroughMenus bool
	lastComp                       layout.Composition
	lastLayout                     map[string]layout.Rect
}

func stepSession[S any](session *Session, app App[S], cell *sessionCell, input StepInput) StepResult {
	if input.Quit {
		return StepResult{Quit: true}
	}
	if input.Resize != nil {
		session.dirty = true
		session.handle(*input.Resize)
		if session.Quitting() {
			return StepResult{Quit: true}
		}
	}
	for _, event := range input.Events {
		if session.Quitting() {
			return StepResult{Quit: true}
		}
		session.dirty = true
		events := []Event{event}
		if session.cellSurface {
			events = routeCellEvent(cell, app, event, input.RenderContext)
		}
		for _, routed := range events {
			if key, ok := routed.(KeyEvent); ok && key.Key == KeyCtrlC && session.defaultQuit {
				session.trace(routed)
				return StepResult{Quit: true}
			}
			session.handle(routed)
			if session.Quitting() {
				return StepResult{Quit: true}
			}
		}
	}

	if input.UpdatesPending {
		if session.applyPendingBatch() > 0 {
			session.dirty = true
		}
	}
	if session.Quitting() {
		return StepResult{Quit: true}
	}
	if !input.FrameDue {
		return StepResult{Dirty: session.dirty}
	}
	result := StepResult{Paint: true}
	if session.cellSurface {
		frame, ok := cellFrame(session, app, cell, input.RenderContext)
		if !ok {
			return StepResult{Quit: session.Quitting()}
		}
		result.Frame = frame
		session.dirty = false
		if session.publishMedia != nil {
			var snapshot *NowPlaying
			if value := session.NowPlaying(); value != nil {
				copy := *value
				snapshot = &copy
			}
			session.publishMedia(snapshot)
		}
	} else {
		session.dirty = false
	}
	return result
}

func routeCellEvent[S any](cell *sessionCell, app App[S], event Event, ctx RenderContext) []Event {
	if cell.dialogs.isOpen() {
		routed, pass := cell.dialogs.route(event)
		if !pass {
			return nil
		}
		event = routed
		if _, resize := event.(ResizeEvent); resize && !cell.dialogResizeRoutesThroughMenus {
			return []Event{event}
		}
	}
	if mouse, ok := event.(MouseEvent); ok {
		mouse.Target, mouse.TargetRect = cell.lastComp.HitTest(mouse.X, mouse.Y)
		mouse.Caret, mouse.HasCaret = cell.lastComp.CaretAt(mouse.X, mouse.Y)
		event = mouse
	}
	return RouteInput(&cell.menus, &cell.windows, app, event, ctx.Width, ctx.Height)
}

func cellFrame[S any](session *Session, app App[S], cell *sessionCell, ctx RenderContext) (Frame, bool) {
	ctx.Capabilities = session.Capabilities()
	for _, event := range SyncWindows(&cell.windows, app, ctx.Width, ctx.Height) {
		session.handle(event)
		if session.Quitting() {
			return Frame{}, false
		}
	}
	var handleLayout func(LayoutEvent)
	if app.Handle != nil {
		handleLayout = func(event LayoutEvent) {
			session.handle(event)
		}
	}
	frame, comp, delivered := settleFrame(
		func() Frame { return session.View(ctx) },
		handleLayout,
		cell.lastLayout,
		ctx.Width,
		func(root layout.Node) int { return rootHeight(root, ctx.Height, session.inline) },
	)
	cell.lastLayout, cell.lastComp = delivered, comp
	if session.Quitting() {
		return Frame{}, false
	}
	frame = PaintWindows(&cell.windows, app, ctx, frame, handleLayout)
	frame = cell.menus.Paint(frame)
	frame = cell.dialogs.paint(frame, ctx.Width, ctx.Height)
	return frame, true
}

// applyPendingBatch applies one snapshot so a continuously producing effect
// cannot keep one host turn from reaching its next frame.
func (s *Session) applyPendingBatch() int {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	if len(pending) > 0 {
		select {
		case <-s.updates:
		default:
		}
	}
	s.mu.Unlock()
	for _, update := range pending {
		update()
	}
	return len(pending)
}

// Step advances the shared host loop without blocking. Host drivers own their
// wait mechanism (channels, callbacks, or animation frames) and pass its result
// here. Events and queued updates are processed before a requested frame.
func (s *Session) Step(input StepInput) StepResult { return s.step(input) }
