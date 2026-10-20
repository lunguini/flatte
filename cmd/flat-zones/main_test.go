package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// ready is a state that has seen its first frame at 72×24: the runtime's
// LayoutEvent has fed the panel rects into the zone map.
func ready() *State {
	s := NewState()
	settle(s, 72, 24)
	return s
}

// settle composes the view at width×height and delivers the solved rects as
// the runtime does before painting.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

// composed is View's frame as the runtime paints it at 72×24.
func composed(f flatte.Frame) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, 72, 24)
	return f
}

func TestClickSelectsPanelThroughZoneMap(t *testing.T) {
	s := ready()
	rect, ok := s.zones.Rect(logsZone)
	if !ok {
		t.Fatal("logs zone missing")
	}

	Handle(s, flatte.MouseEvent{
		X: rect.X + 2, Y: rect.Y + 1, Button: flatte.MouseLeft, Action: flatte.MousePress,
	}, flatte.Effects[State]{})

	if s.selected != logsZone {
		t.Fatalf("selected = %q, want %q", s.selected, logsZone)
	}
	if s.last != "logs local 2,1" {
		t.Fatalf("last = %q, want logs local 2,1", s.last)
	}
}

func TestClickOutsidePanelDoesNotChangeSelection(t *testing.T) {
	s := ready()
	s.selected = metricsZone

	Handle(s, flatte.MouseEvent{
		X: 0, Y: 0, Button: flatte.MouseLeft, Action: flatte.MousePress,
	}, flatte.Effects[State]{})

	if s.selected != metricsZone {
		t.Fatalf("selected = %q, want unchanged metrics", s.selected)
	}
	if s.last != "outside" {
		t.Fatalf("last = %q, want outside", s.last)
	}
}

func TestResizeDistributesPanelZones(t *testing.T) {
	s := ready()

	settle(s, 80, 24)

	left, _ := s.zones.Rect(logsZone)
	right, _ := s.zones.Rect(metricsZone)
	if left.W != right.W {
		t.Fatalf("panel widths = %d,%d want equal", left.W, right.W)
	}
	if right.X != left.X+left.W+panelGap {
		t.Fatalf("right.X = %d, want %d", right.X, left.X+left.W+panelGap)
	}
}

func TestQuitKeys(t *testing.T) {
	s := ready()
	var quit bool
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestViewShowsZonesAndSelection(t *testing.T) {
	s := ready()
	s.selected = metricsZone
	s.last = "metrics local 1,2"

	frame := composed(View(s, flatte.RenderContext{Width: 72})).Content
	for _, want := range []string{"Flat Zones", "LOGS", "METRICS", "selected: metrics", "last: metrics local 1,2"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing %q:\n%s", want, frame)
		}
	}
}

func TestViewSnapshot(t *testing.T) {
	s := ready()

	flatest.AssertGoldenFrame(t, "testdata/zones.golden", composed(View(s, flatte.RenderContext{Width: 72})))
}
