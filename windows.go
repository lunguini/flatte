package flatte

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// Window is a secondary window an app declares (App.Windows). ID names it:
// View draws it when RenderContext.Window is ID, and its events carry it.
// Width and Height are its initial content size in cells (0: 60×20);
// MinWidth and MinHeight its smallest. Autosave, when set, names the
// window's saved frame: a desktop host restores its position and size
// from it. A terminal draws a window over the main frame instead.
type Window struct {
	ID                  string
	Width, Height       int
	MinWidth, MinHeight int
	Autosave            string
}

func (w Window) size() (int, int) {
	cols, lines := w.Width, w.Height
	if cols <= 0 {
		cols = 60
	}
	if lines <= 0 {
		lines = 20
	}
	return max(cols, w.MinWidth), max(lines, w.MinHeight)
}

// WindowEventKind is what a WindowEvent reports.
type WindowEventKind int

const (
	// WindowFocused: Window ("" the main window) now has the keyboard; keys
	// and pastes go to it until the next WindowFocused.
	WindowFocused WindowEventKind = iota
	// WindowCloseRequest: the user asked to close Window (its close button,
	// ⌘W). It closes when the app stops declaring it — or stays, if the app
	// keeps it (to ask about unsaved changes, say).
	WindowCloseRequest
)

// WindowEvent reports a window gaining focus or the user asking to close
// it.
type WindowEvent struct {
	Window string
	Kind   WindowEventKind
}

func (WindowEvent) isEvent() {}

// WindowHost is the window presentation of a host that draws secondary
// windows itself — the terminal runtime and flatest: it draws each
// declared window as a bordered box over the main frame, tracks which one
// has focus, and locates mouse events in them. Host plumbing, not
// application API. The zero value is ready.
type WindowHost struct {
	open  []*hostWindow // in declaration order: later ones are on top
	focus string
}

type hostWindow struct {
	win Window
	// x, y is the box's top-left cell; w, h its content size.
	x, y, w, h int
	comp       layout.Composition
	delivered  map[string]layout.Rect
	title      string
	frame      Frame
}

// Focus is the focused window's ID ("" the main window).
func (wh *WindowHost) Focus() string { return wh.focus }

// SyncWindows adopts app's declared windows for a width×height screen and
// returns the WindowEvents that result: a newly declared window takes
// focus; when the focused one goes, focus falls to the topmost left (or the
// main window).
func SyncWindows[S any](wh *WindowHost, app App[S], width, height int) []Event {
	var declared []Window
	if app.Windows != nil {
		declared = app.Windows(app.State)
	}
	old := make(map[string]*hostWindow, len(wh.open))
	for _, hw := range wh.open {
		old[hw.win.ID] = hw
	}
	open := make([]*hostWindow, 0, len(declared))
	added := ""
	seen := map[string]bool{}
	for _, w := range declared {
		if w.ID == "" || seen[w.ID] {
			continue // the main window's ID, or a duplicate
		}
		seen[w.ID] = true
		hw, ok := old[w.ID]
		if !ok {
			hw = &hostWindow{}
			added = w.ID
		}
		hw.win = w
		open = append(open, hw)
	}
	wh.open = open
	for i, hw := range wh.open {
		cw, ch := hw.win.size()
		hw.w, hw.h = max(min(cw, width-2), 1), max(min(ch, height-2), 1)
		hw.x = min(max((width-hw.w-2)/2+2*i, 0), max(width-hw.w-2, 0))
		hw.y = min(max((height-hw.h-2)/2+i, 0), max(height-hw.h-2, 0))
	}
	focus := wh.focus
	switch {
	case added != "":
		focus = added
	case focus != "" && !seen[focus]:
		focus = ""
		if n := len(wh.open); n > 0 {
			focus = wh.open[n-1].win.ID
		}
	}
	if focus == wh.focus {
		return nil
	}
	wh.focus = focus
	return []Event{WindowEvent{Window: focus, Kind: WindowFocused}}
}

// PaintWindows draws app's windows over frame: each window's View, settled
// at its size (its LayoutEvents, with Window set, go to handle), in a
// bordered box titled by its Frame.Title. The focused window's cursor
// shows; the main frame's shows only while it has focus.
func PaintWindows[S any](wh *WindowHost, app App[S], frame Frame, handle func(LayoutEvent)) Frame {
	if len(wh.open) == 0 {
		return frame
	}
	if wh.focus != "" {
		frame.Cursor = nil
	}
	for _, hw := range wh.open {
		id := hw.win.ID
		var deliver func(LayoutEvent)
		if handle != nil {
			deliver = func(ev LayoutEvent) {
				ev.Window = id
				handle(ev)
			}
		}
		wf, comp, delivered := SettleFrame(func() Frame {
			return app.View(app.State, RenderContext{Width: hw.w, Window: id})
		}, deliver, hw.delivered, hw.w, hw.h)
		hw.comp, hw.delivered, hw.title, hw.frame = comp, delivered, wf.Title, wf
		frame.Content = layout.OverlayAt(frame.Content, hw.render(id == wh.focus), hw.x, hw.y)
		if id == wh.focus && wf.Cursor != nil && wf.Cursor.X < hw.w && wf.Cursor.Y < hw.h {
			frame.Cursor = &Cursor{X: hw.x + 1 + wf.Cursor.X, Y: hw.y + 1 + wf.Cursor.Y, Style: wf.Cursor.Style}
		}
	}
	return frame
}

// WindowFrame is the last frame drawn for window id (composed at its size,
// without the box), for tests.
func (wh *WindowHost) WindowFrame(id string) (Frame, bool) {
	for _, hw := range wh.open {
		if hw.win.ID == id {
			return hw.frame, true
		}
	}
	return Frame{}, false
}

var (
	windowBorder        = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	windowBorderFocused = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

const windowClose = "[×]"

// render is the window's box: a border with its title and a close mark on
// the top rule, and its content.
func (hw *hostWindow) render(focused bool) string {
	st := windowBorder
	if focused {
		st = windowBorderFocused
	}
	title := ""
	if hw.title != "" {
		title = " " + ansi.Truncate(hw.title, max(hw.w-len(windowClose)-4, 0), "…") + " "
	}
	fill := max(hw.w-ansi.StringWidth(title)-ansi.StringWidth(windowClose)-1, 0)
	top := st.Render("╭─"+title+strings.Repeat("─", fill)) + st.Render(windowClose) + st.Render("╮")
	if hw.w < ansi.StringWidth(title)+ansi.StringWidth(windowClose)+1 {
		top = st.Render("╭" + strings.Repeat("─", hw.w) + "╮")
	}
	lines := strings.Split(hw.frame.Content, "\n")
	out := make([]string, 0, hw.h+2)
	out = append(out, top)
	for i := range hw.h {
		line := ""
		if i < len(lines) {
			line = ansi.Truncate(lines[i], hw.w, "")
		}
		pad := max(hw.w-ansi.StringWidth(line), 0)
		out = append(out, st.Render("│")+line+strings.Repeat(" ", pad)+st.Render("│"))
	}
	out = append(out, st.Render("╰"+strings.Repeat("─", hw.w)+"╯"))
	return strings.Join(out, "\n")
}

// closeAt reports whether (x, y) is on the window's close mark.
func (hw *hostWindow) closeAt(x, y int) bool {
	cx := hw.x + 1 + hw.w - ansi.StringWidth(windowClose)
	return y == hw.y && x >= cx && x < cx+ansi.StringWidth(windowClose) && hw.w >= ansi.StringWidth(windowClose)+1
}

// Route locates a mouse event in the window under the pointer (topmost
// first): inside a window's content it is made window-local, hit-tested
// there, and tagged with the window; a press on a window (or outside every
// window) focuses it first; a press on a close mark is a close request.
// Other events pass unchanged. It returns the events to deliver, in order.
func (wh *WindowHost) Route(ev Event) []Event {
	m, ok := ev.(MouseEvent)
	if !ok || len(wh.open) == 0 {
		return []Event{ev}
	}
	var out []Event
	focus := func(id string) {
		if m.Action == MousePress && wh.focus != id {
			wh.focus = id
			out = append(out, WindowEvent{Window: id, Kind: WindowFocused})
		}
	}
	for i := len(wh.open) - 1; i >= 0; i-- {
		hw := wh.open[i]
		if m.X < hw.x || m.X >= hw.x+hw.w+2 || m.Y < hw.y || m.Y >= hw.y+hw.h+2 {
			continue
		}
		id := hw.win.ID
		if hw.closeAt(m.X, m.Y) {
			if m.Action == MousePress && m.Button == MouseLeft {
				return append(out, WindowEvent{Window: id, Kind: WindowCloseRequest})
			}
			return out
		}
		focus(id)
		lx, ly := m.X-hw.x-1, m.Y-hw.y-1
		if lx < 0 || ly < 0 || lx >= hw.w || ly >= hw.h {
			return out // the border
		}
		m.X, m.Y, m.Window = lx, ly, id
		m.Target, m.TargetRect = hw.comp.HitTest(lx, ly)
		m.Caret, m.HasCaret = hw.comp.CaretAt(lx, ly)
		return append(out, m)
	}
	focus("")
	return append(out, m)
}

// origin is the screen cell of window id's content top-left.
func (wh *WindowHost) origin(id string) (x, y int) {
	for _, hw := range wh.open {
		if hw.win.ID == id {
			return hw.x + 1, hw.y + 1
		}
	}
	return 0, 0
}

// RouteInput routes one input event the way a host that draws its own
// windows and menus does: through the windows (locating mouse events and
// focusing on press), then the menus (shortcuts, context menus at the
// pointer's screen position). It returns the events to deliver, in order.
// Host plumbing, not application API.
func RouteInput[S any](menus *MenuHost, windows *WindowHost, app App[S], ev Event, width, height int) []Event {
	var out []Event
	for _, ev := range windows.Route(ev) {
		m, mouse := ev.(MouseEvent)
		if !mouse || m.Window == "" {
			if ev, ok := RouteMenus(menus, app, ev, width, height); ok {
				out = append(out, ev)
			}
			continue
		}
		// The menu opens at the pointer on screen; the app gets the event
		// in its window's frame.
		ox, oy := windows.origin(m.Window)
		screen := m
		screen.X, screen.Y = m.X+ox, m.Y+oy
		routed, ok := RouteMenus(menus, app, screen, width, height)
		switch {
		case !ok:
		case routed == Event(screen):
			out = append(out, m)
		default:
			out = append(out, routed)
		}
	}
	return out
}
