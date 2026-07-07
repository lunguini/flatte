//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

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
		state.activeTab = tab
	}
	render := func() {
		state.layout(browserColumns(surface), browserRows(surface))
		frame := View(state, flatte.RenderContext{Width: state.width})
		surface.Set("innerHTML", ansiToHTML(frame.Content))
		if frame.Title != "" {
			doc.Set("title", frame.Title)
		}
		syncHash(state)
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
		Handle(state, ev, flatte.Effects[State]{})
		render()
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
		Handle(state, flatte.MouseEvent{X: x, Y: y, Button: flatte.MouseLeft, Action: flatte.MousePress}, flatte.Effects[State]{})
		render()
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
		if tab, ok := tabFromHash(); ok && tab != state.activeTab {
			state.activeTab = tab
			state.searching = false
			render()
		}
		return nil
	})
	browserCallbacks = append(browserCallbacks, keydown, resize, click, wheel, hashchange)

	js.Global().Call("addEventListener", "keydown", keydown)
	js.Global().Call("addEventListener", "resize", resize)
	js.Global().Call("addEventListener", "hashchange", hashchange)
	surface.Call("addEventListener", "click", click)
	surface.Call("addEventListener", "wheel", wheel)
	render()
	surface.Call("focus")

	select {}
}

// tabFromHash maps the current URL fragment (e.g. "#components") to a tab so
// the page can deep-link into a view. Unknown fragments report false.
func tabFromHash() (landingTab, bool) {
	hash := strings.TrimPrefix(js.Global().Get("location").Get("hash").String(), "#")
	for i, tab := range landingTabs {
		if tab.ID == hash {
			return landingTab(i), true
		}
	}
	return 0, false
}

// syncHash mirrors the active tab back into the URL fragment via replaceState
// (which does not fire hashchange, so there is no feedback loop).
func syncHash(s *State) {
	if int(s.activeTab) >= len(landingTabs) {
		return
	}
	id := landingTabs[s.activeTab].ID
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
	charWidth = measureCharacterWidth(surface, style)
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

func measureCharacterWidth(surface, style js.Value) float64 {
	doc := js.Global().Get("document")
	probe := doc.Call("createElement", "span")
	probe.Get("style").Set("position", "absolute")
	probe.Get("style").Set("visibility", "hidden")
	probe.Get("style").Set("whiteSpace", "pre")
	probe.Get("style").Set("font", style.Call("getPropertyValue", "font").String())
	probe.Set("textContent", strings.Repeat("0", 64))
	doc.Get("body").Call("appendChild", probe)
	width := probe.Call("getBoundingClientRect").Get("width").Float() / 64
	doc.Get("body").Call("removeChild", probe)
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
