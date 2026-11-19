package flatte

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
)

// Panic is a panic the runtime caught for App.Panic: what was running, the
// value it panicked with, and the stack where it did.
type Panic struct {
	// In names what was running: "Init", "Handle KeyEvent", "View",
	// "Menus", "update <name>", "work <name>" (off-loop Go, Latest or
	// Stream work), and the like.
	In    string
	Value any
	Stack []byte
}

func (p Panic) Error() string { return fmt.Sprintf("panic in %s: %v", p.In, p.Value) }

// Where is the first frame of the stack in the app's own code — not the
// runtime's, the standard library's, or Flatte's — as "file.go:42
// pkg.Func", or "" when none is found: a short pointer to the cause.
func (p Panic) Where() string {
	lines := strings.Split(string(p.Stack), "\n")
	for i := 0; i+1 < len(lines); i++ {
		fn := strings.TrimSpace(lines[i])
		loc := strings.TrimSpace(lines[i+1])
		if !strings.HasPrefix(lines[i+1], "\t") || strings.HasPrefix(fn, "panic(") {
			continue
		}
		name := fn
		if j := strings.LastIndexByte(name, '('); j > 0 {
			name = name[:j]
		}
		if !appFrame(name) {
			continue
		}
		file := loc
		if j := strings.LastIndexByte(file, ' '); j > 0 {
			file = file[:j] // drop " +0x1c"
		}
		if j := strings.LastIndexByte(file, '/'); j >= 0 {
			file = file[j+1:]
		}
		if j := strings.LastIndexByte(name, '/'); j >= 0 {
			name = name[j+1:]
		}
		return file + " " + name
	}
	return ""
}

// appFrame reports a stack function that is the app's: not the standard
// library's (no domain in its first path element; only package main has
// none) and not Flatte's runtime.
func appFrame(name string) bool {
	for _, own := range []string{"github.com/lunguini/flatte.", "github.com/lunguini/flatte/window.", "github.com/lunguini/flatte/flatest.", "github.com/lunguini/flatte/internal/"} {
		if strings.HasPrefix(name, own) {
			return false
		}
	}
	slash := strings.IndexByte(name, '/')
	if slash < 0 {
		return strings.HasPrefix(name, "main.")
	}
	return strings.Contains(name[:slash], ".")
}

// catcher recovers panics in app code for App.Panic and hands them to
// report; with no App.Panic it lets them through.
type catcher struct {
	report func(Panic)
	broken map[string]bool // reads (View, Menus…) that panicked and have not run cleanly since
}

// read is run for what the runtime reads after every change (View,
// Menus…): a panic is reported once, until f returns normally again, so a
// broken View does not report every frame. Loop goroutine only.
func (c *catcher) read(in string, f func()) (ok bool) {
	if c == nil || c.report == nil {
		f()
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			ok = false
			if !c.broken[in] {
				if c.broken == nil {
					c.broken = map[string]bool{}
				}
				c.broken[in] = true
				c.report(Panic{In: in, Value: r, Stack: debug.Stack()})
			}
		}
	}()
	f()
	delete(c.broken, in)
	return true
}

// run calls f, catching a panic in it as in; it reports whether f
// returned normally.
func (c *catcher) run(in string, f func()) (ok bool) {
	if c == nil || c.report == nil {
		f()
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			ok = false
			c.report(Panic{In: in, Value: r, Stack: debug.Stack()})
		}
	}()
	f()
	return true
}

// guardPanicHook calls the app's Panic hook, dropping (and printing) a
// panic in the hook itself so a broken hook cannot loop.
func guardPanicHook(p Panic, hook func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "flatte: App.Panic panicked (%v) handling %v\n", r, p)
		}
	}()
	hook()
}

// eventName is an event's type for Panic.In: "KeyEvent".
func eventName(ev Event) string {
	n := fmt.Sprintf("%T", ev)
	if i := strings.LastIndexByte(n, '.'); i >= 0 {
		n = n[i+1:]
	}
	return n
}

// guardApp is app with its code run under catch. What the runtime reads
// after every change (View per window, Menus, Windows, NowPlaying) keeps
// its last good value when it panics, reported once until it runs
// cleanly again; a context menu that panics is none.
func guardApp[S any](app App[S], c *catcher) App[S] {
	if init := app.Init; init != nil {
		app.Init = func(s *S, fx Effects[S]) { c.run("Init", func() { init(s, fx) }) }
	}
	if handle := app.Handle; handle != nil {
		app.Handle = func(s *S, ev Event, fx Effects[S]) {
			c.run("Handle "+eventName(ev), func() { handle(s, ev, fx) })
		}
	}
	view := app.View
	frames := map[string]Frame{}
	app.View = func(s *S, ctx RenderContext) Frame {
		in := "View"
		if ctx.Window != "" {
			in += " " + ctx.Window
		}
		var f Frame
		if c.read(in, func() { f = view(s, ctx) }) {
			frames[ctx.Window] = f
		}
		return frames[ctx.Window]
	}
	if menus := app.Menus; menus != nil {
		var last []Menu
		app.Menus = func(s *S) []Menu {
			var m []Menu
			if c.read("Menus", func() { m = menus(s) }) {
				last = m
			}
			return last
		}
	}
	if context := app.ContextMenu; context != nil {
		app.ContextMenu = func(s *S, target string) []MenuItem {
			var m []MenuItem
			c.read("ContextMenu", func() { m = context(s, target) })
			return m
		}
	}
	if windows := app.Windows; windows != nil {
		var last []Window
		app.Windows = func(s *S) []Window {
			var w []Window
			if c.read("Windows", func() { w = windows(s) }) {
				last = w // a panic keeps the windows open
			}
			return last
		}
	}
	if playing := app.NowPlaying; playing != nil {
		var last *NowPlaying
		app.NowPlaying = func(s *S) *NowPlaying {
			var n *NowPlaying
			if c.read("NowPlaying", func() { n = playing(s) }) {
				last = n
			}
			return last
		}
	}
	return app
}
