package window

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

type snapshotThemeState struct {
	width    int
	ctxWidth int
	rects    map[string]layout.PointRect
	frames   int
	hideIcon bool
}

func snapshotThemeApp(s *snapshotThemeState, design *ds.Theme) flatte.App[snapshotThemeState] {
	return flatte.App[snapshotThemeState]{
		State: s,
		Handle: func(s *snapshotThemeState, ev flatte.Event, _ flatte.Effects[snapshotThemeState]) {
			switch ev := ev.(type) {
			case flatte.ResizeEvent:
				s.width = ev.Width
			case flatte.LayoutEvent:
				s.rects = ev.PointRects
			}
		},
		View: func(s *snapshotThemeState, ctx flatte.RenderContext) flatte.Frame {
			s.frames++
			s.ctxWidth = ctx.Width
			icon := layout.IconSidebarLeft
			if s.hideIcon {
				icon = layout.IconNone
			}
			return flatte.Frame{Theme: design, Root: layout.Row{
				NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillCanvas}},
				Children: []layout.Node{
					layout.Col{
						NodeBase: layout.NodeBase{ID: "rail", W: layout.Points(56), H: layout.Grow(1), PanePadding: true, Box: layout.Box{Fill: layout.FillChrome}},
						Children: []layout.Node{
							layout.Button{NodeBase: layout.NodeBase{ID: "rail.icon", W: layout.ControlSize(), H: layout.ControlSize()}, Icon: icon, Label: " "},
							layout.Label{NodeBase: layout.NodeBase{ID: "rail.label"}, Text: "Files"},
						},
					},
					layout.Col{
						NodeBase: layout.NodeBase{ID: "pane", W: layout.Grow(1), H: layout.Grow(1), PanePadding: true, Box: layout.Box{Fill: layout.FillContent}},
						Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "pane.label"}, Text: fmt.Sprintf("Columns %d", s.width)}},
					},
				},
			}}
		},
	}
}

// Snapshot must use the frame's theme for both geometry and paint, including
// the resize that a real window receives after its cell metrics change.
func TestSnapshotRendersFrameThemeWithRestyledGeometry(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	design := ds.Default().With(ds.Seeds{Density: ds.Compact})
	design.Type.Code.Size = 22
	opts := Options{Width: 80, Height: 24, Appearance: flatte.AppearanceDark}
	actualState := &snapshotThemeState{}
	actual, err := Snapshot(snapshotThemeApp(actualState, &design), opts, 1)
	if err != nil {
		t.Fatal(err)
	}

	referenceState := &snapshotThemeState{}
	referenceApp := snapshotThemeApp(referenceState, &design)
	expected, scene := snapshotThemeReference(t, referenceApp, opts, design)
	if actual.Bounds() != expected.Bounds() || !bytes.Equal(actual.(*image.RGBA).Pix, expected.Pix) {
		t.Fatal("snapshot differs from the render laid out in its frame theme")
	}
	if actualState.width != referenceState.width {
		t.Errorf("ResizeEvent width %d, want %d after restyle", actualState.width, referenceState.width)
	}
	if actualState.ctxWidth != referenceState.ctxWidth {
		t.Errorf("View width %d, want %d after restyle", actualState.ctxWidth, referenceState.ctxWidth)
	}
	for _, id := range []string{"rail.icon", "pane.label"} {
		if actualState.rects[id] != referenceState.rects[id] {
			t.Errorf("%s rect %+v, want %+v", id, actualState.rects[id], referenceState.rects[id])
		}
	}
	if got, want := actualState.rects["pane.label"].X-actualState.rects["pane"].X, float64(design.Space[1]); got != want {
		t.Errorf("pane inset %vpt, want %vpt", got, want)
	}
	var icon bool
	for _, op := range scene.Ops {
		if control, ok := op.(layout.ControlOp); ok && control.ID == "rail.icon" && control.Icon == layout.IconSidebarLeft {
			icon = true
		}
	}
	if !icon {
		t.Fatal("reference render has no side rail icon")
	}
	withoutIconState := &snapshotThemeState{hideIcon: true}
	withoutIcon, _ := snapshotThemeReference(t, snapshotThemeApp(withoutIconState, &design), opts, design)
	if differentPixels(actual, withoutIcon, actualState.rects["rail.icon"]) == 0 {
		t.Fatal("snapshot has no pixels for the side rail icon")
	}
}

func differentPixels(a, b image.Image, r layout.PointRect) int {
	area := image.Rect(int(math.Floor(r.X)), int(math.Floor(r.Y)), int(math.Ceil(r.X+r.W)), int(math.Ceil(r.Y+r.H)))
	area = area.Intersect(a.Bounds()).Intersect(b.Bounds())
	different := 0
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				different++
			}
		}
	}
	return different
}

func snapshotThemeReference(t *testing.T, app flatte.App[snapshotThemeState], opts Options, design ds.Theme) (*image.RGBA, layout.Scene) {
	t.Helper()
	session := flatte.NewSession(context.Background(), app)
	session.UseTheme(design)
	dark := opts.dark(true)
	baseDesign := ds.Default()
	initial := newTheme(themeFor(opts.Theme, dark, &baseDesign)).metrics()
	th := newTheme(themeFor(opts.Theme, dark, session.Theme(true)))
	m := th.metrics()
	cols, lines := opts.size()
	width, height := float64(cols)*initial.Column, float64(lines)*initial.Line
	h := newSessionHost(context.Background(), session, m, func(func()) {}, func() {}, func() {})
	h.restyle = func(d *ds.Theme) layout.Metrics {
		th = newTheme(themeFor(opts.Theme, dark, d))
		m = th.metrics()
		return m
	}
	defer h.cancel()
	defer session.Cancel()
	h.surface.setAppearance(th.appearance)
	h.start(width, height)
	scene := h.surface.render()
	return renderScene(scene, th, m, width, height, 1, h.frameCursor(), presentation{}, false, nil), scene
}
