package layout

// Face is the type a GUI sets a cell grid (a Text) in, as the design
// system names it (theme.Theme's fonts and type scale). Each face has its
// own cell size in points (Metrics.Cell); a terminal host ignores faces,
// since a cell is a cell there.
type Face uint8

const (
	// FaceCode is the code type (the theme's MonoFont and Type.Code): the
	// layout's own columns and lines.
	FaceCode Face = iota
	// FaceTerminal is the type of embedded terminals (the theme's
	// TerminalFont and Type.Terminal); unset, it is the code type.
	FaceTerminal
)

// CellSize is one cell of a face, in points: its fixed-pitch advance and
// its line height.
type CellSize struct{ Column, Line float64 }

// Cell is face f's cell under m: its entry in Faces, else (FaceCode, or a
// face without one) Column and Line.
func (m Metrics) Cell(f Face) CellSize {
	if c, ok := m.Faces[f]; ok && f != FaceCode && c.Column > 0 && c.Line > 0 {
		return c
	}
	return CellSize{Column: m.Column, Line: m.Line}
}

// Cells converts r to whole columns and lines of c, rounding each field
// down as PointRect.Cells does: how many of a face's cells fit in a node's
// point rect (LayoutEvent.PointRects).
func (c CellSize) Cells(r PointRect) Rect {
	return r.Cells(Metrics{Column: c.Column, Line: c.Line})
}
