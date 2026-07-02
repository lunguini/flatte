package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// State is the single mutable state struct. All fields are gob-serializable.
type State struct {
	Width     int
	Height    int
	ShowModal bool
	Cursor    int
}

const stateFile = ".flatte-state.gob"

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ResizeEvent:
		s.Width = ev.Width
		s.Height = ev.Height
	case flatte.KeyEvent:
		switch ev.Key {
		case flatte.KeyCharacter:
			switch ev.Rune {
			case 'q', 'Q':
				fx.Quit()
			case 'm', 'M':
				s.ShowModal = !s.ShowModal
			case 'j', 'J':
				s.Cursor++
			case 'k', 'K':
				if s.Cursor > 0 {
					s.Cursor--
				}
			}
		case flatte.KeyEscape:
			if s.ShowModal {
				s.ShowModal = false
			} else {
				fx.Quit()
			}
		}
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	w := ctx.Width
	if s.Width > 0 {
		w = s.Width
	}
	h := s.Height
	if h <= 0 {
		h = 24
	}

	tree := buildTree(s)
	rects := layout.Solve(tree, w, h)
	content := renderBlocks(rects, w, h, s.Cursor)

	hint := "m toggle modal  ·  j/k cursor  ·  q quit"
	if s.ShowModal {
		hint = fmt.Sprintf("esc/m close modal  ·  cursor: %d  ·  q quit", s.Cursor)
	}

	return flatte.Frame{
		Content: content,
		Title:   "flat-layout — " + hint,
	}
}

func buildTree(s *State) layout.Node {
	children := []layout.Node{
		layout.Text{NodeBase: layout.NodeBase{ID: "header", H: layout.Fixed(3), Bordered: true}},
		layout.Row{
			NodeBase: layout.NodeBase{H: layout.Grow(1)},
			Children: []layout.Node{
				layout.Text{NodeBase: layout.NodeBase{ID: "sidebar", W: layout.Fixed(20)}},
				layout.Text{NodeBase: layout.NodeBase{ID: "content", W: layout.Grow(1)}},
			},
		},
		layout.Text{NodeBase: layout.NodeBase{ID: "footer", H: layout.Fixed(1)}},
	}
	if s.ShowModal {
		children = append(children, layout.Text{NodeBase: layout.NodeBase{
			ID: "modal", W: layout.Fixed(40), H: layout.Fixed(10), Overlay: true, Bordered: true,
		}})
	}
	return layout.Col{Children: children}
}

func renderBlocks(rects map[string]layout.Rect, w, h, cursor int) string {
	type cell struct {
		ch rune
		bg string
		fg string
	}

	grid := make([][]cell, h)
	for y := range grid {
		grid[y] = make([]cell, w)
		for x := range grid[y] {
			grid[y][x] = cell{ch: ' ', bg: "234", fg: "240"}
		}
	}

	paint := func(id, bg, fg, label string) {
		r, ok := rects[id]
		if !ok {
			return
		}
		for y := r.Y; y < r.Y+r.H && y >= 0 && y < h; y++ {
			for x := r.X; x < r.X+r.W && x >= 0 && x < w; x++ {
				grid[y][x] = cell{ch: ' ', bg: bg, fg: fg}
			}
		}
		for i, ch := range label {
			x := r.X + i
			if x >= 0 && x < r.X+r.W && x < w && r.Y >= 0 && r.Y < h {
				grid[r.Y][x] = cell{ch: ch, bg: bg, fg: fg}
			}
		}
	}

	paint("header", "25", "255", " header")
	paint("sidebar", "22", "255", " sidebar")
	paint("content", "236", "245", fmt.Sprintf(" content  cursor=%d", cursor))
	paint("footer", "88", "255", " footer")
	paint("modal", "90", "255", " modal")

	var lines []string
	for y := 0; y < h; y++ {
		var sb strings.Builder
		prevBg := ""
		for x := 0; x < w; x++ {
			c := grid[y][x]
			if c.bg != prevBg {
				fmt.Fprintf(&sb, "\x1b[48;5;%sm\x1b[38;5;%sm", c.bg, c.fg)
				prevBg = c.bg
			}
			sb.WriteRune(c.ch)
		}
		sb.WriteString("\x1b[0m")
		lines = append(lines, sb.String())
	}
	return strings.Join(lines, "\n")
}

// main runs the layout validation harness with a save-state/reload loop.
//
// State loop: edit any .go file → external watcher (wgo/air) kills this
// process with SIGTERM → OnExit saves state to .flatte-state.gob → watcher
// restarts → LoadState restores cursor/modal/size on boot.
//
// Known costs (acceptable for prototyping):
//   - Brief terminal re-init flicker on each restart.
//   - State resets when the State struct shape changes (the decode-error
//     fallback in LoadState handles this gracefully — no crash, just defaults).
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	defer signal.Stop(sigCh)

	state := flatte.LoadState(stateFile, State{})

	err := flatte.Run(ctx, flatte.App[State]{
		State:  &state,
		Handle: Handle,
		View:   View,
		OnExit: func(s *State) {
			_ = flatte.SaveState(stateFile, *s)
		},
	}, flatte.WithMouse(flatte.MouseModeNone))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
