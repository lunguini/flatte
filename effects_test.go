package flatte

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func newTestEffects(t *testing.T) (Effects[testState], chan StateUpdate[testState], context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan StateUpdate[testState], 16)
	return newEffects(ctx, updates, nil), updates, cancel
}

func TestEffectsSendQueuesUpdate(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	if !fx.Send(Named("inc", func(s *testState) { s.count++ })) {
		t.Fatal("Send returned false for a live Effects value")
	}
	state := testState{}
	(<-updates).Apply(&state)
	if state.count != 1 {
		t.Fatalf("count = %d, want 1", state.count)
	}
}

func TestEffectsSendIsSafeOnZeroAndNilUpdate(t *testing.T) {
	var fx Effects[testState]
	if fx.Send(nil) {
		t.Fatal("Send(nil) on zero Effects returned true")
	}
	if fx.Send(Named("noop", func(*testState) {})) {
		t.Fatal("Send on zero Effects returned true")
	}
}

func TestZeroEffectsDoesNotStartAsyncWork(t *testing.T) {
	var fx Effects[int]
	clock := &countingClock{}
	fx.clock = clock
	var started atomic.Int32
	Go(fx, "go", func(context.Context) (int, error) {
		started.Add(1)
		return 0, nil
	}, func(*int, int, error) {})
	Latest(fx, "latest", func(context.Context) (int, error) {
		started.Add(1)
		return 0, nil
	}, func(*int, int, error) {})
	Stream(fx, "stream", func(context.Context, func(int)) {
		started.Add(1)
	}, func(*int, int) {})
	if err := Every(fx, "every", time.Second, func(*int, time.Time) { started.Add(1) }); err != nil {
		t.Fatal(err)
	}
	scope := NewScope(fx, "scope")
	ScopeGo(scope, fx, func(context.Context) (int, error) {
		started.Add(1)
		return 0, nil
	}, func(*int, int, error) {})
	ScopeStream(scope, fx, func(context.Context, func(int)) { started.Add(1) }, func(*int, int) {})
	if err := ScopeEvery(scope, fx, time.Second, func(*int, time.Time) { started.Add(1) }); err != nil {
		t.Fatal(err)
	}
	if started.Load() != 0 || clock.ticks != 0 {
		t.Fatalf("zero Effects started %d workers and registered %d ticks", started.Load(), clock.ticks)
	}
	if err := Every(fx, "invalid", 0, func(*int, time.Time) {}); err != ErrInvalidInterval {
		t.Fatalf("Every with invalid interval returned %v", err)
	}
}

type countingClock struct{ ticks int }

func (c *countingClock) Tick(context.Context, time.Duration, func(time.Time)) { c.ticks++ }

func TestExecWithoutATerminalReportsUnsupported(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	fx := newHarnessEffectsForTest[testState](context.Background(), updates, nil, nil, nil)
	var got error
	Exec(fx, "edit", nil, func(_ *testState, err error) { got = err })
	select {
	case u := <-updates:
		if u.Name() != "edit" {
			t.Fatalf("update name = %q, want edit", u.Name())
		}
		u.Apply(&testState{})
	default:
		t.Fatal("Exec sent no update on a host that cannot exec")
	}
	if got != ErrUnsupported {
		t.Fatalf("Exec error = %v, want ErrUnsupported", got)
	}
}

func TestHostsReportTheirCapabilities(t *testing.T) {
	var zero Effects[testState]
	if zero.Capabilities() != (Capabilities{}) {
		t.Fatal("a zero Effects value reports capabilities")
	}
	term := terminalCapabilities(runConfig{})
	if !term.Exec || !term.Alert || !term.Quit || term.Mouse || term.Drop {
		t.Fatalf("terminal without WithMouse = %+v", term)
	}
	if !terminalCapabilities(runConfig{mouse: MouseModeCellMotion}).Mouse {
		t.Fatal("terminal with WithMouse reports no mouse")
	}
	web := browserCapabilities(runConfig{})
	if web.Exec || web.OpenFile || web.SaveFile || web.Quit || !web.Alert || web.Mouse {
		t.Fatalf("browser without WithMouse = %+v", web)
	}
	fx := withHostCapabilities(zero, Capabilities{Drop: true})
	if !fx.Capabilities().Drop {
		t.Fatal("WithHostCapabilities was not reported")
	}
}

func TestEffectsSendRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan StateUpdate[testState], 1)
	fx := newEffects(ctx, updates, nil)
	cancel()

	if fx.Send(Named("noop", func(*testState) {})) {
		t.Fatal("Send returned true after context cancellation")
	}
	select {
	case <-updates:
		t.Fatal("Send queued an update after context cancellation")
	default:
	}
}

func TestGoSendsNamedFoldedUpdate(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	Go(fx, "counter.load",
		func(context.Context) (int, error) { return 5, nil },
		func(s *testState, value int, err error) { s.count = value },
	)

	update := receiveCoreUpdate(t, updates)
	if update.Name() != "counter.load" {
		t.Fatalf("Name() = %q, want counter.load", update.Name())
	}
	state := testState{}
	update.Apply(&state)
	if state.count != 5 {
		t.Fatalf("count = %d, want 5", state.count)
	}
}

func TestEverySendsRepeatedUpdatesUntilCancelled(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)

	Every(fx, "tick", time.Millisecond, func(s *testState, _ time.Time) { s.count++ })

	state := testState{}
	for range 3 {
		receiveCoreUpdate(t, updates).Apply(&state)
	}
	cancel()
	if state.count != 3 {
		t.Fatalf("count = %d, want 3", state.count)
	}
}

func TestStreamForwardsSourceValuesInOrder(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	Stream(fx, "feed",
		func(ctx context.Context, send func(int)) {
			send(1)
			send(2)
		},
		func(s *testState, v int) { s.count += v },
	)

	state := testState{}
	receiveCoreUpdate(t, updates).Apply(&state)
	receiveCoreUpdate(t, updates).Apply(&state)
	if state.count != 3 {
		t.Fatalf("count = %d, want 1+2=3", state.count)
	}
}

func TestLatestSupersedesPriorWorkOfSameName(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	fold := func(s *testState, v int, err error) {
		if err != nil {
			return
		}
		s.count = v
	}

	// First call completes immediately; hold its queued update unapplied.
	Latest(fx, "search",
		func(ctx context.Context) (int, error) { return 1, nil },
		fold,
	)
	staleUpdate := receiveCoreUpdate(t, updates)

	// Second call supersedes the first: replace cancels the first call's
	// context synchronously, so the already-queued first update is stale.
	Latest(fx, "search",
		func(ctx context.Context) (int, error) { return 2, nil },
		fold,
	)

	state := testState{}
	receiveCoreUpdate(t, updates).Apply(&state)
	if state.count != 2 {
		t.Fatalf("count = %d, want latest result 2", state.count)
	}

	// Applying the queued-then-superseded update must be a no-op: the
	// apply-time ctx.Err() guard drops it.
	staleUpdate.Apply(&state)
	if state.count != 2 {
		t.Fatalf("count = %d after stale apply, want 2 untouched", state.count)
	}
}

func TestCancelStopsInFlightLatestWork(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	started := make(chan struct{})
	Latest(fx, "search",
		func(ctx context.Context) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		},
		func(s *testState, v int, err error) {
			if err == nil {
				s.count = v
			}
		},
	)
	<-started

	Cancel(fx, "search")

	select {
	case update := <-updates:
		state := testState{count: -1}
		update.Apply(&state)
		if state.count != -1 {
			t.Fatalf("cancelled update mutated state: count = %d", state.count)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLatestReleasesRegistryEntryAfterApply(t *testing.T) {
	fx, updates, cancel := newTestEffects(t)
	defer cancel()

	Latest(fx, "search",
		func(ctx context.Context) (int, error) { return 1, nil },
		func(s *testState, v int, err error) {},
	)
	update := receiveCoreUpdate(t, updates)

	// The entry must survive until the update is applied, so a newer Latest
	// call can still supersede the queued result.
	if !fx.latest.contains("search") {
		t.Fatal("registry entry released before its update was applied")
	}

	state := testState{}
	update.Apply(&state)

	// release runs synchronously inside Apply.
	if fx.latest.contains("search") {
		t.Fatal("registry still holds entry after the update was applied")
	}
}

func TestReleaseIgnoresSupersededGeneration(t *testing.T) {
	registry := newLatestRegistry()

	_, first := registry.replace("search", context.Background())
	_, second := registry.replace("search", context.Background())

	registry.release("search", first)
	if !registry.contains("search") {
		t.Fatal("older generation's release removed the newer entry")
	}

	registry.release("search", second)
	if registry.contains("search") {
		t.Fatal("owning generation's release did not remove its entry")
	}
}

// contains reports whether the registry holds an in-flight entry for name.
// Test-only observability helper.
func (r *latestRegistry) contains(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.entries[name]
	return ok
}

func TestWithHostClipboardRoutesClipboard(t *testing.T) {
	var set []string
	reads := 0
	fx := withHostClipboard(newHarnessEffectsForTest[struct{}](context.Background(), nil, nil, nil, nil),
		func(s string) { set = append(set, s) }, func() { reads++ })
	fx.SetClipboard("copied")
	fx.ReadClipboard()
	if len(set) != 1 || set[0] != "copied" || reads != 1 {
		t.Fatalf("set %q, reads %d", set, reads)
	}
	// Without a host clipboard, harness effects stay no-ops.
	newHarnessEffectsForTest[struct{}](context.Background(), nil, nil, nil, nil).SetClipboard("x")
}
