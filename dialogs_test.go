package flatte

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type dialogState struct {
	paths  []string
	path   string
	button int
	err    error
	folds  int
}

// queueFx is an Effects whose updates are applied by apply.
func queueFx() (Effects[dialogState], func(*dialogState)) {
	var queued []StateUpdate[dialogState]
	fx := NewHarnessEffectsQueue(context.Background(), func(u StateUpdate[dialogState]) bool {
		queued = append(queued, u)
		return true
	}, nil, nil, nil)
	return fx, func(s *dialogState) {
		for _, u := range queued {
			u.Apply(s)
		}
		queued = nil
	}
}

func TestDialogsWithoutAHost(t *testing.T) {
	fx, apply := queueFx()
	s := &dialogState{}
	OpenFile(fx, "open", OpenDialog{}, func(s *dialogState, p []string, err error) { s.paths, s.err = p, err; s.folds++ })
	SaveFile(fx, "save", SaveDialog{}, func(s *dialogState, p string, err error) { s.path, s.err = p, err; s.folds++ })
	Alert(fx, "alert", AlertDialog{}, func(s *dialogState, b int) { s.button = b; s.folds++ })
	apply(s)
	if s.folds != 3 || s.paths != nil || s.path != "" || s.button != -1 || !errors.Is(s.err, ErrNoDialogs) {
		t.Fatalf("%+v", s)
	}
}

func TestDialogsThroughAHost(t *testing.T) {
	fx, apply := queueFx()
	var shown []Dialog
	var answers []func(DialogResult)
	fx = WithHostDialogs(fx, func(d Dialog, done func(DialogResult)) {
		shown = append(shown, d)
		answers = append(answers, done)
	})
	s := &dialogState{}
	OpenFile(fx, "open", OpenDialog{Title: "Pick", Multiple: true}, func(s *dialogState, p []string, err error) { s.paths, s.err = p, err })
	SaveFile(fx, "save", SaveDialog{Name: "a.txt"}, func(s *dialogState, p string, err error) { s.path = p })
	Alert(fx, "alert", AlertDialog{Buttons: []string{"Yes", "No"}}, func(s *dialogState, b int) { s.button = b })
	if len(shown) != 3 || shown[0].Open.Title != "Pick" || shown[1].Save.Name != "a.txt" || shown[2].Alert.Buttons[1] != "No" {
		t.Fatalf("shown %+v", shown)
	}
	answers[0](DialogResult{Paths: []string{"/a", "/b"}})
	answers[0](DialogResult{Paths: []string{"/twice"}}) // a second answer is ignored
	answers[1](DialogResult{Paths: []string{"/x.txt"}})
	answers[2](DialogResult{Button: 1})
	apply(s)
	if len(s.paths) != 2 || s.paths[1] != "/b" || s.path != "/x.txt" || s.button != 1 || s.err != nil {
		t.Fatalf("%+v", s)
	}
}

func TestTermAlert(t *testing.T) {
	var d termDialogs
	var got []int
	d.show(Dialog{Alert: &AlertDialog{Title: "Discard changes?", Message: "Your edits to notes.txt will be lost.", Buttons: []string{"Discard", "Cancel"}}},
		func(r DialogResult) { got = append(got, r.Button) })
	if _, ok := d.route(KeyEvent{Key: KeyCharacter, Rune: 'x'}); ok {
		t.Fatal("a key leaked past the dialog")
	}
	if ev, ok := d.route(ResizeEvent{Width: 1, Height: 1}); !ok || ev == nil {
		t.Fatal("resize must pass")
	}
	f := d.paint(Frame{Content: strings.Repeat(strings.Repeat(".", 60)+"\n", 11) + strings.Repeat(".", 60), Cursor: &Cursor{}}, 60, 12)
	plain := ansi.Strip(f.Content)
	for _, want := range []string{"Discard changes?", "Your edits to notes.txt will be lost.", "[ Discard ]", "[ Cancel ]", "╭"} {
		if !strings.Contains(plain, want) {
			t.Errorf("alert missing %q:\n%s", want, plain)
		}
	}
	if f.Cursor != nil {
		t.Error("cursor shows over an alert")
	}
	d.route(KeyEvent{Key: KeyRight})
	d.route(KeyEvent{Key: KeyRight}) // wraps
	d.route(KeyEvent{Key: KeyTab, Mod: ModShift})
	d.route(KeyEvent{Key: KeyEnter})
	if len(got) != 1 || got[0] != 1 || d.isOpen() {
		t.Fatalf("enter on Cancel: %v open %v", got, d.isOpen())
	}
	// Esc picks Cancel; without one it dismisses (-1).
	d.show(Dialog{Alert: &AlertDialog{Buttons: []string{"OK", "cancel"}}}, func(r DialogResult) { got = append(got, r.Button) })
	d.show(Dialog{Alert: &AlertDialog{}}, func(r DialogResult) { got = append(got, r.Button) })
	d.route(KeyEvent{Key: KeyEscape})
	if !d.isOpen() || d.cur.d.Alert.buttons()[0] != "OK" {
		t.Fatal("the queued alert did not follow")
	}
	d.route(KeyEvent{Key: KeyEscape})
	if len(got) != 3 || got[1] != 1 || got[2] != -1 || d.isOpen() {
		t.Fatalf("esc: %v", got)
	}
	// A click on a button chooses it.
	d.show(Dialog{Alert: &AlertDialog{Title: "Pick", Buttons: []string{"One", "Two"}}}, func(r DialogResult) { got = append(got, r.Button) })
	d.paint(Frame{Content: strings.Repeat(" \n", 23)}, 80, 24)
	id := "dialog:button:1"
	r, ok := d.cur.comp.Rects[id]
	if !ok {
		t.Fatalf("no %s in %v", id, d.cur.comp.Rects)
	}
	d.route(MouseEvent{X: d.cur.x + r.X + 1, Y: d.cur.y + r.Y, Button: MouseLeft, Action: MousePress})
	if len(got) != 4 || got[3] != 1 {
		t.Fatalf("click: %v", got)
	}
}

// tree makes a directory: names ending in "/" are directories.
func tree(t *testing.T, names ...string) string {
	root := t.TempDir()
	for _, n := range names {
		p := filepath.Join(root, n)
		if strings.HasSuffix(n, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func names(td *termDialog) string {
	var out []string
	for _, e := range td.entries {
		n := e.name
		if e.dir {
			n += "/"
		}
		out = append(out, n)
	}
	return strings.Join(out, " ")
}

func typeText(d *termDialogs, s string) {
	for _, r := range s {
		d.route(KeyEvent{Key: KeyCharacter, Rune: r})
	}
}

func TestTermOpenFile(t *testing.T) {
	root := tree(t, "notes.txt", "Notebook.md", "readme.md", ".hidden", "src/", "src/main.go", "zz/")
	var got [][]string
	var d termDialogs
	d.show(Dialog{Open: &OpenDialog{Directory: root, Types: []string{"txt", ".md"}}}, func(r DialogResult) { got = append(got, r.Paths) })
	td := d.cur
	if td.input != root+"/" {
		t.Fatalf("input %q", td.input)
	}
	// Directories first; no hidden files; only .txt and .md files.
	if n := names(td); n != "src/ zz/ Notebook.md notes.txt readme.md" {
		t.Fatalf("listing %q", n)
	}
	typeText(&d, "no")
	if n := names(td); n != "Notebook.md notes.txt" {
		t.Fatalf("prefix (any case) %q", n)
	}
	d.route(KeyEvent{Key: KeyTab}) // Notebook / notes share "note"; "no" stays as typed
	if td.input != root+"/note" {
		t.Fatalf("tab on a shared prefix: %q", td.input)
	}
	typeText(&d, "s")
	d.route(KeyEvent{Key: KeyTab}) // unique → notes.txt
	if td.input != root+"/notes.txt" {
		t.Fatalf("tab on a unique match: %q", td.input)
	}
	d.route(KeyEvent{Key: KeyBackspace, Mod: ModAlt}) // back to the directory
	if td.input != root+"/" {
		t.Fatalf("alt+backspace: %q", td.input)
	}
	typeText(&d, ".")
	if n := names(td); n != "" {
		t.Fatalf("hidden files are not of the types: %q", n)
	}
	d.route(KeyEvent{Key: KeyBackspace})
	d.route(KeyEvent{Key: KeyDown}) // src/
	d.route(KeyEvent{Key: KeyEnter})
	if td.input != root+"/src/" || td.hi != -1 {
		t.Fatalf("enter on a directory: %q hi %d", td.input, td.hi)
	}
	if n := names(td); n != "" { // main.go is not of the types
		t.Fatalf("src listing %q", n)
	}
	d.route(KeyEvent{Key: KeyBackspace, Mod: ModAlt})
	d.route(PasteEvent{Text: "readme.md\n"})
	d.route(KeyEvent{Key: KeyEnter}) // the typed path exists: accepted
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != filepath.Join(root, "readme.md") || d.isOpen() {
		t.Fatalf("accepted %q", got)
	}
	// Esc cancels with no paths.
	d.show(Dialog{Open: &OpenDialog{Directory: root}}, func(r DialogResult) { got = append(got, r.Paths) })
	typeText(&d, "missing")
	d.route(KeyEvent{Key: KeyEnter}) // does not exist: stays open
	if !d.isOpen() {
		t.Fatal("a missing file was accepted")
	}
	d.route(KeyEvent{Key: KeyEscape})
	if len(got) != 2 || got[1] != nil {
		t.Fatalf("cancel: %q", got)
	}
}

func TestTermOpenFolderAndSave(t *testing.T) {
	root := tree(t, "a.txt", "docs/", "docs/b.txt")
	var got []string
	var d termDialogs
	d.show(Dialog{Open: &OpenDialog{Directory: root, Folders: true}}, func(r DialogResult) { got = append(got, r.Paths...) })
	if n := names(d.cur); n != "docs/" {
		t.Fatalf("folders only: %q", n)
	}
	d.route(KeyEvent{Key: KeyDown})
	d.route(KeyEvent{Key: KeyEnter}) // opens docs/
	d.route(KeyEvent{Key: KeyEnter}) // accepts it
	if len(got) != 1 || got[0] != filepath.Join(root, "docs") {
		t.Fatalf("folder: %q", got)
	}
	d.show(Dialog{Save: &SaveDialog{Directory: root, Name: "new.txt"}}, func(r DialogResult) { got = append(got, r.Paths...) })
	if d.cur.input != root+"/new.txt" {
		t.Fatalf("save input %q", d.cur.input)
	}
	d.route(KeyEvent{Key: KeyEnter})
	if len(got) != 2 || got[1] != filepath.Join(root, "new.txt") {
		t.Fatalf("save: %q", got)
	}
	// A name in a missing directory is not accepted.
	d.show(Dialog{Save: &SaveDialog{Directory: root, Name: "nope/x.txt"}}, func(r DialogResult) { got = append(got, r.Paths...) })
	d.route(KeyEvent{Key: KeyEnter})
	if !d.isOpen() || len(got) != 2 {
		t.Fatalf("missing directory accepted: %q", got)
	}
}

func TestTermFilePromptPaint(t *testing.T) {
	root := tree(t, "one.txt", "two.txt", "sub/")
	var d termDialogs
	d.show(Dialog{Open: &OpenDialog{Title: "Open a note", Directory: root}}, func(DialogResult) {})
	d.route(KeyEvent{Key: KeyDown})
	d.route(KeyEvent{Key: KeyDown})
	f := d.paint(Frame{Content: strings.Repeat(" \n", 23)}, 80, 24)
	plain := ansi.Strip(f.Content)
	for _, want := range []string{"Open a note", "sub/", "one.txt", "two.txt", "enter open"} {
		if !strings.Contains(plain, want) {
			t.Errorf("prompt missing %q:\n%s", want, plain)
		}
	}
	if f.Cursor == nil || f.Cursor.Y != d.cur.y+1 {
		t.Fatalf("cursor %+v, want on the path line", f.Cursor)
	}
	// A click on an entry highlights it; a second click opens it.
	r := d.cur.comp.Rects["dialog:entry:0"]
	click := MouseEvent{X: d.cur.x + r.X, Y: d.cur.y + r.Y, Button: MouseLeft, Action: MousePress}
	d.route(click)
	if d.cur.hi != 0 {
		t.Fatalf("click highlighted %d", d.cur.hi)
	}
	d.route(click)
	if d.cur.input != root+"/sub/" {
		t.Fatalf("second click: %q", d.cur.input)
	}
}

func TestExpandHome(t *testing.T) {
	if got := expandHome("~/x", "/home/me"); got != "/home/me/x" {
		t.Error(got)
	}
	if got := expandHome("~x", "/home/me"); got != "~x" {
		t.Error(got)
	}
	var d termDialogs
	d.home = tree(t, "inhome.txt")
	d.show(Dialog{Open: &OpenDialog{Directory: "~"}}, func(DialogResult) {})
	if n := names(d.cur); n != "inhome.txt" {
		t.Errorf("~ listing %q", n)
	}
}

// Run draws a dialog the app asks for, takes the keys while it shows, and
// folds the answer in.
func TestRunShowsDialogs(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	type state struct {
		buttons []int
		keys    []rune
	}
	s := &state{}
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), App[state]{
			State: s,
			Handle: func(s *state, ev Event, fx Effects[state]) {
				k, ok := ev.(KeyEvent)
				if !ok || k.Key != KeyCharacter {
					return
				}
				switch k.Rune {
				case 'a':
					Alert(fx, "confirm", AlertDialog{Title: "Really?", Buttons: []string{"Yes", "No"}}, func(s *state, b int) {
						s.buttons = append(s.buttons, b)
					})
				case 'q':
					if len(s.buttons) > 0 {
						fx.Quit()
					}
				}
				s.keys = append(s.keys, k.Rune)
			},
			View: func(*state, RenderContext) Frame { return Frame{Content: "ready"} },
		}, WithInput(reader), WithOutput(out))
	}()
	waitFor(t, out, "ready")
	if _, err := writer.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, out, "Really?")
	// While it shows, "q" goes to the dialog (nothing), → moves to No,
	// Enter chooses it; the next "q" reaches the app, which quits.
	if _, err := writer.Write([]byte("q\x1b[C\rq")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not quit: the answer was not applied before the next key")
	}
	if len(s.buttons) != 1 || s.buttons[0] != 1 || string(s.keys) != "aq" {
		t.Fatalf("buttons %v keys %q", s.buttons, string(s.keys))
	}
}
