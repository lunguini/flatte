package flatte

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// termDialogs is the terminal runtime's dialogs: drawn over the frame and
// modal while one shows (host presentation, like MenuHost). Dialogs asked
// for while one shows wait their turn.
type termDialogs struct {
	cur   *termDialog
	queue []*termDialog
	// home is the user's home directory, for "~" (tests set it).
	home string
}

type termDialog struct {
	d    Dialog
	done func(DialogResult)

	// An alert: the focused button.
	focus int

	// A file prompt: the typed path, the listing for it, the highlighted
	// entry (-1: none), and the first listed row shown.
	input   string
	entries []dirEntry
	hi, top int

	// Where it was last painted, for the mouse.
	x, y int
	comp layout.Composition
}

type dirEntry struct {
	name string
	dir  bool
}

// show opens d, or queues it behind the one showing.
func (t *termDialogs) show(d Dialog, done func(DialogResult)) {
	td := &termDialog{d: d, done: done, hi: -1}
	switch {
	case d.Open != nil:
		td.input = t.startDir(d.Open.Directory)
	case d.Save != nil:
		td.input = t.startDir(d.Save.Directory) + d.Save.Name
	}
	td.list(t.home)
	if t.cur == nil {
		t.cur = td
		return
	}
	t.queue = append(t.queue, td)
}

// startDir is dir (the working directory when "") with a trailing slash.
func (t *termDialogs) startDir(dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	return dir
}

func (t *termDialogs) isOpen() bool { return t.cur != nil }

// finish answers the dialog showing and shows the next one.
func (t *termDialogs) finish(r DialogResult) {
	done := t.cur.done
	t.cur = nil
	if len(t.queue) > 0 {
		t.cur, t.queue = t.queue[0], t.queue[1:]
	}
	done(r)
}

// route takes an event while a dialog shows: the dialog's keys, pastes, and
// clicks go to it; resizes pass; everything else is dropped (it is modal).
func (t *termDialogs) route(ev Event) (Event, bool) {
	if _, resize := ev.(ResizeEvent); resize {
		return ev, true
	}
	if t.cur.d.Alert != nil {
		t.routeAlert(ev)
	} else {
		t.routeFile(ev)
	}
	return nil, false
}

func (t *termDialogs) routeAlert(ev Event) {
	td := t.cur
	buttons := td.d.Alert.buttons()
	switch e := ev.(type) {
	case KeyEvent:
		switch {
		case e.Key == KeyLeft || e.Key == KeyTab && e.Mod.Contains(ModShift):
			td.focus = (td.focus + len(buttons) - 1) % len(buttons)
		case e.Key == KeyRight || e.Key == KeyTab:
			td.focus = (td.focus + 1) % len(buttons)
		case e.Key == KeyEnter || e.Key == KeyCharacter && e.Rune == ' ' && e.Mod == 0:
			t.finish(DialogResult{Button: td.focus})
		case e.Key == KeyEscape || e.Key == KeyCtrlC:
			cancel := slices.IndexFunc(buttons, func(b string) bool { return strings.EqualFold(b, "Cancel") })
			t.finish(DialogResult{Button: cancel})
		}
	case MouseEvent:
		if e.Button != MouseLeft || e.Action != MousePress {
			return
		}
		id, _ := td.comp.HitTest(e.X-td.x, e.Y-td.y)
		if n, ok := strings.CutPrefix(id, "dialog:button:"); ok {
			if i, err := strconv.Atoi(n); err == nil {
				t.finish(DialogResult{Button: i})
			}
		}
	}
}

func (t *termDialogs) routeFile(ev Event) {
	td := t.cur
	switch e := ev.(type) {
	case PasteEvent:
		td.edit(td.input + strings.NewReplacer("\n", "", "\r", "").Replace(e.Text))
	case KeyEvent:
		switch {
		case e.Key == KeyEscape || e.Key == KeyCtrlC:
			t.finish(DialogResult{})
		case e.Key == KeyUp:
			td.move(-1)
		case e.Key == KeyDown:
			td.move(1)
		case e.Key == KeyTab:
			td.complete()
		case e.Key == KeyEnter:
			t.enter()
		case e.Key == KeyBackspace && e.Mod&(ModAlt|ModCtrl) != 0,
			e.Key == KeyCharacter && e.Mod == ModCtrl && (e.Rune == 'w' || e.Rune == 'h'):
			td.edit(trimSegment(td.input))
		case e.Key == KeyBackspace:
			_, n := utf8.DecodeLastRuneInString(td.input)
			td.edit(td.input[:len(td.input)-n])
		case e.Key == KeyCharacter && e.Mod&(ModCtrl|ModAlt|ModSuper) == 0:
			td.edit(td.input + string(e.Rune))
		}
	case MouseEvent:
		if e.Button != MouseLeft || e.Action != MousePress {
			return
		}
		id, _ := td.comp.HitTest(e.X-td.x, e.Y-td.y)
		if n, ok := strings.CutPrefix(id, "dialog:entry:"); ok {
			if i, err := strconv.Atoi(n); err == nil {
				if i == td.hi {
					t.enter() // a click on the highlighted entry opens it
				} else {
					td.hi = i
				}
			}
		}
	}
	td.list(t.home)
}

// edit replaces the typed path and drops the highlight.
func (td *termDialog) edit(input string) {
	td.input, td.hi, td.top = input, -1, 0
}

// trimSegment drops the last path segment (and the separator before it).
func trimSegment(p string) string {
	p = strings.TrimSuffix(p, string(filepath.Separator))
	if i := strings.LastIndexByte(p, filepath.Separator); i >= 0 {
		return p[:i+1]
	}
	return ""
}

func (td *termDialog) move(dir int) {
	if len(td.entries) == 0 {
		return
	}
	switch {
	case td.hi < 0 && dir > 0:
		td.hi = 0
	case td.hi < 0:
		td.hi = len(td.entries) - 1
	default:
		td.hi = min(max(td.hi+dir, 0), len(td.entries)-1)
	}
}

// split is the typed path's directory (with its separator) and the prefix
// typed after it.
func (td *termDialog) split() (dir, prefix string) {
	i := strings.LastIndexByte(td.input, filepath.Separator)
	return td.input[:i+1], td.input[i+1:]
}

// pathOf is entry e of the listing as a path to type: a directory ends in
// a separator.
func (td *termDialog) pathOf(e dirEntry) string {
	dir, _ := td.split()
	if e.dir {
		return dir + e.name + string(filepath.Separator)
	}
	return dir + e.name
}

// complete takes the highlighted entry, or completes the typed name as far
// as the listing agrees.
func (td *termDialog) complete() {
	if td.hi >= 0 && td.hi < len(td.entries) {
		td.edit(td.pathOf(td.entries[td.hi]))
		return
	}
	if len(td.entries) == 0 {
		return
	}
	if len(td.entries) == 1 {
		td.edit(td.pathOf(td.entries[0]))
		return
	}
	common := td.entries[0].name
	for _, e := range td.entries[1:] {
		for !strings.HasPrefix(strings.ToLower(e.name), strings.ToLower(common)) {
			_, n := utf8.DecodeLastRuneInString(common)
			common = common[:len(common)-n]
		}
	}
	// Keep what was typed as typed; add the rest of the shared prefix.
	dir, prefix := td.split()
	if len(common) > len(prefix) && strings.EqualFold(common[:len(prefix)], prefix) {
		td.edit(dir + prefix + common[len(prefix):])
	}
}

// enter opens the highlighted directory or accepts the highlighted file;
// with none highlighted it acts on the typed path: a directory opens (or,
// when the dialog asks for folders, is accepted), an existing file is
// accepted, and a save accepts a new name in an existing directory.
func (t *termDialogs) enter() {
	td := t.cur
	if td.hi >= 0 && td.hi < len(td.entries) {
		e := td.entries[td.hi]
		if e.dir {
			td.edit(td.pathOf(e))
			return
		}
		t.finish(DialogResult{Paths: []string{td.pathOf(e)}})
		return
	}
	path := expandHome(td.input, t.home)
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		if td.d.Open != nil && td.d.Open.Folders {
			t.finish(DialogResult{Paths: []string{filepath.Clean(path)}})
			return
		}
		if !strings.HasSuffix(td.input, string(filepath.Separator)) {
			td.edit(td.input + string(filepath.Separator))
		}
	case err == nil && td.d.Open != nil && !td.d.Open.Folders:
		t.finish(DialogResult{Paths: []string{filepath.Clean(path)}})
	case err == nil && td.d.Save != nil:
		t.finish(DialogResult{Paths: []string{filepath.Clean(path)}})
	case os.IsNotExist(err) && td.d.Save != nil && filepath.Base(path) != "" && !strings.HasSuffix(path, string(filepath.Separator)):
		if parent, err := os.Stat(filepath.Dir(path)); err == nil && parent.IsDir() {
			t.finish(DialogResult{Paths: []string{filepath.Clean(path)}})
		}
	}
}

// expandHome replaces a leading "~" with home.
func expandHome(p, home string) string {
	if home != "" && (p == "~" || strings.HasPrefix(p, "~"+string(filepath.Separator))) {
		return home + p[1:]
	}
	return p
}

// list reads the listing for the typed path: the entries of its directory
// whose names start with the typed prefix (any case) — directories first;
// hidden ones only for a "." prefix; files only of the dialog's Types, and
// none when it asks for folders.
func (td *termDialog) list(home string) {
	if td.d.Alert != nil {
		return
	}
	dir, prefix := td.split()
	read := dir
	if read == "" {
		read = "."
	}
	des, _ := os.ReadDir(expandHome(read, home))
	var types []string
	folders := false
	switch {
	case td.d.Open != nil:
		types, folders = td.d.Open.Types, td.d.Open.Folders
	case td.d.Save != nil:
		types = td.d.Save.Types
	}
	td.entries = td.entries[:0]
	for _, de := range des {
		name := de.Name()
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		isDir := de.IsDir()
		if !isDir && de.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(expandHome(read, home), name)); err == nil {
				isDir = info.IsDir()
			}
		}
		if !isDir && (folders || len(types) > 0 && !slices.ContainsFunc(types, func(t string) bool {
			return strings.EqualFold(strings.TrimPrefix(filepath.Ext(name), "."), strings.TrimPrefix(t, "."))
		})) {
			continue
		}
		td.entries = append(td.entries, dirEntry{name: name, dir: isDir})
	}
	slices.SortStableFunc(td.entries, func(a, b dirEntry) int {
		if a.dir != b.dir {
			if a.dir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
	td.hi = min(td.hi, len(td.entries)-1)
}

var (
	dialogSubtle = layout.TextStyle{Role: layout.TextSubtle}
	dialogHi     = layout.TextStyle{Fg: lipgloss.Color("255"), Bg: lipgloss.Color("24")}
)

// paint draws the dialog showing centered over frame (width×height cells).
// A file prompt shows the cursor at the end of its path; an alert hides it.
func (t *termDialogs) paint(frame Frame, width, height int) Frame {
	td := t.cur
	if td == nil {
		return frame
	}
	var node layout.Node
	var w, h int
	if td.d.Alert != nil {
		node, w, h = td.alertNode(width)
	} else {
		node, w, h = td.fileNode(width, height)
	}
	td.comp = layout.Compose(node, w, h)
	td.x, td.y = max((width-w)/2, 0), max((height-h)/2, 0)
	frame.Content = layout.OverlayAt(frame.Content, td.comp.Frame, td.x, td.y)
	frame.Cursor = nil
	if x, y, ok := td.comp.Cursor(); ok {
		frame.Cursor = &Cursor{X: td.x + x, Y: td.y + y}
	}
	return frame
}

func (td *termDialog) alertNode(width int) (layout.Node, int, int) {
	a := td.d.Alert
	buttons := a.buttons()
	row := layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: []layout.Node{layout.NewSpacer()}}
	bw := 0
	for i, b := range buttons {
		row.Children = append(row.Children, layout.Button{
			NodeBase: layout.NodeBase{ID: "dialog:button:" + strconv.Itoa(i)},
			Label:    b, Default: i == 0, Focused: i == td.focus,
		})
		bw += ansi.StringWidth(b) + 4 + 2
	}
	w := min(max(44, bw+2, ansi.StringWidth(a.Title)+6), max(width-4, 10))
	inner := w - 4
	msg := 0
	if a.Message != "" {
		msg = strings.Count(ansi.Wrap(a.Message, inner, ""), "\n") + 2
	}
	children := []layout.Node{layout.Label{Text: a.Title, Style: layout.TextStyle{Bold: true}}}
	if a.Message != "" {
		children = append(children, layout.Label{}, layout.Label{Text: a.Message, Wrap: true})
	}
	children = append(children, layout.Label{}, row)
	return layout.Col{
		NodeBase: layout.NodeBase{Box: layout.Box{Border: layout.BorderRounded}, PadLeft: 1, PadRight: 1},
		Children: children,
	}, w, msg + 5
}

func (td *termDialog) fileNode(width, height int) (layout.Node, int, int) {
	title := "Open"
	switch {
	case td.d.Open != nil && td.d.Open.Title != "":
		title = td.d.Open.Title
	case td.d.Save != nil:
		title = "Save"
		if td.d.Save.Title != "" {
			title = td.d.Save.Title
		}
	}
	w := min(72, max(width-4, 20))
	rows := min(10, max(height-7, 1))
	// Keep the highlight in view.
	if td.hi >= 0 {
		td.top = min(max(td.top, td.hi-rows+1), td.hi)
	}
	td.top = min(td.top, max(len(td.entries)-rows, 0))
	children := []layout.Node{
		layout.Input{NodeBase: layout.NodeBase{ID: "dialog:path"}, Value: td.input, Caret: len(td.input), Focused: true},
		layout.Label{Text: strings.Repeat("─", w-4), Style: dialogSubtle},
	}
	for i := td.top; i < td.top+rows; i++ {
		if i >= len(td.entries) {
			children = append(children, layout.Label{})
			continue
		}
		e := td.entries[i]
		name := e.name
		if e.dir {
			name += string(filepath.Separator)
		}
		st := layout.TextStyle{}
		if e.dir {
			st.Bold = true
		}
		if i == td.hi {
			st = dialogHi
		}
		children = append(children, layout.Label{NodeBase: layout.NodeBase{ID: "dialog:entry:" + strconv.Itoa(i)}, Text: name, Style: st})
	}
	verb := "open"
	if td.d.Save != nil {
		verb = "save"
	}
	children = append(children, layout.Label{Text: "↑/↓ choose  tab complete  enter " + verb + "  esc cancel", Style: dialogSubtle})
	return layout.Col{
		NodeBase: layout.NodeBase{Box: layout.Box{Border: layout.BorderRounded, Title: title}, PadLeft: 1, PadRight: 1},
		Children: children,
	}, w, rows + 5
}
