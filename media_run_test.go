package flatte

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestRunHostMediaLifecycle(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	commands := make(chan MediaEvent, 1)
	type state struct {
		item    NowPlaying
		handled bool
	}
	s := state{item: NowPlaying{Title: "before"}}
	var snapshots []*NowPlaying
	commands <- MediaEvent{Command: MediaNext}
	err = Run(ctx, App[state]{
		State:      &s,
		View:       func(*state, RenderContext) Frame { return Frame{Content: "same"} },
		NowPlaying: func(s *state) *NowPlaying { return &s.item },
		Handle: func(s *state, e Event, fx Effects[state]) {
			if m, ok := e.(MediaEvent); ok {
				if m.Command != MediaNext {
					t.Errorf("command = %v", m.Command)
				}
				s.handled = true
				s.item.Title = "after"
				close(commands)

			}
		},
	}, WithInput(reader), WithOutput(io.Discard), withHostMedia(commands, func(n *NowPlaying) {
		snapshots = append(snapshots, n)
		if n != nil && n.Title == "after" {
			cancel()
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !s.handled {
		t.Fatal("media command not delivered")
	}
	if len(snapshots) < 3 || snapshots[0] == nil || snapshots[0].Title != "before" {
		t.Fatalf("initial copied snapshot: %+v", snapshots)
	}
	found := false
	for _, n := range snapshots {
		if n != nil && n.Title == "after" {
			found = true
		}
	}
	if !found {
		t.Fatal("unchanged frame skipped updated metadata")
	}
	if snapshots[len(snapshots)-1] != nil {
		t.Fatal("shutdown did not clear metadata")
	}
}
