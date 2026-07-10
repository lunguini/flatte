//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/lunguini/flatte/cmd/internal/snakeapp"
)

// leaderboard is the browser-only high-score overlay. It lives entirely in the
// DOM — the shared Flatte core (State/View) never learns about the network or
// the board — which is why it is a plain struct of element handles driven off
// each rendered frame, not app state. When the hosted Snake game ends on the
// Game tab it pops a submit dialog; a badge button reopens the board any time.
//
// The Worker endpoint is read from window.FLATTE_LEADERBOARD_URL (set in
// index.html). If it is empty the whole feature stays dormant: no badge, no
// prompt, no network — the demo still plays, just without a shared board.
type leaderboard struct {
	url string
	get *State // landing state, for reading the hosted game on submit

	openBtn   js.Value // "🏆 Leaderboard" badge, shown on the Game tab
	modal     js.Value // dialog container (hidden attr toggles visibility)
	submitBox js.Value // score + name form, shown only after a game ends
	scoreEl   js.Value
	nameEl    js.Value
	sendBtn   js.Value
	statusEl  js.Value
	listEl    js.Value

	prevOver  bool // last-seen game-over state, to fire the prompt on the edge
	sending   bool // a submit is in flight (guards the throttle / double-send)
	callbacks []js.Func
}

func newLeaderboard(state *State) *leaderboard {
	doc := js.Global().Get("document")
	byID := func(id string) js.Value { return doc.Call("getElementById", id) }

	url := ""
	if v := js.Global().Get("FLATTE_LEADERBOARD_URL"); v.Type() == js.TypeString {
		url = v.String()
	}

	lb := &leaderboard{
		url:       url,
		get:       state,
		openBtn:   byID("lb-open"),
		modal:     byID("lb"),
		submitBox: byID("lb-submit"),
		scoreEl:   byID("lb-score"),
		nameEl:    byID("lb-name"),
		sendBtn:   byID("lb-send"),
		statusEl:  byID("lb-status"),
		listEl:    byID("lb-list"),
	}
	if url == "" || lb.modal.IsNull() {
		return lb // dormant: elements missing or unconfigured
	}

	lb.on(lb.openBtn, "click", func(js.Value) { lb.openBoard() })
	lb.on(byID("lb-close"), "click", func(js.Value) { lb.hide() })
	lb.on(lb.sendBtn, "click", func(js.Value) { lb.submit() })
	// Enter submits, Escape closes — handled on the input so they work while the
	// global key handler is suppressed (see blocking()).
	lb.on(lb.nameEl, "keydown", func(ev js.Value) {
		switch ev.Get("key").String() {
		case "Enter":
			ev.Call("preventDefault")
			lb.submit()
		case "Escape":
			ev.Call("preventDefault")
			lb.hide()
		}
	})
	return lb
}

// on wires a DOM event listener and retains the js.Func for the page lifetime.
func (lb *leaderboard) on(el js.Value, event string, fn func(js.Value)) {
	if el.IsNull() || el.IsUndefined() {
		return
	}
	cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var ev js.Value
		if len(args) > 0 {
			ev = args[0]
		}
		fn(ev)
		return nil
	})
	lb.callbacks = append(lb.callbacks, cb)
	el.Call("addEventListener", event, cb)
}

// blocking reports whether the modal is open, so the main input handlers skip
// forwarding keys/clicks to the hosted game while the dialog has focus.
func (lb *leaderboard) blocking() bool {
	return !lb.modal.IsNull() && !lb.modal.Get("hidden").Bool()
}

// afterFrame runs once per rendered frame. It keeps the badge visibility in sync
// with the active tab and fires the submit prompt on the transition into
// game-over while the Game tab is showing.
func (lb *leaderboard) afterFrame(s *State) {
	if lb.url == "" || lb.modal.IsNull() {
		return
	}
	onGame := s.active == tabGame
	showBadge := onGame && !lb.blocking()
	lb.openBtn.Set("hidden", !showBadge)

	over := onGame && snakeapp.Over(s.game)
	if over && !lb.prevOver {
		lb.openSubmit()
	}
	lb.prevOver = over
}

// openBoard shows the dialog in read-only mode (no form) and fetches the board.
func (lb *leaderboard) openBoard() {
	lb.submitBox.Set("hidden", true)
	lb.show()
	lb.fetchBoard()
}

// openSubmit shows the dialog with the score and name form, then fetches the
// current standings underneath.
func (lb *leaderboard) openSubmit() {
	lb.scoreEl.Set("textContent", fmt.Sprintf("Game over — you scored %d.", snakeapp.Score(lb.get.game)))
	lb.setStatus("")
	lb.sendBtn.Set("disabled", false)
	lb.submitBox.Set("hidden", false)
	lb.show()
	lb.nameEl.Call("focus")
	lb.fetchBoard()
}

func (lb *leaderboard) show() { lb.modal.Set("hidden", false) }
func (lb *leaderboard) hide() { lb.modal.Set("hidden", true) }
func (lb *leaderboard) setStatus(s string) {
	lb.statusEl.Set("textContent", s)
}

// submit POSTs the finished game's (seed, inputs) plus the entered name. The
// server verifies the run and returns the authoritative score, rank, and board.
func (lb *leaderboard) submit() {
	if lb.sending || lb.url == "" {
		return
	}
	seed, inputs := snakeapp.Submission(lb.get.game)
	body, err := json.Marshal(struct {
		Name   string   `json:"name"`
		Seed   string   `json:"seed"`
		Inputs [][2]int `json:"inputs"`
	}{
		Name:   lb.nameEl.Get("value").String(),
		Seed:   seed,
		Inputs: inputs,
	})
	if err != nil {
		lb.setStatus("could not encode your run")
		return
	}

	lb.sending = true
	lb.sendBtn.Set("disabled", true)
	lb.setStatus("submitting…")

	opts := js.ValueOf(map[string]any{
		"method":  "POST",
		"headers": map[string]any{"Content-Type": "application/json"},
		"body":    string(body),
	})
	promise := js.Global().Call("fetch", lb.url+"/api/score", opts)
	lb.onResponse(promise, func(data js.Value) {
		lb.sending = false
		if data.IsNull() {
			lb.sendBtn.Set("disabled", false)
			return
		}
		rank := data.Get("rank")
		if rank.Type() == js.TypeNumber {
			lb.setStatus(fmt.Sprintf("submitted — you're #%d", rank.Int()))
		} else {
			lb.setStatus("submitted")
		}
		lb.renderBoard(data.Get("board"))
	}, func(status int) {
		lb.sending = false
		lb.sendBtn.Set("disabled", false)
		if status == 429 {
			lb.setStatus("slow down — try again in a moment")
		} else {
			lb.setStatus("submit failed")
		}
	})
}

// fetchBoard loads and renders the current top-N standings.
func (lb *leaderboard) fetchBoard() {
	if lb.url == "" {
		return
	}
	lb.listEl.Set("innerHTML", `<li class="lb-empty">loading…</li>`)
	promise := js.Global().Call("fetch", lb.url+"/api/leaderboard")
	lb.onResponse(promise, func(data js.Value) {
		if !data.IsNull() {
			lb.renderBoard(data.Get("board"))
		}
	}, func(int) {
		lb.listEl.Set("innerHTML", `<li class="lb-empty">leaderboard unavailable</li>`)
	})
}

// renderBoard paints a board array ([{name, score}]) into the ordered list.
func (lb *leaderboard) renderBoard(board js.Value) {
	if board.IsNull() || board.IsUndefined() || board.Length() == 0 {
		lb.listEl.Set("innerHTML", `<li class="lb-empty">no scores yet — be the first</li>`)
		return
	}
	var b []byte
	for i := 0; i < board.Length(); i++ {
		e := board.Index(i)
		name := htmlEscape(e.Get("name").String())
		score := e.Get("score").Int()
		b = append(b, fmt.Sprintf(
			`<li><span class="lb-rank">%d</span><span class="lb-who">%s</span><span class="lb-pts">%d</span></li>`,
			i+1, name, score)...)
	}
	lb.listEl.Set("innerHTML", string(b))
}

// onResponse chains a fetch promise: ok → parse JSON and call ok(data);
// non-2xx → fail(status); network error → fail(0). Both branches release their
// one-shot callbacks.
func (lb *leaderboard) onResponse(promise js.Value, ok func(js.Value), fail func(int)) {
	then(promise, func(resp js.Value) {
		if resp.IsNull() || !resp.Get("ok").Bool() {
			status := 0
			if !resp.IsNull() {
				status = resp.Get("status").Int()
			}
			fail(status)
			return
		}
		then(resp.Call("json"), func(data js.Value) { ok(data) })
	})
	catch(promise, func() { fail(0) })
}

// then attaches a self-releasing one-shot resolve handler and returns the
// chained promise.
func then(promise js.Value, fn func(js.Value)) js.Value {
	var cb js.Func
	cb = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer cb.Release()
		var v js.Value
		if len(args) > 0 {
			v = args[0]
		}
		fn(v)
		return nil
	})
	return promise.Call("then", cb)
}

// catch attaches a self-releasing one-shot rejection handler.
func catch(promise js.Value, fn func()) js.Value {
	var cb js.Func
	cb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		defer cb.Release()
		fn()
		return nil
	})
	return promise.Call("catch", cb)
}

// htmlEscape makes a user name safe to drop into innerHTML. The server already
// strips control/zero-width characters and caps length; this guards the markup.
func htmlEscape(s string) string {
	repl := map[rune]string{'&': "&amp;", '<': "&lt;", '>': "&gt;", '"': "&quot;", '\'': "&#39;"}
	var b []byte
	for _, r := range s {
		if esc, ok := repl[r]; ok {
			b = append(b, esc...)
		} else {
			b = append(b, string(r)...)
		}
	}
	return string(b)
}
