package flat

import (
	"context"
	"testing"
	"time"
)

func newTestEffects(t *testing.T) (Effects[testState], chan StateUpdate[testState], context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan StateUpdate[testState], 16)
	return NewEffects(ctx, updates, nil), updates, cancel
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
