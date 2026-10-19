package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/ebitengine/purego/objc"
)

// AppKit must run on the process's main thread.
func init() { runtime.LockOSThread() }

const (
	nsApplicationActivationPolicyRegular = 0
	nsWindowStyleMask                    = 1 | 2 | 4 | 8 // titled, closable, miniaturizable, resizable
	nsBackingStoreBuffered               = 2
	nsBitmapImageFileTypePNG             = 4
)

var windowSize = CGSize{Width: 760, Height: 380}

func main() {
	snapshot := flag.String("snapshot", "", "render one frame to this PNG path and exit")
	scale := flag.Float64("scale", 2, "pixel scale for -snapshot")
	verbose := flag.Bool("v", false, "log text-input callbacks to stderr")
	flag.Parse()
	if err := run(*snapshot, *scale, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "gui-probe:", err)
		os.Exit(1)
	}
}

func run(snapshot string, scale float64, verbose bool) error {
	if err := loadFrameworks(); err != nil {
		return err
	}
	pool := alloc("NSAutoreleasePool").Send(sel("init"))
	defer pool.Send(sel("drain"))

	app := class("NSApplication").Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), nsApplicationActivationPolicyRegular)

	cls, err := registerViewClass()
	if err != nil {
		return err
	}
	theView.verbose = verbose
	view := objc.ID(cls).Send(sel("alloc")).Send(sel("initWithFrame:"), CGRect{Size: windowSize})
	if snapshot != "" {
		return writeSnapshot(view, snapshot, scale)
	}

	window := alloc("NSWindow").Send(sel("initWithContentRect:styleMask:backing:defer:"),
		CGRect{Size: windowSize}, uint(nsWindowStyleMask), uint(nsBackingStoreBuffered), false)
	window.Send(sel("setTitle:"), nsString("Flatte GUI probe"))
	window.Send(sel("setContentView:"), view)
	window.Send(sel("makeFirstResponder:"), view)
	window.Send(sel("center"))
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))

	delegate, err := newAppDelegate()
	if err != nil {
		return err
	}
	app.Send(sel("setDelegate:"), delegate)
	app.Send(sel("setMainMenu:"), newMainMenu())
	app.Send(sel("activateIgnoringOtherApps:"), true)
	app.Send(sel("run"))
	return nil
}

// newAppDelegate quits the app when its only window closes.
func newAppDelegate() (objc.ID, error) {
	cls, err := objc.RegisterClass("FlatProbeAppDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{
		Cmd: sel("applicationShouldTerminateAfterLastWindowClosed:"),
		Fn:  func(objc.ID, objc.SEL, objc.ID) bool { return true },
	}})
	if err != nil {
		return 0, err
	}
	return objc.ID(cls).Send(sel("new")), nil
}

// newMainMenu provides ⌘Q; without a main menu AppKit has no quit shortcut.
func newMainMenu() objc.ID {
	quit := alloc("NSMenuItem").Send(sel("initWithTitle:action:keyEquivalent:"),
		nsString("Quit Flatte GUI probe"), sel("terminate:"), nsString("q"))
	appMenu := alloc("NSMenu").Send(sel("init"))
	appMenu.Send(sel("addItem:"), quit)
	appItem := alloc("NSMenuItem").Send(sel("init"))
	appItem.Send(sel("setSubmenu:"), appMenu)
	bar := alloc("NSMenu").Send(sel("init"))
	bar.Send(sel("addItem:"), appItem)
	return bar
}

// writeSnapshot renders view offscreen at scale pixels per point to a PNG.
func writeSnapshot(view objc.ID, path string, scale float64) error {
	if scale <= 0 {
		return errors.New("-scale must be positive")
	}
	bounds := objc.Send[CGRect](view, sel("bounds"))
	rep := alloc("NSBitmapImageRep").Send(
		sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bytesPerRow:bitsPerPixel:"),
		uintptr(0), int(bounds.Size.Width*scale), int(bounds.Size.Height*scale),
		8, 4, true, false, nsString("NSCalibratedRGBColorSpace"), 0, 0)
	if rep == 0 {
		return errors.New("could not allocate bitmap")
	}
	defer rep.Send(sel("release"))
	rep.Send(sel("setSize:"), bounds.Size)
	view.Send(sel("cacheDisplayInRect:toBitmapImageRep:"), bounds, rep)
	data := rep.Send(sel("representationUsingType:properties:"), uint(nsBitmapImageFileTypePNG), class("NSDictionary").Send(sel("dictionary")))
	if data == 0 || !objc.Send[bool](data, sel("writeToFile:atomically:"), nsString(path), true) {
		return fmt.Errorf("write %s failed", path)
	}
	return nil
}
