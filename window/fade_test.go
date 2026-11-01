package window

import (
	"math"
	"testing"
	"time"
)

func TestFaderEasesInAndOut(t *testing.T) {
	t0 := time.Unix(0, 0)
	f := newFader(100 * time.Millisecond)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	f.set("row", true, at(0))
	if l := f.level("row", at(0)); l != 0 {
		t.Fatalf("starts at %v", l)
	}
	if l := f.level("row", at(50)); math.Abs(l-0.5) > 1e-9 {
		t.Fatalf("halfway %v", l)
	}
	if l := f.level("row", at(100)); l != 1 {
		t.Fatalf("done %v", l)
	}
	if _, moving := f.levels(at(150)); moving {
		t.Fatal("still moving when done")
	}
	// Off from halfway back in: it eases from where it was, not from 1.
	f.set("row", false, at(200))
	f.set("row", true, at(250)) // at 0.5
	if l := f.level("row", at(250)); math.Abs(l-0.5) > 1e-9 {
		t.Fatalf("reversed from %v, want 0.5", l)
	}
	if _, moving := f.levels(at(260)); !moving {
		t.Fatal("not moving mid-fade")
	}
	// Fully off is forgotten.
	f.set("row", false, at(400))
	f.level("row", at(600))
	if len(f.fades) != 0 {
		t.Fatalf("kept %v", f.fades)
	}
}

func TestFaderOnly(t *testing.T) {
	t0 := time.Unix(0, 0)
	f := newFader(0)
	f.only(map[string]bool{"a": true, "b": true}, t0)
	f.only(map[string]bool{"b": true}, t0)
	levels, _ := f.levels(t0)
	if levels["a"] != 0 || levels["b"] != 1 {
		t.Fatalf("levels %v", levels)
	}
}
