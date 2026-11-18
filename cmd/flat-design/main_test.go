package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/theme"
)

func app() flatte.App[State] { return flatte.App[State]{State: NewState(), Handle: Handle, View: View} }

// golden compares the frame with testdata/<name>.golden (ANSI stripped,
// 120×40); goldens are edited by hand when a view change is intended.
func golden(t *testing.T, d *flatest.Driver[State], name string) {
	t.Helper()
	got := flatest.CleanFrame(d.Frame().Content)
	path := filepath.Join("testdata", name+".golden")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (frame was:\n%s)", err, got)
	}
	if got != string(want) {
		t.Fatalf("%s differs:\n got:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestPagesInATerminal(t *testing.T) {
	th := theme.Default()
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 40, Theme: &th})
	t.Cleanup(d.Close)
	golden(t, d, "colors")
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown})
	golden(t, d, "type")
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '3'})
	golden(t, d, "shape")
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown}) // wraps to the first
	if d.State().page != pageColors {
		t.Fatalf("page %d after wrapping", d.State().page)
	}
}

func TestClickAPage(t *testing.T) {
	th := theme.Default()
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 40, Theme: &th})
	t.Cleanup(d.Close)
	for y, line := range strings.Split(flatest.CleanFrame(d.Frame().Content), "\n") {
		if x := strings.Index(line, "Type"); x >= 0 {
			d.Send(flatte.MouseEvent{X: x, Y: y, Button: flatte.MouseLeft, Action: flatte.MousePress})
			break
		}
	}
	if d.State().page != pageType {
		t.Fatalf("clicking Type showed page %d", d.State().page)
	}
}

func TestThemedCellsPaintTheSurfaces(t *testing.T) {
	th := theme.Default()
	layout.UseCellTheme(&th.Dark)
	t.Cleanup(func() { layout.UseCellTheme(nil) })
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 40, Theme: &th})
	t.Cleanup(d.Close)
	frame := d.Frame().Content
	for name, rgb := range map[string]string{"chrome": "26;29;34", "content": "31;34;40", "selection": "36;64;107"} {
		if !strings.Contains(frame, "48;2;"+rgb) {
			t.Errorf("no %s background in the themed frame", name)
		}
	}
	if !strings.Contains(flatest.CleanFrame(frame), "DESIGN SYSTEM") {
		t.Error("the caption is not upper case")
	}
}

func TestQuit(t *testing.T) {
	d := flatest.Start(app(), 120)
	t.Cleanup(d.Close)
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'})
	if !d.Quit() {
		t.Fatal("q did not quit")
	}
}
