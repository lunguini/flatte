package flatte

import (
	"context"
	"testing"
)

func TestDispatchSeamRunsWorkThroughInjectedRunner(t *testing.T) {
	updates := make(chan StateUpdate[testState], 1)
	var ran []func()
	fx := NewHarnessEffects[testState](
		context.Background(), updates, func() {},
		func(f func()) { ran = append(ran, f) }, // capture, don't run
		nil,                                     // real clock unused here
	)

	Go(fx, "load", func(context.Context) (int, error) { return 7, nil },
		func(s *testState, v int, err error) { s.count = v })

	if len(ran) != 1 {
		t.Fatalf("dispatch captured %d funcs, want 1 (Go must spawn via dispatch)", len(ran))
	}
	if len(updates) != 0 {
		t.Fatalf("update queued before the dispatched body ran")
	}
	ran[0]() // run the captured body now
	select {
	case u := <-updates:
		var st testState
		u.Apply(&st)
		if st.count != 7 {
			t.Fatalf("folded count = %d, want 7", st.count)
		}
	default:
		t.Fatal("running the dispatched body produced no update")
	}
}
