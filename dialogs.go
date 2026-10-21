package flatte

import "errors"

// ErrNoDialogs is the error OpenFile and SaveFile answer with on a host
// that cannot show dialogs (a zero Effects value, or a harness without
// them).
var ErrNoDialogs = errors.New("flatte: this host shows no dialogs")

// OpenDialog asks for files (or folders) to open. Directory is where it
// starts ("" the working directory); Types limits files to these extensions
// (without dots; nil: any); Multiple allows several; Folders asks for
// folders instead of files.
type OpenDialog struct {
	Title     string
	Directory string
	Types     []string
	Multiple  bool
	Folders   bool
}

// SaveDialog asks where to save. Directory is where it starts; Name is the
// proposed file name; Types limits the extensions offered.
type SaveDialog struct {
	Title     string
	Directory string
	Name      string
	Types     []string
}

// AlertDialog is a message with buttons (nil: one "OK"). The first button
// is the default (Enter); Esc chooses a button labelled "Cancel".
type AlertDialog struct {
	Title   string
	Message string
	Buttons []string
}

// buttons is the alert's buttons, "OK" when it has none.
func (a AlertDialog) buttons() []string {
	if len(a.Buttons) == 0 {
		return []string{"OK"}
	}
	return a.Buttons
}

// Dialog is one dialog request as a host sees it: exactly one of Open,
// Save, and Alert is set. Host plumbing, not application API.
type Dialog struct {
	Open  *OpenDialog
	Save  *SaveDialog
	Alert *AlertDialog
}

// DialogResult is a host's answer to a Dialog: the chosen paths (none when
// cancelled) for Open and Save, the chosen button for Alert (-1 when
// dismissed). Host plumbing, not application API.
type DialogResult struct {
	Paths  []string
	Button int
}

// WithHostDialogs returns fx with dialogs shown by show: a host calls done
// once, on the loop goroutine, with the user's answer. Like
// WithHostClipboard it is host plumbing, not application API.
func WithHostDialogs[S any](fx Effects[S], show func(d Dialog, done func(DialogResult))) Effects[S] {
	fx.dialogs = show
	return fx
}

// OpenFile shows an open dialog and folds the chosen paths into state as
// one update named name: none when the user cancelled, with ErrNoDialogs
// on a host without dialogs. A window shows the system panel; a terminal
// draws a path prompt with a directory listing (one path, even when
// Multiple). Call it from Init, Handle, or a fold.
func OpenFile[S any](fx Effects[S], name string, d OpenDialog, fold func(s *S, paths []string, err error)) {
	showDialog(fx, name, Dialog{Open: &d}, func(s *S, r DialogResult, err error) { fold(s, r.Paths, err) })
}

// SaveFile shows a save dialog and folds the chosen path into state as one
// update named name: "" when the user cancelled, with ErrNoDialogs on a
// host without dialogs. A window asks before replacing a file; a terminal
// does not (yet). Call it from Init, Handle, or a fold.
func SaveFile[S any](fx Effects[S], name string, d SaveDialog, fold func(s *S, path string, err error)) {
	showDialog(fx, name, Dialog{Save: &d}, func(s *S, r DialogResult, err error) {
		path := ""
		if len(r.Paths) > 0 {
			path = r.Paths[0]
		}
		fold(s, path, err)
	})
}

// Alert shows a message with buttons and folds the chosen button's index
// into state as one update named name: -1 when it was dismissed without a
// choice, or on a host without dialogs. Call it from Init, Handle, or a
// fold.
func Alert[S any](fx Effects[S], name string, d AlertDialog, fold func(s *S, button int)) {
	showDialog(fx, name, Dialog{Alert: &d}, func(s *S, r DialogResult, err error) {
		if err != nil {
			r.Button = -1
		}
		fold(s, r.Button)
	})
}

func showDialog[S any](fx Effects[S], name string, d Dialog, fold func(*S, DialogResult, error)) {
	deliver := func(r DialogResult, err error) {
		update := Named(name, func(s *S) { fold(s, r, err) })
		if !fx.trySend(update) {
			// A full update queue: spill instead of blocking the loop on
			// its own channel (as Exec does).
			ctx := fx.Ctx()
			go func() { fx.send(ctx, update) }()
		}
	}
	if fx.dialogs == nil {
		deliver(DialogResult{Button: -1}, ErrNoDialogs)
		return
	}
	done := false
	fx.dialogs(d, func(r DialogResult) {
		if done {
			return // hosts answer once; ignore a second answer
		}
		done = true
		deliver(r, nil)
	})
}
