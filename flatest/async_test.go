package flatest

import (
	"context"
	"testing"
	"time"

	"github.com/lunguini/flatte"
)

type search struct {
	searching bool
	result    string
	applied   int // how many search results were actually folded
}

// searchApp starts a Latest("q") search on '?' that returns the fixed
// result. The view shows the in-flight state.
func searchApp(result string) flatte.App[search] {
	return flatte.App[search]{
		State: &search{},
		Handle: func(s *search, ev flatte.Event, fx flatte.Effects[search]) {
			k, ok := ev.(flatte.KeyEvent)
			if !ok || k.Rune != '?' {
				return
			}
			s.searching = true
			flatte.Latest(fx, "q",
				func(context.Context) (string, error) { return result, nil },
				func(s *search, v string, err error) {
					s.result = v
					s.searching = false
					s.applied++
				})
		},
		View: func(s *search, ctx flatte.RenderContext) flatte.Frame {
			if s.searching {
				return flatte.Frame{Content: "searching…"}
			}
			return flatte.Frame{Content: "result=" + s.result}
		},
	}
}

func TestSettleAppliesAsyncResult(t *testing.T) {
	d := Start(searchApp("opus"), 40)

	f := d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '?'})
	if !d.State().searching || f.Content != "searching…" {
		t.Fatalf("Send must show the in-flight state before Settle: searching=%v frame=%q",
			d.State().searching, f.Content)
	}

	f = d.Settle()
	if d.State().result != "opus" || d.State().searching {
		t.Fatalf("after Settle: result=%q searching=%v, want opus/false",
			d.State().result, d.State().searching)
	}
	if f.Content != "result=opus" {
		t.Fatalf("settled frame = %q, want result=opus", f.Content)
	}
}

func TestSettleDropsSupersededLatest(t *testing.T) {
	d := Start(searchApp("opus"), 40)

	// Two searches before settling: the second supersedes the first.
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '?'})
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '?'})
	d.Settle()

	// Both bodies run during Settle; gen-1's ctx was cancelled by gen-2's
	// Latest.replace, so its result is dropped at apply time. Exactly one
	// result lands.
	if d.State().applied != 1 {
		t.Fatalf("applied = %d, want 1 (superseded result dropped)", d.State().applied)
	}
}

type ticker struct{ n int }

func tickerApp() flatte.App[ticker] {
	return flatte.App[ticker]{
		State: &ticker{},
		Init: func(s *ticker, fx flatte.Effects[ticker]) {
			flatte.Every(fx, "t", 10*time.Millisecond, func(s *ticker, _ time.Time) { s.n++ })
		},
		View: func(s *ticker, ctx flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Content: "tick"}
		},
	}
}

func TestAdvanceFiresEveryTicks(t *testing.T) {
	d := Start(tickerApp(), 40)
	d.Advance(35 * time.Millisecond)
	if d.State().n != 3 {
		t.Fatalf("n = %d, want 3 ticks in 35ms@10ms", d.State().n)
	}
}
