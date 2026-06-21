package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestSelectorCommandUsesConfiguredShellCommand(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "printf ./chosen.txt")

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want true")
	}
	if label != "printf ./chosen.txt" {
		t.Fatalf("label = %q, want configured command", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "printf ./chosen.txt") {
		t.Fatalf("cmd args = %q, want configured command", got)
	}
}

func TestSelectorCommandFallsBackToFDAndFZFOnlyWhenPresent(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("freebsd")
	defer restoreOS()
	restore := stubLookPath(func(name string) (string, error) {
		if name == "fd" || name == "fzf" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	defer restore()

	_, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want true")
	}
	if label != "fd . | fzf" {
		t.Fatalf("label = %q, want fallback pipeline", label)
	}
}

func TestSelectorCommandPrefersMacOSNativePicker(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("darwin")
	defer restoreOS()
	restore := stubLookPath(func(name string) (string, error) {
		if name == "osascript" || name == "fd" || name == "fzf" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	defer restore()

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want native picker")
	}
	if label != "macOS file dialog" {
		t.Fatalf("label = %q, want macOS file dialog", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "choose file") {
		t.Fatalf("cmd args = %q, want osascript choose file", got)
	}
}

func TestSelectorCommandPrefersWindowsNativePicker(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("windows")
	defer restoreOS()
	restore := stubLookPath(func(name string) (string, error) {
		if name == "powershell" || name == "fd" || name == "fzf" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	defer restore()

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want native picker")
	}
	if label != "Windows file dialog" {
		t.Fatalf("label = %q, want Windows file dialog", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "OpenFileDialog") {
		t.Fatalf("cmd args = %q, want PowerShell OpenFileDialog", got)
	}
}

func TestSelectorCommandPrefersLinuxDesktopPicker(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("linux")
	defer restoreOS()
	restore := stubLookPath(func(name string) (string, error) {
		if name == "zenity" || name == "fd" || name == "fzf" {
			return "/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	defer restore()

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want desktop picker")
	}
	if label != "zenity file dialog" {
		t.Fatalf("label = %q, want zenity file dialog", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "--file-selection") {
		t.Fatalf("cmd args = %q, want zenity file selection", got)
	}
}

func TestSelectorCommandFallsBackToBuiltInSelectorWithoutFZF(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("freebsd")
	defer restoreOS()
	restore := stubLookPath(func(string) (string, error) {
		return "", errors.New("missing")
	})
	defer restore()
	restoreExecutable := stubExecutable(func() (string, error) {
		return "/tmp/flat-file-select", nil
	})
	defer restoreExecutable()

	cmd, label, ok := selectorCommand()
	if !ok {
		t.Fatal("selectorCommand() ok = false, want built-in fallback")
	}
	if label != "built-in selector" {
		t.Fatalf("label = %q, want built-in selector", label)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "--basic-selector") {
		t.Fatalf("cmd args = %q, want self selector flag", got)
	}
}

func TestOpenSelectorReportsMissingSelfSelector(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "")
	restoreOS := stubGOOS("freebsd")
	defer restoreOS()
	restore := stubLookPath(func(string) (string, error) {
		return "", errors.New("missing")
	})
	defer restore()
	restoreExecutable := stubExecutable(func() (string, error) {
		return "", errors.New("no executable")
	})
	defer restoreExecutable()

	s := NewState()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'o'}, flat.Effects[State]{})

	if s.status != "file selector unavailable: no executable" {
		t.Fatalf("status = %q, want unavailable", s.status)
	}
}

func TestBasicSelectorPrintsChosenPathOnlyOnStdout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	var selected, screen bytes.Buffer
	if err := runBasicSelector(dir, strings.NewReader("2\n"), &selected, &screen); err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(selected.String()); got != "b.txt" {
		t.Fatalf("selected stdout = %q, want b.txt", got)
	}
	for _, want := range []string{"Select a file", "1) a.txt", "2) b.txt"} {
		if !strings.Contains(screen.String(), want) {
			t.Fatalf("screen missing %q:\n%s", want, screen.String())
		}
	}
}

func TestBasicSelectorBlankInputCancels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	var selected, screen bytes.Buffer
	if err := runBasicSelector(dir, strings.NewReader("\n"), &selected, &screen); err != nil {
		t.Fatal(err)
	}
	if selected.String() != "" {
		t.Fatalf("selected stdout = %q, want empty cancel", selected.String())
	}
}

func TestOpenSelectorCapturesSelectedPathThroughRun(t *testing.T) {
	t.Setenv("FLAT_FILE_SELECTOR", "printf ' ./picked.txt\n'")
	state := NewState()
	settled := make(chan struct{})
	state.settled = settled
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flat.Run(context.Background(), flat.App[State]{
			State:  state,
			Handle: Handle,
			View:   View,
		}, flat.WithInput(reader), flat.WithOutput(&out))
	}()

	if _, err := writer.Write([]byte("o")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-settled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for selected state")
	}
	if state.path != "./picked.txt" {
		t.Fatalf("path = %q, want ./picked.txt", state.path)
	}

	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run")
	}
}

func TestInitialFrame(t *testing.T) {
	frame := View(NewState(), flat.RenderContext{Width: 72})
	clean := flatest.CleanFrame(frame.Content)
	for _, want := range []string{"Flat File Select", "status: ready", "selected: (none)", "o open selector"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("frame missing %q:\n%s", want, clean)
		}
	}
}

func stubLookPath(fn func(string) (string, error)) func() {
	previous := lookPath
	lookPath = fn
	return func() { lookPath = previous }
}

func stubExecutable(fn func() (string, error)) func() {
	previous := executable
	executable = fn
	return func() { executable = previous }
}

func stubGOOS(value string) func() {
	previous := goos
	goos = value
	return func() { goos = previous }
}
