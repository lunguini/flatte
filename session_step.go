package flatte

import "github.com/lunguini/flatte/flatui/layout"

// StepInput is one non-blocking turn of the shared host loop. A driver passes
// the events it received, whether async updates are pending, a frame deadline,
// and context cancellation as inputs; it never waits inside Step.
type StepInput struct {
	Events         []Event
	Resize         *ResizeEvent
	Start          *ResizeEvent
	UpdatesPending bool
	FrameDue       bool
	Quit           bool
	RenderContext  RenderContext
}

// HostState is a values-only snapshot used to synchronize host presentation
// after a step. Target-specific lookups are Session methods.
type HostState struct {
	Capabilities Capabilities
	Menus        []Menu
	Windows      []Window
	NowPlaying   *NowPlaying
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
	menus                          menuHost
	windows                        windowHost
	dialogs                        termDialogs
	dialogResizeRoutesThroughMenus bool
	lastComp                       layout.Composition
	lastLayout                     map[string]layout.Rect
}

func stepSession[S any](session *Session, app App[S], cell *sessionCell, input StepInput) StepResult {
	if input.Quit {
		return StepResult{Quit: true}
	}
	if input.Start != nil {
		session.start(*input.Start)
		session.dirty = true
		if session.quittingNow() {
			return StepResult{Quit: true}
		}
	}
	if input.Resize != nil {
		session.dirty = true
		session.handle(*input.Resize)
		if session.quittingNow() {
			return StepResult{Quit: true}
		}
	}
	for _, event := range input.Events {
		if session.quittingNow() {
			return StepResult{Quit: true}
		}
		events := []Event{event}
		if session.cellSurface {
			session.dirty = true
			events = routeCellEvent(cell, app, event, input.RenderContext)
		} else if key, ok := event.(KeyEvent); ok {
			if item, matched := MenuShortcut(session.menus(), key); matched {
				if item.Disabled {
					events = nil
				} else {
					events = []Event{MenuEvent{ID: item.ID}}
				}
			}
		}
		for _, routed := range events {
			session.dirty = true
			if key, ok := routed.(KeyEvent); ok && key.Key == KeyCtrlC && session.defaultQuit {
				session.trace(routed)
				return StepResult{Quit: true}
			}
			session.handle(routed)
			if session.quittingNow() {
				return StepResult{Quit: true}
			}
		}
	}

	if input.UpdatesPending {
		if session.applyPendingBatch() > 0 {
			session.dirty = true
		}
	}
	if session.quittingNow() {
		return StepResult{Quit: true}
	}
	if !input.FrameDue {
		return StepResult{Dirty: session.dirty}
	}
	result := StepResult{Paint: true}
	if session.cellSurface {
		frame, ok := cellFrame(session, app, cell, input.RenderContext)
		if !ok {
			return StepResult{Quit: session.quittingNow()}
		}
		result.Frame = frame
		session.dirty = false
		if session.publishMedia != nil {
			var snapshot *NowPlaying
			if value := session.nowPlaying(); value != nil {
				copy := *value
				snapshot = &copy
			}
			session.publishMedia(snapshot)
		}
	} else {
		ctx := input.RenderContext
		ctx.Capabilities = session.caps()
		result.Frame = session.view(ctx)
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
	return routeInput(&cell.menus, &cell.windows, app, event, ctx.Width, ctx.Height)
}

func cellFrame[S any](session *Session, app App[S], cell *sessionCell, ctx RenderContext) (Frame, bool) {
	ctx.Capabilities = session.caps()
	for _, event := range syncWindows(&cell.windows, app, ctx.Width, ctx.Height) {
		session.handle(event)
		if session.quittingNow() {
			return Frame{}, false
		}
	}
	var handleLayout func(LayoutEvent)
	if app.Handle != nil {
		handleLayout = func(event LayoutEvent) {
			session.handle(event)
		}
	}
	frame, comp, delivered := settleFrameWithHeightProfile(
		func() Frame { return session.view(ctx) },
		handleLayout,
		cell.lastLayout,
		ctx.Width,
		func(root layout.Node) int { return rootHeight(root, ctx.Height, session.inline) },
		ctx.ColorProfile,
	)
	cell.lastLayout, cell.lastComp = delivered, comp
	if session.quittingNow() {
		return Frame{}, false
	}
	frame = paintWindows(&cell.windows, app, ctx, frame, handleLayout)
	frame = cell.menus.paint(frame)
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

func (s *Session) State() HostState {
	state := HostState{Capabilities: s.caps()}
	if s.menus != nil {
		state.Menus = cloneMenus(s.menus())
	}
	if s.windows != nil {
		state.Windows = append([]Window(nil), s.windows()...)
	}
	if s.nowPlaying != nil {
		if item := s.nowPlaying(); item != nil {
			copy := *item
			state.NowPlaying = &copy
		}
	}
	return state
}

// UsesNowPlaying reports whether the app supplies a NowPlaying callback.
func (s *Session) UsesNowPlaying() bool { return s.hasNowPlaying }

// ContextMenu returns the current menu items for a target ID.
func (s *Session) ContextMenu(target string) []MenuItem {
	if s.contextMenu == nil {
		return nil
	}
	return cloneMenuItems(s.contextMenu(target))
}

// WindowFrame returns the latest cell-host frame for a secondary window.
func (s *Session) WindowFrame(id string) (Frame, bool) {
	return s.cell.windows.windowFrame(id)
}

func cloneMenus(menus []Menu) []Menu {
	if menus == nil {
		return nil
	}
	clone := make([]Menu, len(menus))
	for i, menu := range menus {
		clone[i] = Menu{Title: menu.Title, Items: cloneMenuItems(menu.Items)}
	}
	return clone
}

func cloneMenuItems(items []MenuItem) []MenuItem {
	if items == nil {
		return nil
	}
	clone := make([]MenuItem, len(items))
	for i, item := range items {
		clone[i] = item
		clone[i].Keys = append([]KeyEvent(nil), item.Keys...)
		clone[i].Items = cloneMenuItems(item.Items)
	}
	return clone
}
