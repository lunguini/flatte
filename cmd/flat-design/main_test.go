package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/kit"
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
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown})
	golden(t, d, "components")
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown})
	golden(t, d, "shell")
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
	if !strings.Contains(flatest.CleanFrame(frame), "FLATTE") {
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

// The dials restyle the whole frame: the frame carries the theme they
// make, and the terminal's cells repaint in it.
func TestDialsRestyleLive(t *testing.T) {
	th := theme.Default()
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 40, Theme: &th})
	t.Cleanup(d.Close)
	if got := d.State().theme; got.Size != th.Size || got.Dark.Accent != th.Dark.Accent {
		t.Fatal("the dials do not start at the default theme")
	}
	press(t, d, kit.OptionID(idDensity, 0))
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'r'}) // subtle → round
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'a'}) // blue → teal
	s := d.State()
	want := theme.Default().With(theme.Seeds{Density: theme.Compact, Roundness: theme.Round, Accent: &accents[1].pair})
	if s.density != 0 || s.shape != 2 || s.accent != 1 || s.theme.Size != want.Size || s.theme.Radius != want.Radius || s.theme.Dark.Accent != want.Dark.Accent {
		t.Fatalf("dials %d/%d/%d, sizes %+v", s.density, s.shape, s.accent, s.theme.Size)
	}
	c := s.theme.Dark.SelectionActive
	if frame := d.Frame().Content; !strings.Contains(frame, fmt.Sprintf("48;2;%d;%d;%d", c.R, c.G, c.B)) {
		t.Error("cells not repainted in the teal selection")
	}
	for i := 0; i < len(accents); i++ {
		d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'a'})
	}
	if d.State().accent != 1 {
		t.Fatal("the accent dial does not cycle")
	}
}

// Every showcase theme passes the contrast floors.
func TestShowcaseThemesPassCheck(t *testing.T) {
	for i := range accents {
		for j := range densities {
			s := &State{accent: i, density: j, shape: 1}
			s.restyle()
			if p := s.theme.Check(); len(p) > 0 {
				t.Errorf("%s/%s: %v", accents[i].name, densityNames[j], p)
			}
		}
	}
}

func TestComponentsDemoClicks(t *testing.T) {
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 70}) // all of the page on screen
	t.Cleanup(d.Close)
	press := func(id string) { pressAt(t, d, id, 70) }
	press(pageID(pageComponents))
	press(rowPrefix + "2")
	press(tabPrefix + "1")
	press(kit.OptionID(idChoice, 2))
	press(idWrap)
	press(idOpen)
	s := d.State()
	if s.page != pageComponents || s.row != 2 || s.tab != 1 || s.choice != 2 || s.wrap || !s.popover {
		t.Fatalf("demo state page %d row %d tab %d choice %d wrap %v popover %v", s.page, s.row, s.tab, s.choice, s.wrap, s.popover)
	}
	if !strings.Contains(flatest.CleanFrame(d.Frame().Content), "flatui/kit") {
		t.Error("popover not shown")
	}
	d.Send(flatte.KeyEvent{Key: flatte.KeyEscape})
	if d.State().popover {
		t.Fatal("Escape kept the popover")
	}
}

// press clicks the middle of the node id in the driver's 120×40 frame, as
// a terminal reports a click (the runtime hit-tests it).
func press(t *testing.T, d *flatest.Driver[State], id string) { t.Helper(); pressAt(t, d, id, 40) }

func pressAt(t *testing.T, d *flatest.Driver[State], id string, height int) {
	t.Helper()
	ctx := flatte.RenderContext{Width: 120, Height: height}
	r, ok := layout.Compose(View(d.State(), ctx).Root, 120, height).Rects[id]
	if !ok {
		t.Fatalf("%s is not on screen", id)
	}
	d.Send(flatte.MouseEvent{X: r.X + r.W/2, Y: r.Y + r.H/2, Button: flatte.MouseLeft, Action: flatte.MousePress})
}

// The Controls group works from the keyboard: Tab walks the focus, typing
// fills the focused field, Space and ←/→ work the focused control.
func TestControlsFocusAndKeys(t *testing.T) {
	th := theme.Default()
	d := flatest.StartWith(app(), flatest.Options{Width: 120, Height: 120, Theme: &th})
	t.Cleanup(d.Close)
	key := func(k flatte.Key, r rune, mod flatte.Mod) { d.Send(flatte.KeyEvent{Key: k, Rune: r, Mod: mod}) }
	key(flatte.KeyCharacter, '4', 0)
	key(flatte.KeyTab, 0, 0)
	if f := d.State().focus; f != "primary" {
		t.Fatalf("first Tab focused %q", f)
	}
	key(flatte.KeyTab, 0, flatte.ModShift)
	key(flatte.KeyTab, 0, flatte.ModShift)
	if f := d.State().focus; f != controls[len(controls)-2] {
		t.Fatalf("Shift-Tab wrapped to %q", f)
	}
	pressAt(t, d, idQuery, 120)
	for _, r := range "dra" { // the dial keys type while the field has focus
		key(flatte.KeyCharacter, r, 0)
	}
	key(flatte.KeyBackspace, 0, 0)
	if s := d.State(); s.query != "dr" || s.density != 1 || s.focus != idQuery {
		t.Fatalf("typing: query %q density %d focus %q", s.query, s.density, s.focus)
	}
	key(flatte.KeyTab, 0, 0) // → check:format
	key(flatte.KeyCharacter, ' ', 0)
	if d.State().format {
		t.Error("Space did not clear the focused checkbox")
	}
	pressAt(t, d, idRadio+"0", 120)
	key(flatte.KeyRight, 0, 0)
	if s := d.State(); s.radio != 1 || s.focus != idRadio+"1" {
		t.Errorf("→ on a radio: %d, focus %q", s.radio, s.focus)
	}
	pressAt(t, d, idVolume, 120)
	v := d.State().volume
	key(flatte.KeyLeft, 0, 0)
	if got := d.State().volume; got >= v {
		t.Errorf("← on the slider: %v from %v", got, v)
	}
	pressAt(t, d, "switch:autosave", 120)
	if d.State().autosave {
		t.Error("clicking the switch did not turn it off")
	}
	key(flatte.KeyEscape, 0, 0)
	if f := d.State().focus; f != "" {
		t.Errorf("Esc left focus on %q", f)
	}
	// Every focusable control is on the page and lays out.
	r := layout.Compose(View(d.State(), flatte.RenderContext{Width: 120, Height: 120}).Root, 120, 120).Rects
	for _, id := range controls {
		if _, ok := r[id]; !ok {
			t.Errorf("control %s not laid out", id)
		}
	}
}

// The Appearance dial pins light or dark: both palettes become one.
func TestAppearanceDial(t *testing.T) {
	s := NewState()
	if s.theme.Dark == s.theme.Light {
		t.Fatal("System should keep two palettes")
	}
	click(s, kit.OptionID(idMode, 1))
	if s.theme.Dark != theme.Default().Light {
		t.Error("Light did not pin the light palette")
	}
	click(s, kit.OptionID(idMode, 2))
	if s.theme.Light != theme.Default().Dark {
		t.Error("Dark did not pin the dark palette")
	}
}
