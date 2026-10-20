package window

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// ui is what the view needs from the generic host.
type ui interface {
	resize(width, height float64)
	key(flatte.KeyEvent)
	mouse(x, y float64, button flatte.MouseButton, action flatte.MouseAction, mod flatte.Mod)
	render() layout.Scene
	frameTitle() string
	frameCursor() *flatte.Cursor
}

// windowState is the Go side of the one window a process shows. AppKit calls
// the view's methods with self; there is a single view, so the state is a
// package value, touched only on the main thread.
type windowState struct {
	ui     ui
	m      layout.Metrics
	th     theme
	window objc.ID
	title  string
	// marked is the input method's uncommitted text; caret is where the
	// focused input's caret was last drawn, in view coordinates.
	marked  string
	caret   CGRect
	caretOK bool
	wheel   wheelAccumulator
}

var current *windowState

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
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		{Cmd: sel("drawRect:"), Fn: func(self objc.ID, _ objc.SEL, _ CGRect) {
			st := current
			if st == nil {
				return
			}
			b := objc.Send[CGRect](self, sel("bounds"))
			st.ui.resize(b.Size.Width, b.Size.Height)
			scene := st.ui.render()
			ctx := uintptr(class("NSGraphicsContext").Send(sel("currentContext")).Send(sel("CGContext")))
			p := &painter{ctx: ctx, m: st.m, th: st.th, marked: st.marked}
			p.draw(scene, b.Size.Width, b.Size.Height)
			if c := st.ui.frameCursor(); c != nil {
				p.caretAt(float64(c.X)*st.m.Column, float64(c.Y)*st.m.Line, fontFor(fontKey{mono: true}))
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
