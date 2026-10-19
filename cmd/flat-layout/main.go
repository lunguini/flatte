package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/persist"
	"github.com/lunguini/flatte/flatui/layout"
)

// State is the single mutable state struct. All fields are gob-serializable.
type State struct {
	ShowModal bool
	Cursor    int
}

const stateFile = ".flatte-state.gob"

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
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

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	hint := "m toggle modal  ·  j/k cursor  ·  q quit"
	if s.ShowModal {
		hint = fmt.Sprintf("esc/m close modal  ·  cursor: %d  ·  q quit", s.Cursor)
	}

	// The runtime solves and paints the tree at the terminal size.
	return flatte.Frame{
		Root:  buildTree(s),
		Title: "flat-layout — " + hint,
	}
}

func buildTree(s *State) layout.Node {
	children := []layout.Node{
		layout.Text{NodeBase: layout.NodeBase{ID: "header", H: layout.Fixed(3), Bordered: true}, String: " header"},
		layout.Row{
			NodeBase: layout.NodeBase{H: layout.Grow(1)},
			Children: []layout.Node{
				layout.Text{NodeBase: layout.NodeBase{ID: "sidebar", W: layout.Fixed(20)}, String: " sidebar"},
				layout.Text{NodeBase: layout.NodeBase{ID: "content", W: layout.Grow(1)}, String: fmt.Sprintf(" content  cursor=%d", s.Cursor)},
			},
		},
		layout.Text{NodeBase: layout.NodeBase{ID: "footer", H: layout.Fixed(1)}, String: " footer"},
	}
	if s.ShowModal {
		children = append(children, layout.Text{NodeBase: layout.NodeBase{
			ID: "modal", W: layout.Fixed(40), H: layout.Fixed(10), Overlay: true, Bordered: true,
		}, String: " modal"})
	}
	return layout.Col{Children: children}
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

	state, loadErr := persist.Load(stateFile, State{})

	var saveErr error
	runErr := flatte.Run(ctx, flatte.App[State]{
		State:  &state,
		Handle: Handle,
		View:   View,
		OnExit: func(s *State) {
			saveErr = persist.Save(stateFile, *s, loadErr)
		},
	}, flatte.WithMouse(flatte.MouseModeNone))
	// Run restores the terminal before returning, keeping the diagnostic
	// visible when this watcher restarts the sample.
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
	}
	if saveErr != nil {
		fmt.Fprintf(os.Stderr, "save state: %v\n", saveErr)
	}
	if runErr != nil || saveErr != nil {
		os.Exit(1)
	}
}
