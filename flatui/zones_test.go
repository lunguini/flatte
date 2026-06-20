package flatui

import "testing"

func TestZoneMapFindsNamedRect(t *testing.T) {
	var zones ZoneMap
	zones.Set("list", Rect{X: 2, Y: 4, Width: 20, Height: 8})

	if id, ok := zones.At(3, 5); !ok || id != "list" {
		t.Fatalf("At(3,5) = %q,%v want list,true", id, ok)
	}
	if !zones.In("list", 3, 5) {
		t.Fatal("In(list,3,5) = false, want true")
	}
	if zones.In("list", 30, 5) {
		t.Fatal("In(list,30,5) = true, want false")
	}
}

func TestZoneMapUsesLastSetZoneForOverlaps(t *testing.T) {
	var zones ZoneMap
	zones.Set("back", Rect{X: 0, Y: 0, Width: 10, Height: 10})
	zones.Set("front", Rect{X: 2, Y: 2, Width: 4, Height: 4})

	if id, ok := zones.At(3, 3); !ok || id != "front" {
		t.Fatalf("At(3,3) = %q,%v want front,true", id, ok)
	}
}

func TestZoneMapLocalCoordinates(t *testing.T) {
	var zones ZoneMap
	zones.Set("button", Rect{X: 10, Y: 3, Width: 6, Height: 2})

	x, y, ok := zones.Local("button", 12, 4)
	if !ok {
		t.Fatal("Local(button,12,4) ok = false, want true")
	}
	if x != 2 || y != 1 {
		t.Fatalf("Local(button,12,4) = %d,%d want 2,1", x, y)
	}
}

func TestZoneMapIgnoresEmptyRectsAndCanClear(t *testing.T) {
	var zones ZoneMap
	zones.Set("empty", Rect{X: 0, Y: 0})
	zones.Set("list", Rect{X: 0, Y: 0, Width: 1, Height: 1})

	if id, ok := zones.At(0, 0); !ok || id != "list" {
		t.Fatalf("At(0,0) = %q,%v want list,true", id, ok)
	}

	zones.Clear()
	if id, ok := zones.At(0, 0); ok || id != "" {
		t.Fatalf("At(0,0) after Clear = %q,%v want empty,false", id, ok)
	}
}
