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
	nsBackingStoreBuffered    = 2
	nsTerminateCancel         = 0
	nsEventApplicationDefined = 15
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

func runWindow[S any](ctx context.Context, app flatte.App[S], opts Options) error {
	if current != nil {
		return errors.New("window: a window is already running")
	}
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
	width, height := float64(cols)*m.Column, float64(lines)*m.Line
	view := objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), cgRect(0, 0, width, height))
	defer view.Send(sel("release"))
	// Pointer motion without a button, for hover, pointer shapes, and
	// tooltips: entered/exited and moved, while key, over the visible rect.
	tracking := alloc("NSTrackingArea").Send(sel("initWithRect:options:owner:userInfo:"),
		cgRect(0, 0, 0, 0), uint(nsTrackingMouseEnteredAndExited|nsTrackingMouseMoved|nsTrackingActiveInKeyWindow|nsTrackingInVisibleRect), view, objc.ID(0))
	view.Send(sel("addTrackingArea:"), tracking)
	tracking.Send(sel("release"))
	win := alloc("NSWindow").Send(sel("initWithContentRect:styleMask:backing:defer:"),
		cgRect(0, 0, width, height), uint(nsWindowStyleMask), uint(nsBackingStoreBuffered), false)
	win.Send(sel("setReleasedWhenClosed:"), false)
	defer win.Send(sel("release"))
	title := opts.Title
	if title == "" {
		title = "Flatte"
	}
	win.Send(sel("setTitle:"), nsString(title))
	win.Send(sel("setContentView:"), view)
	win.Send(sel("setContentMinSize:"), CGSize{Width: 20 * m.Column, Height: 4 * m.Line})

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
	current = &windowState{ui: h, m: m, th: th, follow: opts.Theme == nil, dark: dark, window: win, title: title}
	st := current
	h.text = windowText{th: func() theme { return st.th }}
	h.clip = pasteboard{}
	defer func() {
		if st := current; st != nil && st.blink != nil {
			st.blink.Stop()
		}
		current = nil
	}()

	delegate, err := newDelegate(stop)
	if err != nil {
		return err
	}
	defer delegate.Send(sel("release"))
	nsApp.Send(sel("setDelegate:"), delegate)
	win.Send(sel("setDelegate:"), delegate)
	nsApp.Send(sel("setMainMenu:"), mainMenu(title))

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			post(stop)
		case <-done:
		}
	}()

	win.Send(sel("center"))
	win.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	win.Send(sel("makeFirstResponder:"), view)
	h.start(width, height)
	if !stopped {
		nsApp.Send(sel("activateIgnoringOtherApps:"), true)
		nsApp.Send(sel("run"))
	}
	win.Send(sel("setDelegate:"), objc.ID(0))
	nsApp.Send(sel("setDelegate:"), objc.ID(0))
	win.Send(sel("orderOut:"), objc.ID(0))
	h.close()
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
				{Cmd: sel("windowWillClose:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
					if delegateStop != nil {
						delegateStop()
					}
				}},
			})
	})
	if delegateClassErr != nil {
		return 0, delegateClassErr
	}
	return objc.ID(delegateClass).Send(sel("new")), nil
}

// mainMenu provides the application menu with ⌘Q (and ⌘W to close).
func mainMenu(title string) objc.ID {
	appMenu := alloc("NSMenu").Send(sel("init"))
	appMenu.Send(sel("addItem:"), alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"),
		nsString("Quit "+title), sel("terminate:"), nsString("q")))
	appItem := alloc("NSMenuItem").Send(sel("init"))
	appItem.Send(sel("setSubmenu:"), appMenu)
	winMenu := alloc("NSMenu").Send(sel("initWithTitle:"), nsString("Window"))
	winMenu.Send(sel("addItem:"), alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"),
		nsString("Close"), sel("performClose:"), nsString("w")))
	winItem := alloc("NSMenuItem").Send(sel("init"))
	winItem.Send(sel("setSubmenu:"), winMenu)
	bar := alloc("NSMenu").Send(sel("init"))
	bar.Send(sel("addItem:"), appItem)
	bar.Send(sel("addItem:"), winItem)
	return bar
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
	return renderImage(scene, th, m, width, height, scale, h.frameCursor(), "", ""), nil
}

// renderImage draws scene offscreen into an RGBA image of width×height
// points at scale pixels per point, with the controls hovered and pressed
// as given.
func renderImage(scene layout.Scene, th theme, m layout.Metrics, width, height, scale float64, cursor *flatte.Cursor, hovered, pressed string) *image.RGBA {
	pw, ph := int(math.Ceil(width*scale)), int(math.Ceil(height*scale))
	cs := cgColorSpaceCreateDeviceRGB()
	defer cgColorSpaceRelease(cs)
	ctx := cgBitmapContextCreate(0, uintptr(pw), uintptr(ph), 8, 0, cs, cgImageAlphaPremultipliedLast|cgBitmapByteOrder32Big)
	defer cgContextRelease(ctx)
	// Top-left origin points, as in the flipped view.
	cgContextTranslateCTM(ctx, 0, float64(ph))
	cgContextScaleCTM(ctx, scale, -scale)
	p := &painter{ctx: ctx, m: m, th: th, scale: scale, hovered: hovered, pressed: pressed}
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
