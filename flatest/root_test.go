package flatest

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type clicks struct {
	targets []string
	tall    bool
}

func rootApp() flatte.App[clicks] {
	return flatte.App[clicks]{
		State: &clicks{},
		Handle: func(s *clicks, ev flatte.Event, _ flatte.Effects[clicks]) {
			if m, ok := ev.(flatte.MouseEvent); ok {
				s.targets = append(s.targets, m.Target)
			}
		},
		View: func(s *clicks, _ flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Col{Children: []layout.Node{
				layout.Text{NodeBase: layout.NodeBase{ID: "top", H: layout.Fixed(1)}, String: "top"},
				layout.Text{NodeBase: layout.NodeBase{ID: "rest", H: layout.Grow(1)}, String: "rest"},
			}}}
		},
	}
}

func TestDriverComposesRootLikeRun(t *testing.T) {
	d := Start(rootApp(), 12)
	frame := d.Frame()
	want, _ := flatte.ComposeFrame(frame, 12, 24)
	if frame.Content == "" || frame.Content != want.Content {
		t.Fatalf("driver frame = %q, want the composed root %q", frame.Content, want.Content)
	}
	if got := len(strings.Split(frame.Content, "\n")); got != 24 {
		t.Fatalf("composed rows = %d, want the 24-row driver height", got)
	}
}

func TestDriverFillsMouseTarget(t *testing.T) {
	d := Start(rootApp(), 12)
	d.Send(flatte.MouseEvent{X: 0, Y: 0, Action: flatte.MousePress})
	d.Send(flatte.MouseEvent{X: 0, Y: 5, Action: flatte.MousePress})
	d.Send(flatte.MouseEvent{X: 30, Y: 0, Action: flatte.MousePress, Target: "stale"})
	if got, want := strings.Join(d.State().targets, ","), "top,rest,"; got != want {
		t.Fatalf("targets = %q, want %q", got, want)
	}
}

func TestDriverTracksResizeHeightForRoot(t *testing.T) {
	d := Start(rootApp(), 12)
	frame := d.Send(flatte.ResizeEvent{Width: 12, Height: 5})
	if got := len(strings.Split(frame.Content, "\n")); got != 5 {
		t.Fatalf("composed rows after resize = %d, want 5", got)
	}
	d.Send(flatte.MouseEvent{X: 0, Y: 6, Action: flatte.MousePress})
	if got := d.State().targets; len(got) != 1 || got[0] != "" {
		t.Fatalf("click below the resized frame = %q, want empty target", got)
	}
}

func TestDriverLeavesStringFramesAndTargetsAlone(t *testing.T) {
	d := Start(flatte.App[clicks]{
		State: &clicks{},
		Handle: func(s *clicks, ev flatte.Event, _ flatte.Effects[clicks]) {
			if m, ok := ev.(flatte.MouseEvent); ok {
				s.targets = append(s.targets, m.Target)
			}
		},
		View: func(*clicks, flatte.RenderContext) flatte.Frame { return flatte.Frame{Content: "plain"} },
	}, 12)
	if got := d.Frame().Content; got != "plain" {
		t.Fatalf("string frame = %q", got)
	}
	d.Send(flatte.MouseEvent{X: 0, Y: 0, Target: "scripted"})
	if got := d.State().targets; len(got) != 1 || got[0] != "" {
		t.Fatalf("string-frame target = %q, want empty (Run never sets one)", got)
	}
}

type pane struct {
	rows    int
	layouts int
	local   []int
}

// paneApp sizes a list from the LayoutEvent and records node-local click rows,
// the two geometry needs LayoutEvent and TargetRect exist for.
func paneApp() flatte.App[pane] {
	return flatte.App[pane]{
		State: &pane{},
		Handle: func(s *pane, ev flatte.Event, _ flatte.Effects[pane]) {
			switch ev := ev.(type) {
			case flatte.LayoutEvent:
				s.layouts++
				s.rows = ev.Rects["pane"].H - 2
			case flatte.MouseEvent:
				if ev.Target == "list" {
					s.local = append(s.local, ev.Y-ev.TargetRect.Y)
				}
			}
		},
		View: func(s *pane, _ flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Col{Children: []layout.Node{
				layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(2)}, String: "head"},
				layout.Col{NodeBase: layout.NodeBase{ID: "pane", H: layout.Grow(1), Bordered: true}, Children: []layout.Node{
					layout.Text{NodeBase: layout.NodeBase{ID: "list", H: layout.Fixed(max(s.rows, 0))}, String: strings.Repeat("row\n", max(s.rows-1, 0)) + "row"},
				}},
			}}}
		},
	}
}

func TestDriverDeliversLayoutBeforeFirstFrame(t *testing.T) {
	d := Start(paneApp(), 20)
	if got := d.State().rows; got != 20 { // 24 rows - 2 head - 2 border
		t.Fatalf("rows adopted at Start = %d, want 20", got)
	}
	if got := strings.Count(d.Frame().Content, "row"); got != 20 {
		t.Fatalf("first frame shows %d rows, want 20", got)
	}
	layouts := d.State().layouts
	d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'x'}) // no geometry change
	if d.State().layouts != layouts {
		t.Fatalf("unchanged geometry re-delivered: %d -> %d", layouts, d.State().layouts)
	}
	d.Send(flatte.ResizeEvent{Width: 20, Height: 10})
	if got := d.State().rows; got != 6 {
		t.Fatalf("rows after resize = %d, want 6", got)
	}
}

func TestDriverFillsTargetRect(t *testing.T) {
	d := Start(paneApp(), 20)
	d.Send(flatte.MouseEvent{X: 3, Y: 3, Action: flatte.MousePress}) // first list row: y=2 head, +1 border
	d.Send(flatte.MouseEvent{X: 3, Y: 7, Action: flatte.MousePress})
	if got := d.State().local; len(got) != 2 || got[0] != 0 || got[1] != 4 {
		t.Fatalf("node-local rows = %v, want [0 4]", got)
	}
}

func TestReplaySkipsRecordedLayoutEvents(t *testing.T) {
	rec := &Recorder{}
	app := paneApp()
	app.Tracer = rec
	d := Start(app, 20)
	d.Send(flatte.MouseEvent{X: 3, Y: 3, Action: flatte.MousePress})

	replayed := Replay(paneApp(), 20, rec)
	count := func(r *Recorder) int {
		n := 0
		for _, ev := range r.Events() {
			if _, ok := ev.(flatte.LayoutEvent); ok {
				n++
			}
		}
		return n
	}
	if got, want := count(replayed), count(rec); got != want || want == 0 {
		t.Fatalf("replayed layout events = %d, recorded = %d", got, want)
	}
}

func TestDriverFillsMouseCaret(t *testing.T) {
	type st struct{ caret int }
	d := Start(flatte.App[st]{
		State: &st{caret: -1},
		Handle: func(s *st, ev flatte.Event, _ flatte.Effects[st]) {
			if m, ok := ev.(flatte.MouseEvent); ok && m.HasCaret {
				s.caret = m.Caret
			}
		},
		View: func(*st, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Root: layout.Input{NodeBase: layout.NodeBase{ID: "in"}, Value: "hello"}}
		},
	}, 20)
	d.Send(flatte.MouseEvent{X: 2, Y: 0, Button: flatte.MouseLeft, Action: flatte.MousePress})
	if d.State().caret != 2 {
		t.Fatalf("caret = %d, want 2", d.State().caret)
	}
}
