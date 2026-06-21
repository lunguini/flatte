package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

type State struct {
	status  string
	path    string
	settled chan struct{}
}

func NewState() *State {
	return &State{status: "ready"}
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	key, ok := ev.(flat.KeyEvent)
	if !ok || key.Key != flat.KeyCharacter {
		return
	}
	switch key.Rune {
	case 'o', 'O':
		openSelector(s, fx)
	case 'q', 'Q':
		fx.Quit()
	}
}

func openSelector(s *State, fx flat.Effects[State]) {
	cmd, label, ok := selectorCommand()
	if !ok {
		s.status = "file selector unavailable"
		if label != "" {
			s.status += ": " + label
		}
		return
	}
	s.status = "running " + label + "..."
	flat.SelectFile(fx, "file.select", cmd, func(s *State, selection flat.FileSelection) {
		switch {
		case selection.Err == nil:
			s.path = selection.Path
			s.status = "selected"
		case errors.Is(selection.Err, flat.ErrNoSelection):
			s.status = "no selection"
		default:
			s.status = "selector: " + selection.Err.Error()
		}
		if s.settled != nil {
			close(s.settled)
			s.settled = nil
		}
	})
}

func selectorCommand() (*exec.Cmd, string, bool) {
	if configured := os.Getenv("FLAT_FILE_SELECTOR"); configured != "" {
		return shellCommand(configured), configured, true
	}
	if _, err := lookPath("fd"); err != nil {
		return selfSelectorCommand()
	}
	if _, err := lookPath("fzf"); err != nil {
		return selfSelectorCommand()
	}
	return shellCommand("fd . | fzf"), "fd . | fzf", true
}

func selfSelectorCommand() (*exec.Cmd, string, bool) {
	exe, err := executable()
	if err != nil {
		return nil, err.Error(), false
	}
	return exec.Command(exe, "--basic-selector"), "built-in selector", true
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

var lookPath = exec.LookPath
var executable = os.Executable

func runBasicSelector(root string, input io.Reader, selected io.Writer, screen io.Writer) error {
	files, err := listSelectableFiles(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(screen, "No files found.")
		return nil
	}

	fmt.Fprintln(screen, "Select a file:")
	for i, file := range files {
		fmt.Fprintf(screen, "%d) %s\n", i+1, file)
	}
	fmt.Fprint(screen, "> ")

	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	choice, err := strconv.Atoi(line)
	if err != nil || choice < 1 || choice > len(files) {
		return fmt.Errorf("invalid selection %q", line)
	}
	fmt.Fprintln(selected, files[choice-1])
	return nil
}

func listSelectableFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	path := s.path
	if path == "" {
		path = "(none)"
	}
	lines := []string{
		flatui.Title("Flat File Select"),
		flatui.Subtle("terminal-delegated selector"),
		"",
		"  status: " + s.status,
		"  selected: " + path,
		"",
		flatui.Subtle("o open selector | q quit"),
	}
	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--basic-selector" {
		if err := runBasicSelector(".", os.Stdin, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
