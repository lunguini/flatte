package window

import (
	"context"
	"errors"
	"image"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// AppKit must run on the process's main thread; keep the main goroutine
// there.
func init() { runtime.LockOSThread() }

var errNotMainThread = errors.New("window: Run must be called from the main goroutine")

const (
	nsActivationPolicyRegular = 0
	nsWindowStyleMask         = 1 | 2 | 4 | 8 // titled, closable, miniaturizable, resizable
	nsFullSizeContentView     = 1 << 15
	nsWindowTitleHidden       = 1
	// NSVisualEffectView: the window's own background material, blending
	// what is behind the window, always active; views sized with their
	// window.
	nsMaterialUnderWindow     = 21
	nsBlendBehindWindow       = 0
	nsEffectActive            = 1
	nsViewSizable             = 2 | 16
	nsBackingStoreBuffered    = 2
	nsTerminateCancel         = 0
	nsEventApplicationDefined = 15
	// NSWindowCollectionBehaviorFullScreenPrimary.
	nsWindowCollectionBehaviorFullScreenPrimary = 1 << 7
)

// --- running functions on the main thread ---

var (
	postMu       sync.Mutex
	posted       []func()
	postCallback uintptr
	postOnce     sync.Once
)

// post runs f on the main thread, from any goroutine.
func post(f func()) {
	postOnce.Do(func() {
		postCallback = purego.NewCallback(func(uintptr) { runPosted() })
	})
	postMu.Lock()
	posted = append(posted, f)
	postMu.Unlock()
	dispatchAsyncF(dispatchMainQueue, 0, postCallback)
}

func runPosted() {
	postMu.Lock()
	fs := posted
	posted = nil
	postMu.Unlock()
	for _, f := range fs {
		withPool(f)
	}
}

// --- Run ---

func run[S any](ctx context.Context, app flatte.App[S], opts Options) error {
	if err := loadFrameworks(); err != nil {
		return err
	}
	if pthreadMainNP() == 0 {
		return errNotMainThread
	}
	var err error
	withPool(func() { err = runWindow(ctx, app, opts) })
	return err
}

// running is set while Run runs: one app per process.
var running bool

// chrome is how a window's frame looks: Options.HideTitleBar and
// Translucent, and dark for the blur's appearance.
type chrome struct{ hideTitle, translucent, dark bool }

// makeWindow creates a titled, resizable window of cols×lines cells of m,
// with no smaller than minCols×minLines (at least 20×4), and its view of
// class cls tracking the pointer and accepting drops. When autosave is set
// the window saves its frame under that name and restores it (centered
// otherwise). The caller releases both.
func makeWindow(cls objc.Class, m layout.Metrics, title string, cols, lines, minCols, minLines int, autosave string, ch chrome) (win, view objc.ID) {
	width, height := float64(cols)*m.Column, float64(lines)*m.Line
	view = objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), cgRect(0, 0, width, height))
	// Pointer motion without a button, for hover, pointer shapes, and
	// tooltips: entered/exited and moved, while key, over the visible rect.
	tracking := alloc("NSTrackingArea").Send(sel("initWithRect:options:owner:userInfo:"),
		cgRect(0, 0, 0, 0), uint(nsTrackingMouseEnteredAndExited|nsTrackingMouseMoved|nsTrackingActiveInKeyWindow|nsTrackingInVisibleRect), view, objc.ID(0))
	view.Send(sel("addTrackingArea:"), tracking)
	tracking.Send(sel("release"))
	view.Send(sel("registerForDraggedTypes:"), dragTypes())
	mask := uint(nsWindowStyleMask)
	if ch.hideTitle {
		mask |= nsFullSizeContentView
	}
	win = alloc("NSWindow").Send(sel("initWithContentRect:styleMask:backing:defer:"),
		cgRect(0, 0, width, height), mask, uint(nsBackingStoreBuffered), false)
	win.Send(sel("setReleasedWhenClosed:"), false)
	win.Send(sel("setTitle:"), nsString(title))
	if ch.hideTitle {
		win.Send(sel("setTitlebarAppearsTransparent:"), true)
		win.Send(sel("setTitleVisibility:"), int(nsWindowTitleHidden))
	}
	if ch.translucent {
		// The view draws over a blur of what is behind the window.
		effect := alloc("NSVisualEffectView").Send(sel("initWithFrame:"), cgRect(0, 0, width, height))
		effect.Send(sel("setMaterial:"), int(nsMaterialUnderWindow))
		effect.Send(sel("setBlendingMode:"), int(nsBlendBehindWindow))
		effect.Send(sel("setState:"), int(nsEffectActive))
		view.Send(sel("setAutoresizingMask:"), uint(nsViewSizable))
		effect.Send(sel("addSubview:"), view)
		win.Send(sel("setContentView:"), effect)
		effect.Send(sel("release"))
		win.Send(sel("setOpaque:"), false)
		win.Send(sel("setBackgroundColor:"), class("NSColor").Send(sel("clearColor")))
		appearance := "NSAppearanceNameAqua"
		if ch.dark {
			appearance = "NSAppearanceNameDarkAqua"
		}
		win.Send(sel("setAppearance:"), class("NSAppearance").Send(sel("appearanceNamed:"), nsString(appearance)))
	} else {
		win.Send(sel("setContentView:"), view)
	}
	win.Send(sel("setCollectionBehavior:"), uint(nsWindowCollectionBehaviorFullScreenPrimary))
	win.Send(sel("setContentMinSize:"), CGSize{Width: float64(max(minCols, 20)) * m.Column, Height: float64(max(minLines, 4)) * m.Line})
	win.Send(sel("center"))
	if autosave != "" {
		win.Send(sel("setFrameAutosaveName:"), nsString(autosave))
		win.Send(sel("setFrameUsingName:"), nsString(autosave))
	}
	return win, view
}

// nativeWindows is the window's windowBackend: secondary windows, each with
// its own view and state, sharing the main window's look.
type nativeWindows struct {
	cls      objc.Class
	m        layout.Metrics
	th       func() theme
	follow   bool
	delegate objc.ID
	// focused and closeRequested report window id becoming key and the
	// user asking to close it.
	focused, closeRequested func(id string)
}

func (n nativeWindows) openWindow(w flatte.Window, s ui) func() {
	cols, lines := w.Width, w.Height
	if cols <= 0 {
		cols = 60
	}
	if lines <= 0 {
		lines = 20
	}
	win, view := makeWindow(n.cls, n.m, w.ID, max(cols, w.MinWidth), max(lines, w.MinHeight), w.MinWidth, w.MinHeight, w.Autosave, chrome{})
	th := n.th()
	st := &windowState{ui: s, m: n.m, th: th, follow: n.follow, dark: isDark(view), window: win, view: view, id: w.ID, title: w.ID}
	id := w.ID
	st.focused = func() { n.focused(id) }
	st.closeRequested = func() { n.closeRequested(id) }
	register(st)
	win.Send(sel("setDelegate:"), n.delegate)
	if w.Autosave == "" {
		// Cascade from the key window so windows do not stack exactly.
		if key := class("NSApplication").Send(sel("sharedApplication")).Send(sel("keyWindow")); key != 0 {
			f := objc.Send[CGRect](key, sel("frame"))
			win.Send(sel("cascadeTopLeftFromPoint:"), CGPoint{X: f.Origin.X + 24, Y: f.Origin.Y + f.Size.Height - 24})
		}
	}
	win.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	win.Send(sel("makeFirstResponder:"), view)
	return func() { view.Send(sel("setNeedsDisplay:"), true) }
}

func (n nativeWindows) closeWindow(id string) {
	for _, st := range states {
		if st.id != id || id == "" {
			continue
		}
		unregister(st)
		st.window.Send(sel("setDelegate:"), objc.ID(0))
		st.window.Send(sel("orderOut:"), objc.ID(0))
		st.window.Send(sel("close"))
		st.view.Send(sel("release"))
		st.window.Send(sel("release"))
		return
	}
}

func runWindow[S any](ctx context.Context, app flatte.App[S], opts Options) error {
	if running {
		return errors.New("window: a window is already running")
	}
	running = true
	defer func() { running = false }()
	cols, lines := opts.size()
	nsApp := class("NSApplication").Send(sel("sharedApplication"))
	nsApp.Send(sel("setActivationPolicy:"), nsActivationPolicyRegular)
	cls, err := registerViewClass()
	if err != nil {
		return err
	}
	// Dark and Light share their type, so following the system changes
	// colors only, never the metrics.
	dark := isDark(nsApp)
	th := newTheme(themeFor(opts.Theme, dark))
	m := th.metrics()
	title := opts.Title
	if title == "" {
		title = "Flatte"
	}
	win, view := makeWindow(cls, m, title, max(cols, opts.MinWidth), max(lines, opts.MinHeight), opts.MinWidth, opts.MinHeight, opts.Autosave,
		chrome{hideTitle: opts.HideTitleBar, translucent: opts.Translucent, dark: dark})
	defer win.Send(sel("release"))
	defer view.Send(sel("release"))
	b := objc.Send[CGRect](view, sel("bounds"))
	width, height := b.Size.Width, b.Size.Height

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		nsApp.Send(sel("stop:"), objc.ID(0))
		wake(nsApp)
	}
	h := newHost(ctx, app, m, post, stop, func() { view.Send(sel("setNeedsDisplay:"), true) })
	st := &windowState{ui: h.surface, m: m, th: th, follow: opts.Theme == nil, dark: dark, window: win, view: view, title: title, translucent: opts.Translucent, hideTitle: opts.HideTitleBar}
	st.focused = func() { h.focused("") }
	register(st)
	defer func() {
		for _, other := range states {
			unregister(other)
		}
	}()
	h.text = windowText{th: func() theme { return st.th }}
	h.clip = pasteboard{}

	delegate, err := newDelegate(stop)
	if err != nil {
		return err
	}
	defer delegate.Send(sel("release"))
	nsApp.Send(sel("setDelegate:"), delegate)
	win.Send(sel("setDelegate:"), delegate)
	if _, err := newMenuTarget(); err != nil {
		return err
	}
	menus := nativeMenus{title: title}
	menus.setBar(nil)
	h.menus = menus
	h.dialogs = nativeDialogs{}
	h.windows = nativeWindows{cls: cls, m: m, th: func() theme { return st.th }, follow: opts.Theme == nil, delegate: delegate,
		focused: h.focused, closeRequested: h.closeRequested}
	barChosen = h.menuChosen
	defer func() { barChosen = nil }()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			post(stop)
		case <-done:
		}
	}()

	win.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	win.Send(sel("makeFirstResponder:"), view)
	h.start(width, height)
	if !stopped {
		nsApp.Send(sel("activateIgnoringOtherApps:"), true)
		nsApp.Send(sel("run"))
	}
	h.close()
	win.Send(sel("setDelegate:"), objc.ID(0))
	nsApp.Send(sel("setDelegate:"), objc.ID(0))
	win.Send(sel("orderOut:"), objc.ID(0))
	return nil
}

// wake posts an empty application event so a stopped run loop returns now
// rather than at the next real event.
func wake(nsApp objc.ID) {
	ev := class("NSEvent").Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"),
		uint(nsEventApplicationDefined), CGPoint{}, uint(0), 0.0, 0, objc.ID(0), int16(0), 0, 0)
	nsApp.Send(sel("postEvent:atStart:"), ev, true)
}

var (
	delegateClassOnce sync.Once
	delegateClass     objc.Class
	delegateClassErr  error
	delegateStop      func()
)

// newDelegate is the app and window delegate: quitting from the menu or
// closing the window stops the run loop so Run can return (and run OnExit)
// instead of AppKit exiting the process.
func newDelegate(stop func()) (objc.ID, error) {
	delegateStop = stop
	delegateClassOnce.Do(func() {
		delegateClass, delegateClassErr = objc.RegisterClass("FlatteWindowDelegate", objc.GetClass("NSObject"),
			[]*objc.Protocol{objc.GetProtocol("NSApplicationDelegate"), objc.GetProtocol("NSWindowDelegate")}, nil,
			[]objc.MethodDef{
				{Cmd: sel("applicationShouldTerminate:"), Fn: func(objc.ID, objc.SEL, objc.ID) uint {
					if delegateStop != nil {
						delegateStop()
					}
					return nsTerminateCancel
				}},
				// A secondary window's close button asks the app (it closes
				// when the app stops declaring it); the main window closes,
				// which ends Run.
				{Cmd: sel("windowShouldClose:"), Fn: func(_ objc.ID, _ objc.SEL, win objc.ID) bool {
					if st := byWindow[win]; st != nil && st.closeRequested != nil {
						st.closeRequested()
						return false
					}
					return true
				}},
				{Cmd: sel("windowWillClose:"), Fn: func(_ objc.ID, _ objc.SEL, note objc.ID) {
					if st := byWindow[note.Send(sel("object"))]; st != nil && st.id != "" {
						return
					}
					if delegateStop != nil {
						delegateStop()
					}
				}},
				{Cmd: sel("windowDidBecomeKey:"), Fn: func(_ objc.ID, _ objc.SEL, note objc.ID) {
					if st := byWindow[note.Send(sel("object"))]; st != nil && st.focused != nil {
						st.focused()
					}
				}},
			})
	})
	if delegateClassErr != nil {
		return 0, delegateClassErr
	}
	return objc.ID(delegateClass).Send(sel("new")), nil
}

// --- Snapshot ---

const (
	cgImageAlphaPremultipliedLast = 1
	cgBitmapByteOrder32Big        = 4 << 12
)

func snapshot[S any](app flatte.App[S], opts Options, scale float64) (image.Image, error) {
	if !(scale > 0) || math.IsInf(scale, 0) {
		return nil, errors.New("window: snapshot scale must be positive")
	}
	if err := loadFrameworks(); err != nil {
		return nil, err
	}
	th := newTheme(themeFor(opts.Theme, true))
	m := th.metrics()
	cols, lines := opts.size()
	width, height := float64(cols)*m.Column, float64(lines)*m.Line
	// The first frame only: async results are dropped, quitting is moot.
	h := newHost(context.Background(), app, m, func(func()) {}, func() {}, func() {})
	defer h.cancel()
	h.start(width, height)
	scene := h.render()
	return renderImage(scene, th, m, width, height, scale, h.frameCursor(), presentation{}), nil
}

// renderImage draws scene offscreen into an RGBA image of width×height
// points at scale pixels per point, with pres drawn over it.
func renderImage(scene layout.Scene, th theme, m layout.Metrics, width, height, scale float64, cursor *flatte.Cursor, pres presentation) *image.RGBA {
	pw, ph := int(math.Ceil(width*scale)), int(math.Ceil(height*scale))
	cs := cgColorSpaceCreateDeviceRGB()
	defer cgColorSpaceRelease(cs)
	ctx := cgBitmapContextCreate(0, uintptr(pw), uintptr(ph), 8, 0, cs, cgImageAlphaPremultipliedLast|cgBitmapByteOrder32Big)
	defer cgContextRelease(ctx)
	// Top-left origin points, as in the flipped view.
	cgContextTranslateCTM(ctx, 0, float64(ph))
	cgContextScaleCTM(ctx, scale, -scale)
	p := &painter{ctx: ctx, m: m, th: th, scale: scale, presentation: pres}
	withPool(func() {
		// Fill the whole bitmap, including the partial pixel the point size
		// rounds up to.
		p.draw(scene, float64(pw)/scale, float64(ph)/scale)
		if cursor != nil {
			p.clip = layout.PointRect{W: width, H: height}
			p.caretAt(float64(cursor.X)*m.Column, float64(cursor.Y)*m.Line, m.Line, th.font(fontKey{mono: true}))
		}
		p.paintCaret() // a snapshot shows the caret, unblinking
	})
	img := image.NewRGBA(image.Rect(0, 0, pw, ph))
	data, stride := cgBitmapContextGetData(ctx), int(cgBitmapContextGetBytesPer(ctx))
	src := unsafe.Slice((*byte)(data), stride*ph)
	for y := range ph {
		copy(img.Pix[y*img.Stride:y*img.Stride+pw*4], src[y*stride:y*stride+pw*4])
	}
	return img
}

// pasteboard is the system clipboard: NSPasteboard's general pasteboard
// (or, for tests, a private one by name), as plain text.
type pasteboard struct{ name string }

// nsPasteboardTypeString is NSPasteboardTypeString's value.
const nsPasteboardTypeString = "public.utf8-plain-text"

func (b pasteboard) board() objc.ID {
	if b.name == "" {
		return class("NSPasteboard").Send(sel("generalPasteboard"))
	}
	return class("NSPasteboard").Send(sel("pasteboardWithName:"), nsString(b.name))
}

func (b pasteboard) get() string {
	var text string
	withPool(func() {
		text = plainString(b.board().Send(sel("stringForType:"), nsString(nsPasteboardTypeString)))
	})
	return text
}

func (b pasteboard) set(text string) {
	withPool(func() {
		pb := b.board()
		pb.Send(sel("clearContents"))
		pb.Send(sel("setString:forType:"), nsString(text), nsString(nsPasteboardTypeString))
	})
}

// NSTrackingAreaOptions.
const (
	nsTrackingMouseEnteredAndExited = 0x01
	nsTrackingMouseMoved            = 0x02
	nsTrackingActiveInKeyWindow     = 0x20
	nsTrackingInVisibleRect         = 0x200
)
