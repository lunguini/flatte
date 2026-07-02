package flatui

import "testing"

func TestZoneScannerSetAndRect(t *testing.T) {
	z := NewZoneScanner()
	z.Set("btn", Rect{X: 6, Y: 0, W: 8, H: 1})

	rect, ok := z.Rect("btn")
	if !ok {
		t.Fatal("btn zone not found")
	}
	if rect.X != 6 || rect.Y != 0 || rect.W != 8 || rect.H != 1 {
		t.Fatalf("btn rect = %+v, want {6,0,8,1}", rect)
	}
}

func TestZoneScannerAtFindsContainingZone(t *testing.T) {
	z := NewZoneScanner()
	z.Set("left", Rect{X: 0, Y: 0, W: 1, H: 1})
	z.Set("right", Rect{X: 2, Y: 0, W: 1, H: 1})

	if id, _ := z.At(0, 0); id != "left" {
		t.Errorf("At(0,0) = %q, want left", id)
	}
	if id, _ := z.At(2, 0); id != "right" {
		t.Errorf("At(2,0) = %q, want right", id)
	}
	if _, ok := z.At(1, 0); ok {
		t.Error("At(1,0) should not be inside any zone")
	}
}

func TestZoneScannerAtReturnsLastInserted(t *testing.T) {
	z := NewZoneScanner()
	z.Set("back", Rect{X: 0, Y: 0, W: 10, H: 10})
	z.Set("front", Rect{X: 2, Y: 2, W: 4, H: 4})

	// Both contain (3,3); the later Set wins.
	if id, _ := z.At(3, 3); id != "front" {
		t.Errorf("At(3,3) = %q, want front (last inserted)", id)
	}
	// Only back contains (0,0).
	if id, _ := z.At(0, 0); id != "back" {
		t.Errorf("At(0,0) = %q, want back", id)
	}
}

func TestZoneScannerReSetKeepsOrder(t *testing.T) {
	z := NewZoneScanner()
	z.Set("a", Rect{X: 0, Y: 0, W: 10, H: 10})
	z.Set("b", Rect{X: 0, Y: 0, W: 10, H: 10})
	// Re-Set a: it must not jump ahead of b in hit priority.
	z.Set("a", Rect{X: 0, Y: 0, W: 10, H: 10})
	if id, _ := z.At(1, 1); id != "b" {
		t.Errorf("At(1,1) = %q, want b (re-Set must not change order)", id)
	}
}

func TestZoneScannerReset(t *testing.T) {
	z := NewZoneScanner()
	z.Set("a", Rect{X: 0, Y: 0, W: 4, H: 4})
	z.Reset()
	if _, ok := z.Rect("a"); ok {
		t.Fatal("Reset should clear all zones")
	}
	if _, ok := z.At(1, 1); ok {
		t.Fatal("Reset should leave no hittable zones")
	}
}

func TestZoneScannerIn(t *testing.T) {
	z := NewZoneScanner()
	z.Set("box", Rect{X: 10, Y: 3, W: 6, H: 2})
	if !z.In("box", 12, 4) {
		t.Error("In(box,12,4) should be true")
	}
	if z.In("box", 0, 0) {
		t.Error("In(box,0,0) should be false")
	}
	if z.In("missing", 12, 4) {
		t.Error("In on unknown id should be false")
	}
}
