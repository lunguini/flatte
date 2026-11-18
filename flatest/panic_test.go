package flatest_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

type panicky struct {
	caught   []flatte.Panic
	viewBoom bool
	menuBoom bool
	folded   int
	text     string
}

func panickyApp(s *panicky) flatte.App[panicky] {
	return flatte.App[panicky]{
		State: s,
		Handle: func(s *panicky, ev flatte.Event, fx flatte.Effects[panicky]) {
			k, ok := ev.(flatte.KeyEvent)
			if !ok || k.Key != flatte.KeyCharacter {
				return
			}
			switch k.Rune {
			case 'h':
				var xs []int
				_ = xs[3] // index out of range
			case 'v':
				s.viewBoom = true
			case 'f':
				s.viewBoom = false
			case 'm':
				s.menuBoom = true
			case 'u': // the fold panics
				flatte.Go(fx, "load", func(context.Context) (int, error) { return 1, nil },
					func(s *panicky, _ int, _ error) { panic("fold broke") })
			case 'w': // the off-loop work panics
				flatte.Go(fx, "fetch", func(context.Context) (int, error) { panic(errors.New("work broke")) },
					func(s *panicky, _ int, _ error) { s.folded++ })
			default:
				s.text += string(k.Rune)
			}
		},
		View: func(s *panicky, _ flatte.RenderContext) flatte.Frame {
			if s.viewBoom {
				panic("view broke")
			}
			return flatte.Frame{Content: "text:" + s.text}
		},
		Menus: func(s *panicky) []flatte.Menu {
			if s.menuBoom {
				panic("menus broke")
			}
			return []flatte.Menu{{Title: "File"}}
		},
		Panic: func(s *panicky, p flatte.Panic, _ flatte.Effects[panicky]) { s.caught = append(s.caught, p) },
	}
}

func typeRune(d *flatest.Driver[panicky], r rune) flatte.Frame {
	return d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r})
}

func TestPanicsAreCaughtAndTheAppGoesOn(t *testing.T) {
	s := &panicky{}
	d := flatest.Start(panickyApp(s), 40)
	t.Cleanup(d.Close)

	typeRune(d, 'h')
	d.Settle()
	if len(s.caught) != 1 || s.caught[0].In != "Handle KeyEvent" || !strings.Contains(s.caught[0].Error(), "index out of range") {
		t.Fatalf("Handle panic: %+v", s.caught)
	}
	if w := s.caught[0].Where(); !strings.HasPrefix(w, "panic_test.go:") || !strings.Contains(w, "panickyApp") {
		t.Fatalf("Where %q, want the test's own line", w)
	}
	typeRune(d, 'a') // still running
	if !strings.Contains(flatest.CleanFrame(d.Frame().Content), "text:a") {
		t.Fatalf("after a caught panic: %q", d.Frame().Content)
	}

	// A broken View keeps the last frame and reports once.
	typeRune(d, 'v')
	d.Frame()
	d.Frame()
	d.Settle()
	if n := countIn(s.caught, "View"); n != 1 || !strings.Contains(flatest.CleanFrame(d.Frame().Content), "text:a") {
		t.Fatalf("View: %d reports, frame %q", n, d.Frame().Content)
	}
	typeRune(d, 'f') // fixed: draws again; a new break reports again
	typeRune(d, 'v')
	d.Settle()
	if n := countIn(s.caught, "View"); n != 2 {
		t.Fatalf("View after a clean draw broke again: %d reports", n)
	}
	typeRune(d, 'f')

	typeRune(d, 'm')
	typeRune(d, 'x') // menus are read for the key's shortcut, then again: one report
	typeRune(d, 'y')
	d.Settle()
	if n := countIn(s.caught, "Menus"); n != 1 {
		t.Fatalf("Menus: %d reports", n)
	}

	typeRune(d, 'u')
	d.Settle()
	if n := countIn(s.caught, "update load"); n != 1 {
		t.Fatalf("fold: %+v", s.caught)
	}
	typeRune(d, 'w')
	d.Settle()
	if n := countIn(s.caught, "work fetch"); n != 1 || s.folded != 0 {
		t.Fatalf("work: %d reports, folded %d", n, s.folded)
	}
}

func countIn(ps []flatte.Panic, in string) int {
	n := 0
	for _, p := range ps {
		if p.In == in {
			n++
		}
	}
	return n
}

func TestWithoutAPanicHookPanicsGoThrough(t *testing.T) {
	s := &panicky{}
	app := panickyApp(s)
	app.Panic = nil
	d := flatest.Start(app, 40)
	t.Cleanup(d.Close)
	defer func() {
		if recover() == nil {
			t.Fatal("the panic was swallowed without App.Panic")
		}
	}()
	typeRune(d, 'h')
}

func TestAPanickingHookIsDropped(t *testing.T) {
	s := &panicky{}
	app := panickyApp(s)
	app.Panic = func(*panicky, flatte.Panic, flatte.Effects[panicky]) { panic("hook broke") }
	d := flatest.Start(app, 40)
	t.Cleanup(d.Close)
	typeRune(d, 'h')
	d.Settle()
	typeRune(d, 'b')
	if !strings.Contains(flatest.CleanFrame(d.Frame().Content), "text:b") {
		t.Fatal("a panicking hook stopped the app")
	}
}
