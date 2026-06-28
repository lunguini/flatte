package layout

import (
	"strings"
	"testing"
)

func TestOverlayCentersLayerOverBase(t *testing.T) {
	base := strings.Join([]string{
		"..........",
		"..........",
		"..........",
	}, "\n")
	layer := "XX\nXX"
	got := Overlay(base, layer)
	lines := strings.Split(got, "\n")
	// 10x3 base, 2x2 layer → origin x=(10-2)/2=4, y=(3-2)/2=0
	if !strings.HasPrefix(lines[0], "....XX") {
		t.Fatalf("row0 = %q, want layer at col 4", lines[0])
	}
	if !strings.HasPrefix(lines[1], "....XX") {
		t.Fatalf("row1 = %q, want layer at col 4", lines[1])
	}
}

func TestOverlayOriginMatchesOverlay(t *testing.T) {
	base := "..........\n..........\n.........."
	layer := "XX\nXX"
	x, y := OverlayOrigin(base, layer)
	if x != 4 || y != 0 {
		t.Fatalf("origin = (%d,%d), want (4,0)", x, y)
	}
}
