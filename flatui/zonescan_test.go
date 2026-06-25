package flatui

import (
	"strings"
	"testing"
)

func TestZoneScannerFindsSingleLineMark(t *testing.T) {
	s := NewZoneScanner()
	frame := "hello " + Mark("btn", "click me") + " world"
	s.Scan(frame)

	rect, ok := s.Rect("btn")
	if !ok {
		t.Fatal("btn zone not found")
	}
	if rect.X != 6 || rect.Y != 0 || rect.Width != 8 || rect.Height != 1 {
		t.Fatalf("btn rect = %+v, want {6,0,8,1}", rect)
	}
}

func TestZoneScannerAtReturnsMark(t *testing.T) {
	s := NewZoneScanner()
	frame := Mark("left", "L") + "|" + Mark("right", "R")
	s.Scan(frame)

	if id, _ := s.At(0, 0); id != "left" {
		t.Errorf("At(0,0) = %q, want left", id)
	}
	if id, _ := s.At(2, 0); id != "right" {
		t.Errorf("At(2,0) = %q, want right", id)
	}
	if _, ok := s.At(1, 0); ok {
		t.Error("At(1,0) should not be inside a zone")
	}
}

func TestZoneScannerHandlesAnsiInsideContent(t *testing.T) {
	s := NewZoneScanner()
	styled := "\x1b[31mred\x1b[0m text"
	frame := Mark("zone", styled)
	s.Scan(frame)

	rect, ok := s.Rect("zone")
	if !ok {
		t.Fatal("zone not found")
	}
	if rect.Width != 8 {
		t.Fatalf("width = %d, want 8 (ANSI does not advance x); rect %+v", rect.Width, rect)
	}
}

func TestZoneScannerLaterMarkWins(t *testing.T) {
	s := NewZoneScanner()
	frame := Mark("a", "xxxxx") + "\n" + Mark("b", "xxx")
	s.Scan(frame)

	if id, _ := s.At(2, 0); id != "a" {
		t.Errorf("At(2,0) = %q, want a", id)
	}
	if id, _ := s.At(0, 1); id != "b" {
		t.Errorf("At(0,1) = %q, want b", id)
	}
}

func TestZoneScannerSequentialNonOverlapping(t *testing.T) {
	s := NewZoneScanner()
	frame := Mark("a", "xxxxx") + Mark("b", "xxx")
	s.Scan(frame)

	if id, _ := s.At(2, 0); id != "a" {
		t.Errorf("At(2,0) = %q, want a (a covers 0-4)", id)
	}
	if id, _ := s.At(6, 0); id != "b" {
		t.Errorf("At(6,0) = %q, want b (b covers 5-7)", id)
	}
}

func TestZoneScannerHandlesMultibyteContent(t *testing.T) {
	s := NewZoneScanner()
	frame := Mark("emoji", "● ● ●")
	s.Scan(frame)

	rect, ok := s.Rect("emoji")
	if !ok {
		t.Fatal("emoji zone not found")
	}
	if rect.Width != 5 {
		t.Fatalf("width = %d, want 5 (3 dots + 2 spaces)", rect.Width)
	}
}

func TestZoneScannerEmptyMark(t *testing.T) {
	s := NewZoneScanner()
	frame := "before" + Mark("empty", "") + "after"
	s.Scan(frame)

	rect, ok := s.Rect("empty")
	if !ok {
		t.Fatal("empty zone not registered")
	}
	if rect.Width != 0 {
		t.Fatalf("empty zone width = %d, want 0", rect.Width)
	}
}

func TestZoneScannerSurvivesWrappedContent(t *testing.T) {
	s := NewZoneScanner()
	frame := "first\n" + Mark("wrapped", "ab\ncd") + "\nlast"
	s.Scan(frame)

	rect, ok := s.Rect("wrapped")
	if !ok {
		t.Fatal("wrapped zone not found")
	}
	if rect.Height != 2 {
		t.Fatalf("height = %d, want 2 (wrapped across line)", rect.Height)
	}
}

func TestMarkIsInvisible(t *testing.T) {
	marked := Mark("id", "content")
	if strings.Contains(marked, "content") == false {
		t.Fatal("content must be preserved inside Mark")
	}
	if !strings.HasPrefix(marked, zoneMarkStart) {
		t.Fatalf("Mark should start with zone marker prefix; got %q", marked[:20])
	}
}
