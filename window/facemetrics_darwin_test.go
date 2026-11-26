package window

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

// The terminal face is the code face until a design sets it: then its
// size, line and family measure its cells, and code's stay as they were.
func TestTerminalFaceMetrics(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	base := ds.Default()
	m0 := newTheme(fromDesign(&base, true)).metrics()
	code := m0.Cell(layout.FaceCode)
	if term := m0.Cell(layout.FaceTerminal); term != code {
		t.Fatalf("default terminal cell %v, code %v", term, code)
	}

	big := ds.Default()
	big.Type.Terminal.Size = 20
	m1 := newTheme(fromDesign(&big, true)).metrics()
	if m1.Column != m0.Column || m1.Line != m0.Line {
		t.Errorf("code cell moved: %v×%v, was %v×%v", m1.Column, m1.Line, m0.Column, m0.Line)
	}
	if c := m1.Cell(layout.FaceTerminal); !(c.Column > code.Column*1.4) || !(c.Line > code.Line*1.3) {
		t.Errorf("20pt terminal %v, code %v", c, code)
	}
	// The terminal at the code size measures as code does.
	same := ds.Default()
	same.Type.Terminal.Size = same.Type.Code.Size
	if c := newTheme(fromDesign(&same, true)).metrics().Cell(layout.FaceTerminal); c != code {
		t.Errorf("terminal at the code size %v, code %v", c, code)
	}

	tall := ds.Default()
	tall.Type.Terminal.Line = 24.4
	if c := newTheme(fromDesign(&tall, true)).metrics().Cell(layout.FaceTerminal); c.Line != 24 || c.Column != code.Column {
		t.Errorf("24.4pt lines: %v (whole points, code's column %v)", c, code.Column)
	}

	// A family of its own, here a proportional one, has its own advance.
	fam := ds.Default()
	fam.TerminalFont = "Times New Roman"
	th := newTheme(fromDesign(&fam, true))
	if th.termFamily != "Times New Roman" {
		t.Fatalf("family %q", th.termFamily)
	}
	if c := th.metrics().Cell(layout.FaceTerminal); c.Column == code.Column || c.Line != code.Line {
		t.Errorf("terminal family cell %v, code %v", c, code)
	}

	// A window Theme sets the face directly; zero fields follow the base.
	wt := newTheme(Theme{TerminalSize: 18, TerminalLine: 30, TerminalFont: "Menlo"}.over(Dark()))
	if wt.termSize != 18 || wt.termLine != 30 || wt.termFamily != "Menlo" {
		t.Errorf("window theme face %q %v %v", wt.termFamily, wt.termSize, wt.termLine)
	}
	if d := newTheme(Dark()); d.termSize != d.monoSize || d.termLine != 0 || d.termFamily != "" {
		t.Errorf("default face %q %v %v", d.termFamily, d.termSize, d.termLine)
	}
}

// A grid set in the terminal face is drawn in its cells: a cell's
// background lands where the face's column and line put it.
func TestTerminalFaceDrawsInItsCells(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	d := ds.Default()
	d.Type.Terminal.Size = 2 * d.Type.Code.Size
	th := newTheme(fromDesign(&d, true))
	m := th.metrics()
	term := m.Cell(layout.FaceTerminal)
	red := lipgloss.NewStyle().Background(lipgloss.Color("#ff0000")).Render(" ")
	root := layout.Text{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)}, String: "\n  " + red, Face: layout.FaceTerminal}
	w, h := 10*term.Column, 3*term.Line
	img := renderScene(layout.LayoutScene(root, w, h, m), th, m, w, h, 1, nil, presentation{}, false)
	isRed := func(x, y float64) bool {
		c := img.RGBAAt(int(x), int(y))
		return c.R > 200 && c.G < 60 && c.B < 60
	}
	if !isRed(2.5*term.Column, 1.5*term.Line) {
		t.Error("the red cell is not at the terminal face's column 2, line 1")
	}
	code := m.Cell(layout.FaceCode)
	if isRed(2.5*code.Column, 1.5*code.Line) {
		t.Error("the red cell is in the code face's cells")
	}
}
