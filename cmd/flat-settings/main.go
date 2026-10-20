// Command flat-settings is a settings form built from controls — a text
// field, a switch, a checkbox, radio buttons, a slider, a dropdown, and
// buttons — driven by the keyboard (a focus ring) and the mouse, the same
// in a terminal and, with -gui, in a native window.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
)

// Focus stops, in Tab order; their node IDs are fieldIDs.
const (
	fieldName = iota
	fieldNotify
	fieldSounds
	fieldTheme
	fieldVolume
	fieldLanguage
	fieldReset
	fieldSave
	fieldCount
)

var fieldIDs = [fieldCount]string{"name", "notify", "sounds", "theme", "volume", "language", "reset", "save"}

var themes = []string{"System", "Light", "Dark"}

// Settings are the values the form edits.
type Settings struct {
	Name          string
	Notifications bool
	Sounds        bool
	Theme         int
	Volume        float64
	Language      int
}

func defaults() Settings {
	return Settings{Name: "Ada", Notifications: true, Sounds: true, Volume: 0.6}
}

type State struct {
	focus    flatui.FocusRing
	name     flatui.TextField
	language flatui.Select
	settings Settings
	saved    Settings
	status   string
	// dragging is the slider being dragged (its rect when pressed).
	dragging   bool
	volumeRect layout.Rect
}

func NewState() *State {
	s := &State{}
	s.focus.SetCount(fieldCount)
	s.language.SetOptions([]string{"English", "Français", "Deutsch", "Español", "日本語"})
	s.apply(defaults())
	s.saved = s.settings
	return s
}

// apply loads settings into the form.
func (s *State) apply(v Settings) {
	s.settings = v
	s.name.Value = v.Name
	s.name.SetCursor(len(v.Name))
	s.language.Select(v.Language)
}

// current is the form's settings now.
func (s *State) current() Settings {
	v := s.settings
	v.Name = s.name.Value
	v.Language = s.language.Selected()
	return v
}

func (s *State) dirty() bool { return s.current() != s.saved }

func (s *State) save() {
	s.saved = s.current()
	s.status = "Saved."
}

func (s *State) reset() {
	s.apply(defaults())
	s.status = "Reset to defaults."
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		handleMouse(s, e)
	case flatte.PasteEvent:
		if s.focus.Focused(fieldName) {
			for _, r := range strings.ReplaceAll(e.Text, "\n", " ") {
				s.name.Insert(r)
			}
		}
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	// An open dropdown takes the keys until it closes.
	if s.language.IsOpen() {
		switch key.Key {
		case flatte.KeyUp:
			s.language.MoveUp()
		case flatte.KeyDown:
			s.language.MoveDown()
		case flatte.KeyEnter:
			s.language.Confirm()
		case flatte.KeyEscape, flatte.KeyTab:
			s.language.Close()
		case flatte.KeyCharacter:
			if key.Rune == ' ' {
				s.language.Confirm()
			}
		}
		return
	}
	switch key.Key {
	case flatte.KeyEscape, flatte.KeyCtrlC:
		fx.Quit()
		return
	case flatte.KeyTab:
		if key.Mod.Contains(flatte.ModShift) {
			s.focus.Prev()
		} else {
			s.focus.Next()
		}
		return
	}
	if s.focus.Focused(fieldName) {
		editName(s, key)
		return
	}
	switch key.Key {
	case flatte.KeyUp:
		if s.focus.Focused(fieldTheme) {
			s.settings.Theme = (s.settings.Theme + len(themes) - 1) % len(themes)
		} else {
			s.focus.Prev()
		}
	case flatte.KeyDown:
		if s.focus.Focused(fieldTheme) {
			s.settings.Theme = (s.settings.Theme + 1) % len(themes)
		} else {
			s.focus.Next()
		}
	case flatte.KeyLeft, flatte.KeyRight:
		step := 0.05
		if key.Key == flatte.KeyLeft {
			step = -step
		}
		switch {
		case s.focus.Focused(fieldVolume):
			s.settings.Volume = min(max(s.settings.Volume+step, 0), 1)
		case s.focus.Focused(fieldTheme):
			if step < 0 {
				s.settings.Theme = (s.settings.Theme + len(themes) - 1) % len(themes)
			} else {
				s.settings.Theme = (s.settings.Theme + 1) % len(themes)
			}
		}
	case flatte.KeyEnter:
		activate(s)
	case flatte.KeyCharacter:
		if key.Rune == ' ' {
			activate(s)
		}
	}
}

// editName is the name field's editing keys.
func editName(s *State, key flatte.KeyEvent) {
	switch key.Key {
	case flatte.KeyCharacter:
		s.name.Insert(key.Rune)
	case flatte.KeyBackspace:
		s.name.Backspace()
	case flatte.KeyDelete:
		s.name.Delete()
	case flatte.KeyLeft:
		s.name.MoveLeft()
	case flatte.KeyRight:
		s.name.MoveRight()
	case flatte.KeyEnter, flatte.KeyDown:
		s.focus.Next()
	}
}

// activate is Enter or Space on the focused control.
func activate(s *State) {
	switch s.focus.Index() {
	case fieldNotify:
		s.settings.Notifications = !s.settings.Notifications
	case fieldSounds:
		if s.settings.Notifications {
			s.settings.Sounds = !s.settings.Sounds
		}
	case fieldTheme:
		s.settings.Theme = (s.settings.Theme + 1) % len(themes)
	case fieldLanguage:
		s.language.Open()
	case fieldReset:
		s.reset()
	case fieldSave:
		s.save()
	}
}

// handleMouse: a click focuses the control it hits and acts on it; a
// press on the slider sets the volume and a drag follows it.
func handleMouse(s *State, m flatte.MouseEvent) {
	if s.dragging {
		switch {
		case m.Action == flatte.MouseRelease || m.Button != flatte.MouseLeft:
			s.dragging = false
		case m.Action == flatte.MouseMotion:
			s.settings.Volume = layout.Slider{}.ValueAt(m.X-s.volumeRect.X, s.volumeRect.W)
		}
		return
	}
	if m.Button != flatte.MouseLeft || m.Action != flatte.MousePress {
		return
	}
	if s.language.Click(m.Target, fieldIDs[fieldLanguage]) {
		s.focus.Select(fieldLanguage)
		return
	}
	s.language.Close()
	if i, ok := strings.CutPrefix(m.Target, "theme:"); ok {
		for t, name := range themes {
			if strings.EqualFold(name, i) {
				s.focus.Select(fieldTheme)
				s.settings.Theme = t
			}
		}
		return
	}
	for f, id := range fieldIDs {
		if m.Target != id {
			continue
		}
		s.focus.Select(f)
		switch f {
		case fieldName:
			if m.HasCaret {
				s.name.SetCursor(m.Caret)
			}
		case fieldVolume:
			s.dragging, s.volumeRect = true, m.TargetRect
			s.settings.Volume = layout.Slider{}.ValueAt(m.X-m.TargetRect.X, m.TargetRect.W)
		default:
			activate(s)
		}
	}
}

// labelWidth is the width of the left column of field names.
const labelWidth = 15

// field is one form row: its name and its control.
func field(name string, control layout.Node) layout.Node {
	return layout.Row{Children: []layout.Node{
		layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(labelWidth)}, Text: name, Style: layout.TextStyle{Role: layout.TextSubtle}},
		control,
	}}
}

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	v, f := s.settings, s.focus.Focused
	name := s.name.Layout(fieldIDs[fieldName], f(fieldName))
	name.W = layout.Fixed(24)
	radios := layout.Row{NodeBase: layout.NodeBase{Gap: 2}}
	for i, t := range themes {
		radios.Children = append(radios.Children, layout.Toggle{
			NodeBase: layout.NodeBase{ID: "theme:" + strings.ToLower(t)},
			Kind:     layout.ToggleRadio, Label: t, Checked: v.Theme == i, Focused: f(fieldTheme) && v.Theme == i,
		})
	}
	status := s.status
	if s.dirty() {
		status = "Unsaved changes."
	}
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Settings", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{},
		field("Name", name),
		layout.Label{},
		field("Notifications", layout.Toggle{
			NodeBase: layout.NodeBase{ID: fieldIDs[fieldNotify], Tooltip: "Show a banner when something needs you"},
			Kind:     layout.ToggleSwitch, Label: onOff(v.Notifications), Checked: v.Notifications, Focused: f(fieldNotify),
		}),
		field("", layout.Toggle{
			NodeBase: layout.NodeBase{ID: fieldIDs[fieldSounds]},
			Label:    "Play sounds", Checked: v.Sounds && v.Notifications, Disabled: !v.Notifications, Focused: f(fieldSounds),
		}),
		layout.Label{},
		field("Theme", radios),
		field("Volume", layout.Row{Children: []layout.Node{
			layout.Slider{NodeBase: layout.NodeBase{ID: fieldIDs[fieldVolume], W: layout.Fixed(24)}, Value: v.Volume, Focused: f(fieldVolume)},
			layout.Label{Text: fmt.Sprintf("  %3.0f%%", v.Volume*100)},
		}}),
		field("Language", s.language.Layout(fieldIDs[fieldLanguage], f(fieldLanguage))),
		layout.NewSpacer(),
		layout.Row{NodeBase: layout.NodeBase{Gap: 2}, Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(1)}, Text: status, Style: layout.TextStyle{Role: layout.TextSubtle}},
			layout.Button{NodeBase: layout.NodeBase{ID: fieldIDs[fieldReset], Tooltip: "Restore the defaults"}, Label: "Reset", Focused: f(fieldReset)},
			layout.Button{NodeBase: layout.NodeBase{ID: fieldIDs[fieldSave], Tooltip: "Keep these settings"}, Label: "Save", Default: true, Disabled: !s.dirty(), Focused: f(fieldSave)},
		}},
		layout.Label{},
		layout.Label{Text: "tab focus  space/enter change  ←/→ adjust  esc quit", Style: layout.TextStyle{Role: layout.TextSubtle}},
	)}
}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	app := flatte.App[State]{State: NewState(), Handle: Handle, View: View}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Settings", Width: 64, Height: 22})
	} else {
		err = flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion), flatte.WithoutDefaultQuit())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
