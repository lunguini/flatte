package flatte

import "github.com/lunguini/flatte/flatui/layout"

// StepInput is one non-blocking turn of the shared host loop. A driver passes
// the events it received, whether async updates are pending, a frame deadline,
// and context cancellation as inputs; it never waits inside Step.
type StepInput struct {
	Events         []Event
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
	Quit  bool
}

type sessionCell struct {
	menus      MenuHost
	windows    WindowHost
	dialogs    termDialogs
	lastComp   layout.Composition
	lastLayout map[string]layout.Rect
}

func stepSession[S any](session *Session, app App[S], cell *sessionCell, input StepInput) StepResult {
	if input.Quit {
		return StepResult{Quit: true}
	}
	changed := false
	for _, event := range input.Events {
		if session.Quitting() {
			return StepResult{Quit: true}
		}
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
			changed = true
			if session.Quitting() {
				return StepResult{Quit: true}
			}
		}
	}

	if input.UpdatesPending || len(input.Events) > 0 {
		if session.applyPendingBatch() > 0 {
			changed = true
		}
	}
	if session.Quitting() {
		return StepResult{Quit: true}
	}
	if input.FrameDue {
		changed = true
	}
	if !changed {
		return StepResult{}
	}
	result := StepResult{Paint: true}
	if session.cellSurface {
		frame, ok := cellFrame(session, app, cell, input.RenderContext)
		if !ok {
			return StepResult{Quit: session.Quitting()}
		}
		result.Frame = frame
		if session.publishMedia != nil {
			var snapshot *NowPlaying
			if value := session.NowPlaying(); value != nil {
				copy := *value
				snapshot = &copy
			}
			session.publishMedia(snapshot)
		}
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
		func(layout.Node) int { return ctx.Height },
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
