package window

import (
	"errors"
	"fmt"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
)

const maxDesktopSelection = 1024 * 1024
const desktopHotkeySignature = 0x466c7444 // 'FltD'

type carbonEventType struct {
	_           structs.HostLayout
	Class, Kind uint32
}
type carbonHotkeyID struct {
	_             structs.HostLayout
	Signature, ID uint32
}

var desktopNative struct {
	once              sync.Once
	err               error
	target            objc.Class
	callback          uintptr
	applicationTarget func() uintptr
	installHandler    func(uintptr, uintptr, uint32, *carbonEventType, uintptr, *uintptr) int32
	removeHandler     func(uintptr) int32
	registerHotkey    func(uint32, uint32, carbonHotkeyID, uintptr, uint32, *uintptr) int32
	unregisterHotkey  func(uintptr) int32
	eventParameter    func(uintptr, uint32, uint32, *uint32, uint32, *uint32, *carbonHotkeyID) int32
	trusted           func() bool
	trustPrompt       func(objc.ID) bool
	systemElement     func() uintptr
	copyAttribute     func(uintptr, objc.ID, *objc.ID) int32
	messagingTimeout  func(uintptr, float32) int32
	typeID            func(uintptr) uint
	stringTypeID      func() uint
	elementTypeID     func() uint
	trustPromptKey    objc.ID
}

// Only the main AppKit thread accesses desktopCurrent or nativeDesktop.
var desktopCurrent *nativeDesktop

type nativeDesktop struct {
	app, window, view, target, status, previous objc.ID
	state                                       *windowState
	send                                        func(flatte.Event)
	menuIDs                                     []string
	hotkey, handler                             uintptr
	closed                                      bool
}

func loadDesktopNative() error {
	desktopNative.once.Do(func() {
		carbon, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if err != nil {
			desktopNative.err = err
			return
		}
		services, err := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if err != nil {
			desktopNative.err = err
			return
		}
		cf, err := purego.Dlopen(frameworkCoreFoundation, purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if err != nil {
			desktopNative.err = err
			return
		}
		for _, b := range []struct {
			fn   any
			lib  uintptr
			name string
		}{
			{&desktopNative.applicationTarget, carbon, "GetApplicationEventTarget"},
			{&desktopNative.installHandler, carbon, "InstallEventHandler"},
			{&desktopNative.removeHandler, carbon, "RemoveEventHandler"},
			{&desktopNative.registerHotkey, carbon, "RegisterEventHotKey"},
			{&desktopNative.unregisterHotkey, carbon, "UnregisterEventHotKey"},
			{&desktopNative.eventParameter, carbon, "GetEventParameter"},
			{&desktopNative.trusted, services, "AXIsProcessTrusted"},
			{&desktopNative.trustPrompt, services, "AXIsProcessTrustedWithOptions"},
			{&desktopNative.systemElement, services, "AXUIElementCreateSystemWide"},
			{&desktopNative.copyAttribute, services, "AXUIElementCopyAttributeValue"},
			{&desktopNative.messagingTimeout, services, "AXUIElementSetMessagingTimeout"},
			{&desktopNative.elementTypeID, services, "AXUIElementGetTypeID"},
			{&desktopNative.typeID, cf, "CFGetTypeID"},
			{&desktopNative.stringTypeID, cf, "CFStringGetTypeID"},
		} {
			if _, err := purego.Dlsym(b.lib, b.name); err != nil {
				desktopNative.err = fmt.Errorf("window: desktop %s: %w", b.name, err)
				return
			}
			purego.RegisterLibFunc(b.fn, b.lib, b.name)
		}
		desktopNative.trustPromptKey, err = constant(services, "kAXTrustedCheckOptionPrompt")
		if err != nil {
			desktopNative.err = err
			return
		}
		desktopNative.callback = purego.NewCallback(func(_, event, _ uintptr) int32 {
			d := desktopCurrent
			if d == nil || d.closed {
				return -9874
			} // eventNotHandledErr
			var id carbonHotkeyID
			if desktopNative.eventParameter(event, 0x2d2d2d2d, 0x686b6964, nil, uint32(unsafe.Sizeof(id)), nil, &id) != 0 || id.Signature != desktopHotkeySignature || id.ID != 1 {
				return -9874
			}
			d.captureSelection()
			return 0
		})
		desktopNative.target, desktopNative.err = objc.RegisterClass("FlatteDesktopProvider", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("chooseStatus:"), Fn: func(_ objc.ID, _ objc.SEL, item objc.ID) {
				d := desktopCurrent
				if d == nil || d.closed {
					return
				}
				tag := objc.Send[int](item, sel("tag"))
				if tag >= 0 && tag < len(d.menuIDs) {
					id := d.menuIDs[tag]
					post(func() {
						if !d.closed {
							d.send(flatte.MenuEvent{ID: id})
						}
					})
				}
			}},
			{Cmd: sel("receiveText:userData:error:"), Fn: func(_ objc.ID, _ objc.SEL, board, userData objc.ID, errorOut *objc.ID) {
				d := desktopCurrent
				if d == nil || d.closed {
					return
				}
				value := board.Send(sel("stringForType:"), nsString(nsPasteboardTypeString))
				if value == 0 {
					value = board.Send(sel("stringForType:"), nsString("NSStringPboardType"))
				}
				text, captureError := selectedDesktopText(value)
				id := plainString(userData)
				if id == "" {
					id = "capture"
				}
				ev := flatte.DesktopEvent{ID: id, Source: "service", Text: text, Error: captureError}
				if text == "" && ev.Error == "" {
					ev.Error = "The source application supplied no selected text."
				} else if len(text) > maxDesktopSelection {
					ev.Text = ""
					ev.Error = "The selection is too large (maximum 1 MiB)."
				}
				if ev.Error != "" && errorOut != nil {
					*errorOut = nsString(ev.Error)
				}
				// Copy the explicit pasteboard value before returning to AppKit; mutation
				// and presentation are serialized on the same main-thread host.
				post(func() {
					if !d.closed {
						d.deliverCapture(ev)
					}
				})
			}},
		})
	})
	return desktopNative.err
}

func newNativeDesktop(app, win, view objc.ID, st *windowState, send func(flatte.Event), title string) (*nativeDesktop, error) {
	if err := loadDesktopNative(); err != nil {
		return nil, err
	}
	if desktopCurrent != nil {
		return nil, errors.New("window: desktop utility already running")
	}
	d := &nativeDesktop{app: app, window: win, view: view, state: st, send: send}
	d.target = objc.ID(desktopNative.target).Send(sel("new"))
	statusBar := class("NSStatusBar").Send(sel("systemStatusBar"))
	d.status = statusBar.Send(sel("statusItemWithLength:"), float64(-1)) // variable length
	d.status.Send(sel("retain"))
	button := d.status.Send(sel("button"))
	icon := class("NSImage").Send(sel("imageWithSystemSymbolName:accessibilityDescription:"), nsString("text.bubble"), nsString(title))
	if icon != 0 {
		icon.Send(sel("setTemplate:"), true)
		button.Send(sel("setImage:"), icon)
	} else {
		button.Send(sel("setTitle:"), nsString(title))
	}
	button.Send(sel("setToolTip:"), nsString(title))
	spec := carbonEventType{Class: 0x6b657962, Kind: 5}
	if status := desktopNative.installHandler(desktopNative.applicationTarget(), desktopNative.callback, 1, &spec, 0, &d.handler); status != 0 {
		d.close()
		return nil, fmt.Errorf("window: install global shortcut handler: %d", status)
	}
	win.Send(sel("setLevel:"), int(3)) // NSFloatingWindowLevel
	win.Send(sel("setRestorable:"), false)
	win.Send(sel("setCollectionBehavior:"), uint((1<<0)|(1<<8))) // active space/fullscreen auxiliary
	desktopCurrent = d
	return d, nil
}

func (d *nativeDesktop) activate(shortcut *flatte.KeyEvent) {
	d.app.Send(sel("setServicesProvider:"), d.target)
	d.setShortcut(shortcut)
}

func (d *nativeDesktop) setShortcut(key *flatte.KeyEvent) {
	if d.hotkey != 0 {
		desktopNative.unregisterHotkey(d.hotkey)
		d.hotkey = 0
	}
	if key == nil {
		return
	}
	code, mods, ok := desktopHotkey(*key)
	if !ok {
		d.send(flatte.DesktopEvent{ID: "shortcut", Source: "shortcut", Error: "Unsupported global shortcut. Use Space, Enter, Escape, Tab, or F1–F12 with Command, Control, or Option."})
		return
	}
	if status := desktopNative.registerHotkey(code, mods, carbonHotkeyID{Signature: desktopHotkeySignature, ID: 1}, desktopNative.applicationTarget(), 0, &d.hotkey); status != 0 {
		d.hotkey = 0
		d.send(flatte.DesktopEvent{ID: "shortcut", Source: "shortcut", Error: "This global shortcut is unavailable. Choose another shortcut in Preferences."})
	}
}

func (d *nativeDesktop) deliverCapture(ev flatte.DesktopEvent) {
	if d.closed {
		return
	}
	// Capture is input, not a presentation policy. Apps choose whether to
	// open a preview, show an error, or forward the selection in background.
	d.send(ev)
}

func (d *nativeDesktop) captureSelection() {
	ev := flatte.DesktopEvent{ID: "capture", Source: "shortcut"}
	if !desktopNative.trusted() {
		ev.Error = "Selection capture needs Accessibility permission. Enable it in System Settings, or use the Send to Claude Service."
	} else {
		system := desktopNative.systemElement()
		defer cfRelease(system)
		desktopNative.messagingTimeout(system, 0.3)
		var focused objc.ID
		if desktopNative.copyAttribute(system, nsString("AXFocusedUIElement"), &focused) == 0 && focused != 0 {
			defer cfRelease(uintptr(focused))
			if desktopNative.typeID(uintptr(focused)) == desktopNative.elementTypeID() {
				desktopNative.messagingTimeout(uintptr(focused), 0.3)
				var selected objc.ID
				if desktopNative.copyAttribute(uintptr(focused), nsString("AXSelectedText"), &selected) == 0 && selected != 0 {
					defer cfRelease(uintptr(selected))
					if desktopNative.typeID(uintptr(selected)) == desktopNative.stringTypeID() {
						ev.Text, ev.Error = selectedDesktopText(selected)
					}
				}
			}
		}
		if ev.Text == "" && ev.Error == "" {
			ev.Error = "This application did not expose selected text. Try its Send to Claude Service."
		} else if len(ev.Text) > maxDesktopSelection {
			ev.Text = ""
			ev.Error = "The selection is too large (maximum 1 MiB)."
		}
	}
	post(func() {
		if !d.closed {
			d.deliverCapture(ev)
		}
	})
}

// selectedDesktopText copies NSString's exact UTF-8 bytes, including embedded
// NULs. UTF8String/goString would silently truncate the user's selection.
func selectedDesktopText(value objc.ID) (string, string) {
	if value == 0 {
		return "", ""
	}
	length := objc.Send[uint](value, sel("lengthOfBytesUsingEncoding:"), uint(4))
	if length > maxDesktopSelection {
		return "", "The selection is too large (maximum 1 MiB)."
	}
	if length == 0 {
		return "", ""
	}
	data := value.Send(sel("dataUsingEncoding:"), uint(4))
	if data == 0 {
		return "", "The selected text could not be decoded."
	}
	bytes := objc.Send[*byte](data, sel("bytes"))
	return string(unsafe.Slice(bytes, int(length))), ""
}

func (d *nativeDesktop) show() {
	front := class("NSWorkspace").Send(sel("sharedWorkspace")).Send(sel("frontmostApplication"))
	if front != 0 && objc.Send[int32](front, sel("processIdentifier")) != objc.Send[int32](class("NSProcessInfo").Send(sel("processInfo")), sel("processIdentifier")) {
		if d.previous != 0 {
			d.previous.Send(sel("release"))
		}
		d.previous = front.Send(sel("retain"))
	}
	// Draw the current shared semantic frame before revealing the window; an
	// old selection must never flash while a new capture is being handled.
	d.view.Send(sel("setNeedsDisplay:"), true)
	d.view.Send(sel("displayIfNeeded"))
	d.app.Send(sel("activateIgnoringOtherApps:"), true)
	d.window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	d.window.Send(sel("makeFirstResponder:"), d.view)
}

func (d *nativeDesktop) hide() {
	d.window.Send(sel("orderOut:"), objc.ID(0))
	d.state.marked = ""
	d.state.caretOK = false
	if d.previous != 0 {
		d.previous.Send(sel("activateWithOptions:"), uint(0))
		d.previous.Send(sel("release"))
		d.previous = 0
	}
}

func (d *nativeDesktop) command(c flatte.DesktopCommand) {
	if d.closed {
		return
	}
	switch c.Kind {
	case "show":
		d.show()
	case "hide":
		d.hide()
	case "shortcut":
		d.setShortcut(c.Shortcut)
	case "permission":
		yes := class("NSNumber").Send(sel("numberWithBool:"), true)
		options := class("NSDictionary").Send(sel("dictionaryWithObject:forKey:"), yes, desktopNative.trustPromptKey)
		status := "pending"
		if desktopNative.trustPrompt(options) {
			status = "granted"
		}
		d.send(flatte.DesktopEvent{ID: "permission", Source: "host", Text: status})
	}
}

func (d *nativeDesktop) close() {
	if d.closed {
		return
	}
	d.closed = true
	if desktopCurrent == d {
		desktopCurrent = nil
	}
	if d.hotkey != 0 {
		desktopNative.unregisterHotkey(d.hotkey)
		d.hotkey = 0
	}
	if d.handler != 0 {
		desktopNative.removeHandler(d.handler)
		d.handler = 0
	}
	if d.app.Send(sel("servicesProvider")) == d.target {
		d.app.Send(sel("setServicesProvider:"), objc.ID(0))
	}
	if d.status != 0 {
		class("NSStatusBar").Send(sel("systemStatusBar")).Send(sel("removeStatusItem:"), d.status)
		d.status.Send(sel("release"))
		d.status = 0
	}
	if d.target != 0 {
		d.target.Send(sel("release"))
		d.target = 0
	}
	if d.previous != 0 {
		d.previous.Send(sel("release"))
		d.previous = 0
	}
	d.send = nil
	d.menuIDs = nil
}

// desktopMenus reuses the existing native menu model, renderer, target IDs,
// and MenuEvent dispatch. No second menu vocabulary or app loop is introduced.
type desktopMenus struct {
	nativeMenus
	desktop *nativeDesktop
}

func (m desktopMenus) setBar(menus []flatte.Menu) {
	m.nativeMenus.setBar(menus)
	var items []flatte.MenuItem
	if len(menus) == 1 {
		items = menus[0].Items
	} else {
		for _, menu := range menus {
			items = append(items, flatte.MenuItem{Label: menu.Title, Items: menu.Items})
		}
	}
	d := m.desktop
	d.menuIDs = nil
	tray := buildMenu(m.title, items, sel("chooseStatus:"), &d.menuIDs)
	// buildMenu normally targets FlatteMenuTarget; status items use the same
	// tag mechanism with their own provider target and ID table.
	retargetMenu(tray, d.target)
	d.status.Send(sel("setMenu:"), tray)
	tray.Send(sel("release"))
}
func retargetMenu(menu, target objc.ID) {
	count := objc.Send[int](menu, sel("numberOfItems"))
	for i := 0; i < count; i++ {
		item := menu.Send(sel("itemAtIndex:"), i)
		item.Send(sel("setTarget:"), target)
		if sub := item.Send(sel("submenu")); sub != 0 {
			retargetMenu(sub, target)
		}
	}
}
