//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/lunguini/flatte"
)

var browserCallbacks []js.Func

func main() {
	doc := js.Global().Get("document")
	surface := doc.Call("getElementById", "frame")
	if surface.IsNull() || surface.IsUndefined() {
		return
	}
	surface.Set("tabIndex", 0)

	state := NewState()
	if tab, ok := tabFromHash(); ok {
		state.phase = phaseShell
		state.active = tab
	}
	render := func() {
		state.layout(browserColumns(surface), browserRows(surface))
		frame := View(state, flatte.RenderContext{Width: state.width})
		surface.Set("innerHTML", ansiToHTML(frame.Content))
		if frame.Title != "" {
			doc.Set("title", frame.Title)
		}
	}
	// handleEvent applies an event, repaints, and mirrors the active tab into the
	// URL fragment only when the tab actually changed. The initial paint, resize,
	// and the animation tick never touch the URL, and merely dismissing the intro
	// (a phase change, not a tab change) leaves it alone — so loading the root
	// page keeps it at "/" instead of auto-appending "#game".
	handleEvent := func(ev flatte.Event) {
		before := state.active
		Handle(state, ev, flatte.Effects[State]{})
		render()
		if state.active != before {
			syncHash(state)
		}
	}

	keydown := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		ev, ok := browserKeyEvent(args[0])
		if !ok {
			return nil
		}
		args[0].Call("preventDefault")
		handleEvent(ev)
		return nil
	})
	resize := js.FuncOf(func(this js.Value, args []js.Value) any {
		Handle(state, flatte.ResizeEvent{Width: browserColumns(surface), Height: browserRows(surface)}, flatte.Effects[State]{})
		render()
		return nil
	})
	click := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		surface.Call("focus")
		x, y := browserCell(surface, args[0])
		handleEvent(flatte.MouseEvent{X: x, Y: y, Button: flatte.MouseLeft, Action: flatte.MousePress})
		return nil
	})
	wheel := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		args[0].Call("preventDefault")
		x, y := browserCell(surface, args[0])
		button := flatte.MouseWheelDown
		if args[0].Get("deltaY").Float() < 0 {
			button = flatte.MouseWheelUp
		}
		Handle(state, flatte.MouseEvent{X: x, Y: y, Button: button, Action: flatte.MousePress}, flatte.Effects[State]{})
		render()
		return nil
	})
	hashchange := js.FuncOf(func(this js.Value, args []js.Value) any {
		if tab, ok := tabFromHash(); ok && tab != state.active {
			state.phase = phaseShell
			state.active = tab
			render()
		}
		return nil
	})
	// Animation loop: the intro banner, the hosted game/app, and the UI-tab
	// preview widgets all advance one host tick per frame. Flatte's async engine
	// does not run under WASM, so this setInterval is the sole timing source.
	tick := js.FuncOf(func(this js.Value, args []js.Value) any {
		Tick(state, time.Now())
		render()
		return nil
	})
	browserCallbacks = append(browserCallbacks, keydown, resize, click, wheel, hashchange, tick)

	js.Global().Call("addEventListener", "keydown", keydown)
	js.Global().Call("addEventListener", "resize", resize)
	js.Global().Call("addEventListener", "hashchange", hashchange)
	surface.Call("addEventListener", "click", click)
	surface.Call("addEventListener", "wheel", wheel)
	js.Global().Call("setInterval", tick, 120)
	render()
	surface.Call("focus")

	select {}
}

// tabFromHash maps the current URL fragment (e.g. "#components") to a tab so
// the page can deep-link into a view. Unknown fragments report false.
func tabFromHash() (landingTab, bool) {
	hash := strings.TrimPrefix(js.Global().Get("location").Get("hash").String(), "#")
	for i, id := range tabIDs {
		if id == hash {
			return landingTab(i), true
		}
	}
	return 0, false
}

// syncHash mirrors the active tab back into the URL fragment via replaceState
// (which does not fire hashchange, so there is no feedback loop).
func syncHash(s *State) {
	if int(s.active) >= len(tabIDs) {
		return
	}
	id := tabIDs[s.active]
	loc := js.Global().Get("location")
	if strings.TrimPrefix(loc.Get("hash").String(), "#") == id {
		return
	}
	js.Global().Get("history").Call("replaceState", js.Null(), "", "#"+id)
}

func browserKeyEvent(event js.Value) (flatte.KeyEvent, bool) {
	key := event.Get("key").String()
	switch key {
	case "Tab":
		mod := flatte.Mod(0)
		if event.Get("shiftKey").Bool() {
			mod |= flatte.ModShift
		}
		return flatte.KeyEvent{Key: flatte.KeyTab, Mod: mod}, true
	case "ArrowDown":
		return flatte.KeyEvent{Key: flatte.KeyDown}, true
	case "ArrowUp":
		return flatte.KeyEvent{Key: flatte.KeyUp}, true
	case "ArrowLeft":
		return flatte.KeyEvent{Key: flatte.KeyLeft}, true
	case "ArrowRight":
		return flatte.KeyEvent{Key: flatte.KeyRight}, true
	case "Enter":
		return flatte.KeyEvent{Key: flatte.KeyEnter}, true
	case "Escape":
		return flatte.KeyEvent{Key: flatte.KeyEscape}, true
	case "Backspace":
		return flatte.KeyEvent{Key: flatte.KeyBackspace}, true
	case "Delete":
		return flatte.KeyEvent{Key: flatte.KeyDelete}, true
	case "Home":
		return flatte.KeyEvent{Key: flatte.KeyHome}, true
	case "End":
		return flatte.KeyEvent{Key: flatte.KeyEnd}, true
	case "PageUp":
		return flatte.KeyEvent{Key: flatte.KeyPageUp}, true
	case "PageDown":
		return flatte.KeyEvent{Key: flatte.KeyPageDown}, true
	}
	runes := []rune(key)
	if len(runes) == 1 {
		mod := flatte.Mod(0)
		if event.Get("altKey").Bool() {
			mod |= flatte.ModAlt
		}
		if event.Get("ctrlKey").Bool() {
			mod |= flatte.ModCtrl
		}
		if event.Get("shiftKey").Bool() {
			mod |= flatte.ModShift
		}
		return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: runes[0], Mod: mod}, true
	}
	return flatte.KeyEvent{}, false
}

func browserColumns(surface js.Value) int {
	contentWidth, lineHeight, charWidth := browserMetrics(surface)
	_ = lineHeight
	return clamp(int(contentWidth/charWidth), 48, 104)
}

func browserRows(surface js.Value) int {
	_, lineHeight, _ := browserMetrics(surface)
	height := float64(surface.Get("clientHeight").Int()) - verticalPadding(surface)
	return clamp(int(height/lineHeight), 16, 40)
}

func browserMetrics(surface js.Value) (contentWidth, lineHeight, charWidth float64) {
	style := js.Global().Call("getComputedStyle", surface)
	contentWidth = float64(surface.Get("clientWidth").Int()) - horizontalPadding(surface)
	if contentWidth < 1 {
		contentWidth = float64(js.Global().Get("innerWidth").Int())
	}
	fontSize, ok := cssPixels(style.Call("getPropertyValue", "font-size").String())
	if !ok || fontSize <= 0 {
		fontSize = 13
	}
	lineHeight, ok = cssPixels(style.Call("getPropertyValue", "line-height").String())
	if !ok || lineHeight <= 0 {
		lineHeight = fontSize * 1.18
	}
	charWidth = measureCharacterWidth(surface)
	if charWidth <= 0 {
		charWidth = fontSize * 0.62
	}
	return contentWidth, lineHeight, charWidth
}

func horizontalPadding(surface js.Value) float64 {
	style := js.Global().Call("getComputedStyle", surface)
	left, _ := cssPixels(style.Call("getPropertyValue", "padding-left").String())
	right, _ := cssPixels(style.Call("getPropertyValue", "padding-right").String())
	return left + right
}

func verticalPadding(surface js.Value) float64 {
	style := js.Global().Call("getComputedStyle", surface)
	top, _ := cssPixels(style.Call("getPropertyValue", "padding-top").String())
	bottom, _ := cssPixels(style.Call("getPropertyValue", "padding-bottom").String())
	return top + bottom
}

// measureCharacterWidth returns the pixel advance of one monospace cell by
// measuring a probe span appended *inside* the frame surface, so it inherits the
// exact font-family, font-size, and letter-spacing of the rendered text. An
// earlier version set the probe's `font` from getComputedStyle's shorthand
// (often returned empty) on a span in document.body, which fell back to a
// proportional font and skewed the horizontal cell mapping — clicks drifted on
// the column axis while rows (read from line-height directly) stayed correct.
func measureCharacterWidth(surface js.Value) float64 {
	const samples = 100
	doc := js.Global().Get("document")
	probe := doc.Call("createElement", "span")
	ps := probe.Get("style")
	ps.Set("position", "absolute")
	ps.Set("visibility", "hidden")
	ps.Set("whiteSpace", "pre")
	ps.Set("left", "-9999px")
	ps.Set("top", "0")
	ps.Set("padding", "0")
	ps.Set("border", "0")
	probe.Set("textContent", strings.Repeat("0", samples))
	surface.Call("appendChild", probe)
	width := probe.Call("getBoundingClientRect").Get("width").Float() / samples
	surface.Call("removeChild", probe)
	return width
}

func browserCell(surface, event js.Value) (int, int) {
	rect := surface.Call("getBoundingClientRect")
	contentWidth, lineHeight, charWidth := browserMetrics(surface)
	_ = contentWidth
	style := js.Global().Call("getComputedStyle", surface)
	padLeft, _ := cssPixels(style.Call("getPropertyValue", "padding-left").String())
	padTop, _ := cssPixels(style.Call("getPropertyValue", "padding-top").String())
	x := event.Get("clientX").Float() - rect.Get("left").Float() - padLeft + float64(surface.Get("scrollLeft").Int())
	y := event.Get("clientY").Float() - rect.Get("top").Float() - padTop + float64(surface.Get("scrollTop").Int())
	return clamp(int(x/charWidth), 0, browserColumns(surface)-1), clamp(int(y/lineHeight), 0, browserRows(surface)-1)
}

func clamp(n, low, high int) int {
	if n < low {
		return low
	}
	if n > high {
		return high
	}
	return n
}
