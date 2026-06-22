package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func ready() *State {
	s := NewState()
	s.layout(72)
	return s
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
	s := NewState()

	Handle(s, flatte.ResizeEvent{Width: 80, Height: 24}, flatte.Effects[State]{})

	left, _ := s.zones.Rect(logsZone)
	right, _ := s.zones.Rect(metricsZone)
	if left.Width != right.Width {
		t.Fatalf("panel widths = %d,%d want equal", left.Width, right.Width)
	}
	if right.X != left.X+left.Width+panelGap {
		t.Fatalf("right.X = %d, want %d", right.X, left.X+left.Width+panelGap)
	}
}

func TestQuitKeys(t *testing.T) {
	s := ready()
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestViewShowsZonesAndSelection(t *testing.T) {
	s := ready()
	s.selected = metricsZone
	s.last = "metrics local 1,2"

	frame := View(s, flatte.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat Zones", "LOGS", "METRICS", "selected: metrics", "last: metrics local 1,2"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing %q:\n%s", want, frame)
		}
	}
}

func TestViewSnapshot(t *testing.T) {
	s := ready()

	flatest.AssertGoldenFrame(t, "testdata/zones.golden", View(s, flatte.RenderContext{Width: 72}))
}
