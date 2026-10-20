package main

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func start() *flatest.Driver[State] {
	return flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 80)
}

func key(r rune) flatte.KeyEvent { return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r} }

func wheel(x, y int, b flatte.MouseButton) flatte.MouseEvent {
	return flatte.MouseEvent{X: x, Y: y, Button: b, Action: flatte.MousePress}
}

func click(x, y int) flatte.MouseEvent {
	return flatte.MouseEvent{X: x, Y: y, Button: flatte.MouseLeft, Action: flatte.MousePress}
}

// At 80×24: title row 0, pane borders on rows 1 and 22, footer row 23. The
// list window is rows 2..21 (20 rows); the log window is rows 3..21 (19 rows,
// under its one-row header).
const (
	listX, logX = 3, 40
	listTop     = 2
	listRows    = 20
	logRows     = 19
)

func TestViewGolden(t *testing.T) {
	d := start()
	flatest.AssertGoldenFrame(t, "testdata/scroll.golden", d.Frame())
}

func TestSelectionKeepsRowVisible(t *testing.T) {
	d := start()
	var frame flatte.Frame
	for range 25 {
		frame = d.Send(key('j'))
	}
	s := d.State()
	if s.selected != 25 {
		t.Fatalf("selected = %d, want 25", s.selected)
	}
	if want := 25 - listRows + 1; s.listY != want {
		t.Fatalf("listY = %d, want %d (selected row on the bottom edge)", s.listY, want)
	}
	flatest.AssertGoldenFrame(t, "testdata/scroll-selected.golden", frame)

	// Moving back up inside the window does not scroll.
	d.Send(key('k'))
	if s.listY != 6 {
		t.Fatalf("listY after k = %d, want 6", s.listY)
	}
	// Past the top edge, the window follows the selection.
	for range 20 {
		d.Send(key('k'))
	}
	if s.selected != 4 || s.listY != 4 {
		t.Fatalf("selected, listY = %d, %d, want 4, 4", s.selected, s.listY)
	}
}

func TestWheelScrollsPaneUnderPointer(t *testing.T) {
	d := start()
	s := d.State()
	d.Send(wheel(logX, 10, flatte.MouseWheelDown))
	if s.logY != wheelStep || s.listY != 0 {
		t.Fatalf("wheel over log: logY, listY = %d, %d, want %d, 0", s.logY, s.listY, wheelStep)
	}
	d.Send(wheel(listX, 10, flatte.MouseWheelDown))
	d.Send(wheel(listX, 10, flatte.MouseWheelDown))
	if s.listY != 2*wheelStep || s.selected != 0 {
		t.Fatalf("wheel over list: listY, selected = %d, %d, want %d, 0", s.listY, s.selected, 2*wheelStep)
	}
	// The selection is scrolled out of view and stays that way: later
	// layout passes must not snap the window back to it.
	d.Send(wheel(logX, 10, flatte.MouseWheelUp))
	if s.listY != 2*wheelStep || s.logY != 0 {
		t.Fatalf("after wheel up over log: listY, logY = %d, %d", s.listY, s.logY)
	}
	// Wheel outside both panes does nothing.
	d.Send(wheel(0, 0, flatte.MouseWheelDown))
	if s.listY != 2*wheelStep || s.logY != 0 {
		t.Fatalf("wheel on title moved a pane: listY, logY = %d, %d", s.listY, s.logY)
	}
}

func TestClickSelectsScrolledRow(t *testing.T) {
	d := start()
	for range 3 {
		d.Send(wheel(listX, 10, flatte.MouseWheelDown))
	}
	s := d.State()
	d.Send(click(listX, listTop))
	if s.selected != 3*wheelStep {
		t.Fatalf("click on top visible row selected %d, want %d", s.selected, 3*wheelStep)
	}
	d.Send(click(listX, listTop+listRows-1))
	if want := 3*wheelStep + listRows - 1; s.selected != want {
		t.Fatalf("click on bottom visible row selected %d, want %d", s.selected, want)
	}
	if s.listY != 3*wheelStep {
		t.Fatalf("clicking a visible row scrolled the list: listY = %d", s.listY)
	}
}

func TestOffsetsClampAtEnd(t *testing.T) {
	d := start()
	s := d.State()
	for range 10 {
		d.Send(flatte.KeyEvent{Key: flatte.KeyPageDown})
	}
	if want := logLines - logRows; s.logY != want {
		t.Fatalf("logY after paging past the end = %d, want %d", s.logY, want)
	}
	// Scrolling back responds at once: no hidden overshoot to unwind.
	d.Send(flatte.KeyEvent{Key: flatte.KeyPageUp})
	if want := logLines - 2*logRows; s.logY != want {
		t.Fatalf("logY after one page up = %d, want %d", s.logY, want)
	}
	for range 30 {
		d.Send(wheel(listX, 10, flatte.MouseWheelDown))
	}
	if want := len(s.services) - listRows; s.listY != want {
		t.Fatalf("listY after wheeling past the end = %d, want %d", s.listY, want)
	}
	if got := flatest.CleanFrame(d.Frame().Content); !strings.Contains(got, "service-40") {
		t.Fatalf("last service not visible at the end:\n%s", got)
	}
	d.Send(wheel(listX, 10, flatte.MouseWheelUp))
	if want := len(s.services) - listRows - wheelStep; s.listY != want {
		t.Fatalf("listY after one wheel up = %d, want %d", s.listY, want)
	}
}

func TestResizeKeepsSelectionVisible(t *testing.T) {
	d := start()
	for range 15 {
		d.Send(key('j'))
	}
	s := d.State()
	if s.listY != 0 {
		t.Fatalf("setup listY = %d, want 0", s.listY)
	}
	d.Send(flatte.ResizeEvent{Width: 80, Height: 12}) // list window: 8 rows
	if want := 15 - 8 + 1; s.listY != want {
		t.Fatalf("listY after shrinking = %d, want %d", s.listY, want)
	}
	d.Send(flatte.ResizeEvent{Width: 80, Height: 60}) // everything fits
	if s.listY != 0 || s.logY != 0 {
		t.Fatalf("offsets after growing = %d, %d, want 0, 0", s.listY, s.logY)
	}
}

func TestSelectingResetsLog(t *testing.T) {
	d := start()
	s := d.State()
	d.Send(flatte.KeyEvent{Key: flatte.KeyPageDown})
	if s.logY == 0 {
		t.Fatal("setup: page down did not scroll the log")
	}
	frame := flatest.CleanFrame(d.Send(key('j')).Content)
	if s.logY != 0 {
		t.Fatalf("logY after selecting another service = %d, want 0", s.logY)
	}
	if !strings.Contains(frame, "service-02") || !strings.Contains(frame, "line 01") {
		t.Fatalf("log pane does not show the new service from the top:\n%s", frame)
	}
}

func TestEndKeysAndQuit(t *testing.T) {
	d := start()
	s := d.State()
	d.Send(key('G'))
	if s.selected != len(s.services)-1 || s.listY != len(s.services)-listRows {
		t.Fatalf("G: selected, listY = %d, %d", s.selected, s.listY)
	}
	d.Send(key('g'))
	if s.selected != 0 || s.listY != 0 {
		t.Fatalf("g: selected, listY = %d, %d", s.selected, s.listY)
	}
	d.Send(flatte.KeyEvent{Key: flatte.KeyEnd})
	if s.logY != logLines-logRows {
		t.Fatalf("End: logY = %d", s.logY)
	}
	d.Send(flatte.KeyEvent{Key: flatte.KeyHome})
	if s.logY != 0 {
		t.Fatalf("Home: logY = %d", s.logY)
	}
	d.Send(key('q'))
	if !d.Quit() {
		t.Fatal("q did not quit")
	}
}
