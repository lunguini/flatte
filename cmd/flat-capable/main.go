// Command flat-capable dogfoods the Phase 5 capability surface: clipboard
// (OSC52 write + read), process suspend, and exec (shell out to $EDITOR).
// Each capability is a single explicit effect call from Handle.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

const clipboardLine = "flatte: copied from the capability demo"

// State is the single source of truth: a status line plus the most recent
// results of a clipboard read and an editor session.
type State struct {
	status     string
	clipboard  string
	editorText string
}

func NewState() *State {
	return &State{status: "ready"}
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch ev := ev.(type) {
	case flatte.ClipboardEvent:
		s.clipboard = ev.Text
		s.status = "clipboard read"
	case flatte.KeyEvent:
		handleKey(s, ev, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if key.Key != flatte.KeyCharacter {
		return
	}
	mod := key.Mod &^ flatte.ModShift
	if mod != 0 {
		if mod == flatte.ModCtrl && (key.Rune == 'z' || key.Rune == 'Z') {
			s.status = "suspended; resumed"
			fx.Suspend()
		}
		return
	}
	switch key.Rune {
	case 'y', 'Y':
		fx.SetClipboard(clipboardLine)
		s.status = "copied to clipboard"
	case 'p', 'P':
		fx.ReadClipboard()
		s.status = "requested clipboard read…"
	case 'z', 'Z':
		s.status = "suspended; resumed"
		fx.Suspend()
	case 'e', 'E':
		openEditor(s, fx)
	case 'q', 'Q':
		fx.Quit()
	}
}

func openEditor(s *State, fx flatte.Effects[State]) {
	file, err := os.CreateTemp("", "flat-capable-*.txt")
	if err != nil {
		s.status = "temp file: " + err.Error()
		return
	}
	_, _ = file.WriteString("edit this line, then save and quit your editor\n")
	name := file.Name()
	_ = file.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	s.status = "running " + editor + "…"
	flatte.Exec(fx, "editor", exec.Command(editor, name), func(s *State, err error) {
		defer func() { _ = os.Remove(name) }()
		if err != nil {
			s.status = "editor: " + err.Error()
			return
		}
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			s.status = "read back: " + readErr.Error()
			return
		}
		s.editorText = strings.TrimSpace(string(data))
		s.status = "editor closed"
	})
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	clip := s.clipboard
	if clip == "" {
		clip = "(none read yet)"
	}
	edited := s.editorText
	if edited == "" {
		edited = "(no editor session yet)"
	}

	lines := []string{
		flatui.Title("Flat Capable"),
		flatui.Subtle("clipboard · suspend · exec"),
		"",
		"  status: " + s.status,
		"  last clipboard read: " + clip,
		"  last editor text: " + edited,
		"",
		flatui.Subtle("y copy | p paste | z/Ctrl-Z suspend | e edit | q quit"),
	}
	return flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func runOptions() []flatte.Option {
	if os.Getenv("FLAT_CAPABLE_INLINE") == "" {
		return nil
	}
	return []flatte.Option{flatte.WithInline()}
}

func main() {
	state := NewState()
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	}, runOptions()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
