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
