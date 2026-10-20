package window

import (
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// ui is what the view needs from the generic host.
type ui interface {
	resize(width, height float64)
	key(flatte.KeyEvent)
	paste()
	hover(x, y float64) (pointer, string)
	controlState() (hovered, pressed string)
	mouse(x, y float64, button flatte.MouseButton, action flatte.MouseAction, mod flatte.Mod)
	render() layout.Scene
	frameTitle() string
	frameCursor() *flatte.Cursor
}

// windowState is the Go side of the one window a process shows. AppKit calls
// the view's methods with self; there is a single view, so the state is a
// package value, touched only on the main thread.
type windowState struct {
	ui ui
	m  layout.Metrics
	th theme
	// follow re-resolves th from the system appearance (dark) as it
	// changes; set when Options has no Theme.
	follow, dark bool
	window       objc.ID
	title        string
	// marked is the input method's uncommitted text; caret is where the
	// focused input's caret was last drawn, in view coordinates.
	marked  string
	caret   CGRect
	caretOK bool
	wheel   wheelAccumulator
	// caretMoved is when the caret last moved; blink redraws at its next
	// blink phase.
	caretMoved time.Time
	blink      *time.Timer
	// tooltip is the view's tooltip now.
	tooltip string
}

// showPointer sets the pointer shape and the view's tooltip.
func (st *windowState) showPointer(view objc.ID, shape pointer, tooltip string) {
	name := "arrowCursor"
	switch shape {
	case pointerHand:
		name = "pointingHandCursor"
	case pointerText:
		name = "IBeamCursor"
	}
	class("NSCursor").Send(sel(name)).Send(sel("set"))
	if tooltip != st.tooltip {
		st.tooltip = tooltip
		var tip objc.ID
		if tooltip != "" {
			tip = nsString(tooltip)
		}
		view.Send(sel("setToolTip:"), tip)
	}
}

// caretBlink is a caret blink phase, as macOS fields blink.
const caretBlink = 530 * time.Millisecond

// caretOn reports whether a caret that moved since ago shows: on for a
// phase, off for a phase, and so on.
func caretOn(since time.Duration) bool { return since < 0 || (since/caretBlink)%2 == 0 }

// blinkIn redraws the window after d, for the caret's next blink phase.
func (st *windowState) blinkIn(d time.Duration) {
	redraw := func() {
		post(func() {
			if current == st {
				st.window.Send(sel("contentView")).Send(sel("setNeedsDisplay:"), true)
			}
		})
	}
	if st.blink == nil {
		st.blink = time.AfterFunc(d, redraw)
		return
	}
	st.blink.Reset(d)
}

var current *windowState

// isDark reports whether view's effective appearance is a dark one.
func isDark(view objc.ID) bool {
	name := plainString(view.Send(sel("effectiveAppearance")).Send(sel("name")))
	return strings.Contains(name, "Dark")
}

func (st *windowState) setTitle(t string) {
	if t != "" && t != st.title {
		st.title = t
		st.window.Send(sel("setTitle:"), nsString(t))
	}
}

var (
	viewClassOnce sync.Once
	viewClass     objc.Class
	viewClassErr  error
)

// registerViewClass defines FlatteWindowView, the NSView subclass that
// draws scenes and turns AppKit events into Flatte events. It implements
// NSTextInputClient so input methods work.
func registerViewClass() (objc.Class, error) {
	viewClassOnce.Do(func() {
		viewClass, viewClassErr = objc.RegisterClass(
			"FlatteWindowView",
			objc.GetClass("NSView"),
			[]*objc.Protocol{objc.GetProtocol("NSTextInputClient")},
			nil,
			viewMethods(),
		)
	})
	return viewClass, viewClassErr
}

// location is the event's position in the view's (flipped) coordinates.
func location(self, event objc.ID) CGPoint {
	p := objc.Send[CGPoint](event, sel("locationInWindow"))
	return objc.Send[CGPoint](self, sel("convertPoint:fromView:"), p, objc.ID(0))
}

func eventMods(event objc.ID) flatte.Mod {
	return macMods(objc.Send[uint](event, sel("modifierFlags")))
}

func mouseHandler(button flatte.MouseButton, action flatte.MouseAction) func(objc.ID, objc.SEL, objc.ID) {
	return func(self objc.ID, _ objc.SEL, event objc.ID) {
		if st := current; st != nil {
			p := location(self, event)
			st.ui.mouse(p.X, p.Y, button, action, eventMods(event))
		}
	}
}

// commandKeys maps the editing commands input methods send through
// doCommandBySelector: to keys.
var commandKeys = map[string]flatte.KeyEvent{
	"insertNewline:":         {Key: flatte.KeyEnter},
	"insertTab:":             {Key: flatte.KeyTab},
	"insertBacktab:":         {Key: flatte.KeyTab, Mod: flatte.ModShift},
	"cancelOperation:":       {Key: flatte.KeyEscape},
	"deleteBackward:":        {Key: flatte.KeyBackspace},
	"deleteForward:":         {Key: flatte.KeyDelete},
	"moveUp:":                {Key: flatte.KeyUp},
	"moveDown:":              {Key: flatte.KeyDown},
	"moveLeft:":              {Key: flatte.KeyLeft},
	"moveRight:":             {Key: flatte.KeyRight},
	"moveWordLeft:":          {Key: flatte.KeyLeft, Mod: flatte.ModAlt},
	"moveWordRight:":         {Key: flatte.KeyRight, Mod: flatte.ModAlt},
	"moveToBeginningOfLine:": {Key: flatte.KeyHome},
	"moveToEndOfLine:":       {Key: flatte.KeyEnd},
	"scrollPageUp:":          {Key: flatte.KeyPageUp},
	"scrollPageDown:":        {Key: flatte.KeyPageDown},
}

func viewMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("isFlipped"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("viewDidChangeEffectiveAppearance"), Fn: func(self objc.ID, _ objc.SEL) {
			self.Send(sel("setNeedsDisplay:"), true)
		}},
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		{Cmd: sel("drawRect:"), Fn: func(self objc.ID, _ objc.SEL, _ CGRect) {
			st := current
			if st == nil {
				return
			}
			if st.follow {
				if dark := isDark(self); dark != st.dark {
					st.dark, st.th = dark, newTheme(themeFor(nil, dark))
				}
			}
			b := objc.Send[CGRect](self, sel("bounds"))
			st.ui.resize(b.Size.Width, b.Size.Height)
			scene := st.ui.render()
			ctx := uintptr(class("NSGraphicsContext").Send(sel("currentContext")).Send(sel("CGContext")))
			scale := objc.Send[float64](st.window, sel("backingScaleFactor"))
			p := &painter{ctx: ctx, m: st.m, th: st.th, marked: st.marked, scale: scale}
			p.hovered, p.pressed = st.ui.controlState()
			p.draw(scene, b.Size.Width, b.Size.Height)
			if c := st.ui.frameCursor(); c != nil {
				p.clip = layout.PointRect{W: b.Size.Width, H: b.Size.Height}
				p.caretAt(float64(c.X)*st.m.Column, float64(c.Y)*st.m.Line, st.m.Line, st.th.font(fontKey{mono: true}))
			}
			if p.caretOK {
				now := time.Now()
				if !st.caretOK || p.caret != st.caret {
					st.caretMoved = now // a moved caret shows at once
				}
				if caretOn(now.Sub(st.caretMoved)) {
					p.paintCaret()
				}
				st.blinkIn(caretBlink - now.Sub(st.caretMoved)%caretBlink)
			}
			st.caret, st.caretOK = p.caret, p.caretOK
			st.setTitle(st.ui.frameTitle())
		}},
		{Cmd: sel("keyDown:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			st := current
			if st == nil {
				return
			}
			chars := []rune(plainString(event.Send(sel("charactersIgnoringModifiers"))))
			flags := objc.Send[uint](event, sel("modifierFlags"))
			if st.marked == "" && len(chars) == 1 {
				if ev, ok := macKey(chars[0], flags); ok {
					if ev == (flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'v', Mod: flatte.ModSuper}) {
						st.ui.paste() // ⌘V pastes, as in a terminal
						return
					}
					st.ui.key(ev)
					return
				}
			}
			if flags&macCommand != 0 {
				return // the menu's; unclaimed chords are dropped
			}
			// Text, through the input context so input methods see it.
			self.Send(sel("interpretKeyEvents:"), class("NSArray").Send(sel("arrayWithObject:"), event))
		}},
		{Cmd: sel("mouseMoved:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			if st := current; st != nil {
				p := location(self, event)
				shape, tip := st.ui.hover(p.X, p.Y)
				st.showPointer(self, shape, tip)
			}
		}},
		{Cmd: sel("mouseExited:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if st := current; st != nil {
				shape, tip := st.ui.hover(-1, -1)
				st.showPointer(self, shape, tip)
			}
		}},
		{Cmd: sel("mouseDown:"), Fn: mouseHandler(flatte.MouseLeft, flatte.MousePress)},
		{Cmd: sel("mouseUp:"), Fn: mouseHandler(flatte.MouseLeft, flatte.MouseRelease)},
		{Cmd: sel("mouseDragged:"), Fn: mouseHandler(flatte.MouseLeft, flatte.MouseMotion)},
		{Cmd: sel("rightMouseDown:"), Fn: mouseHandler(flatte.MouseRight, flatte.MousePress)},
		{Cmd: sel("rightMouseUp:"), Fn: mouseHandler(flatte.MouseRight, flatte.MouseRelease)},
		{Cmd: sel("otherMouseDown:"), Fn: mouseHandler(flatte.MouseMiddle, flatte.MousePress)},
		{Cmd: sel("otherMouseUp:"), Fn: mouseHandler(flatte.MouseMiddle, flatte.MouseRelease)},
		{Cmd: sel("scrollWheel:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			st := current
			if st == nil {
				return
			}
			dy := objc.Send[float64](event, sel("scrollingDeltaY"))
			if !objc.Send[bool](event, sel("hasPreciseScrollingDeltas")) {
				dy *= st.m.Line // whole lines from a notched wheel
			}
			n := st.wheel.notches(dy, st.m.Line)
			button := flatte.MouseWheelUp
			if n < 0 {
				button, n = flatte.MouseWheelDown, -n
			}
			p := location(self, event)
			for range n {
				st.ui.mouse(p.X, p.Y, button, flatte.MousePress, eventMods(event))
			}
		}},

		// NSTextInputClient. The "document" is the composition alone:
		// committed text goes to the app as key events, so only the marked
		// text has ranges.
		{Cmd: sel("hasMarkedText"), Fn: func(objc.ID, objc.SEL) bool { return current != nil && current.marked != "" }},
		{Cmd: sel("markedRange"), Fn: func(objc.ID, objc.SEL) NSRange {
			if current == nil || current.marked == "" {
				return NSRange{Location: nsNotFound}
			}
			return NSRange{Length: uint(utf16Len(current.marked))}
		}},
		{Cmd: sel("selectedRange"), Fn: func(objc.ID, objc.SEL) NSRange {
			if current == nil {
				return NSRange{Location: nsNotFound}
			}
			return NSRange{Location: uint(utf16Len(current.marked))}
		}},
		{Cmd: sel("setMarkedText:selectedRange:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, _, _ NSRange) {
			if st := current; st != nil {
				st.marked = plainString(text)
				self.Send(sel("setNeedsDisplay:"), true)
			}
		}},
		{Cmd: sel("unmarkText"), Fn: func(self objc.ID, _ objc.SEL) {
			if st := current; st != nil && st.marked != "" {
				text := st.marked
				st.marked = ""
				for _, ev := range textKeys(text) {
					st.ui.key(ev)
				}
			}
		}},
		{Cmd: sel("validAttributesForMarkedText"), Fn: func(objc.ID, objc.SEL) objc.ID {
			return class("NSArray").Send(sel("array"))
		}},
		{Cmd: sel("attributedSubstringForProposedRange:actualRange:"), Fn: func(objc.ID, objc.SEL, NSRange, *NSRange) objc.ID {
			return 0
		}},
		{Cmd: sel("insertText:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, _ NSRange) {
			if st := current; st != nil {
				st.marked = ""
				for _, ev := range textKeys(plainString(text)) {
					st.ui.key(ev)
				}
				self.Send(sel("setNeedsDisplay:"), true)
			}
		}},
		{Cmd: sel("characterIndexForPoint:"), Fn: func(objc.ID, objc.SEL, CGPoint) uint { return nsNotFound }},
		{Cmd: sel("firstRectForCharacterRange:actualRange:"), Fn: func(self objc.ID, _ objc.SEL, r NSRange, actual *NSRange) CGRect {
			if actual != nil {
				*actual = r
			}
			st := current
			if st == nil || !st.caretOK {
				return CGRect{}
			}
			rect := objc.Send[CGRect](self, sel("convertRect:toView:"), st.caret, objc.ID(0))
			if win := self.Send(sel("window")); win != 0 {
				rect = objc.Send[CGRect](win, sel("convertRectToScreen:"), rect)
			}
			return rect
		}},
		{Cmd: sel("doCommandBySelector:"), Fn: func(self objc.ID, _ objc.SEL, cmd objc.SEL) {
			st := current
			if st == nil {
				return
			}
			for name, ev := range commandKeys {
				if sel(name) == cmd {
					st.ui.key(ev)
					return
				}
			}
		}},
	}
}
