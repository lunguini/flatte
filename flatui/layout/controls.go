package layout

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Controls are the interactive leaves: Button, Toggle, and Slider. Each has
// a terminal spelling and a native rendering in a GUI, where the host also
// shows hover and press itself (no app state). They hold no interaction
// state of their own: give a control an ID, and clicks on it arrive as
// MouseEvents targeting that ID; keys are the app's (Enter or Space on the
// control it has focused). Focused draws the focus look; Disabled draws a
// dimmed one — the app ignores a disabled control's events.

// Button is a push button. The terminal draws "[ Label ]" in Style — bold
// when Default (the primary action), the label reversed when Focused,
// subtle when Disabled; a GUI draws a bezel, filled with the accent when
// Default. On Auto axes it is its spelling wide and one line tall.
//
// With an Icon, a GUI draws the icon's shape centered in the button instead
// of the Label, which stays its terminal spelling (as an Icon's Text): give
// it Points sizes for a square, which a Default button draws as a circle.
// Plain draws no bezel until the pointer is over it (a toolbar button), and
// in the terminal spaces instead of the brackets. An icon button's terminal
// brackets are tight ("[▶]"), so it fits a square of its own.
type Button struct {
	NodeBase
	Label                      string
	Style                      TextStyle
	Default, Disabled, Focused bool
	Icon                       IconName
	Plain                      bool
}

// ToggleKind is how a Toggle looks: they differ in look only.
type ToggleKind int

const (
	// ToggleCheckbox is "[x] Label" / "[ ] Label".
	ToggleCheckbox ToggleKind = iota
	// ToggleSwitch is "[ ●] Label" (on) / "[○ ] Label" (off).
	ToggleSwitch
	// ToggleRadio is "(•) Label" / "( ) Label"; one-of-many is the app's.
	ToggleRadio
)

// Toggle is an on/off control: a checkbox, a switch, or a radio button,
// with its label. The terminal draws the indicator then the label in
// Style (the indicator reversed when Focused, all subtle when Disabled); a
// GUI draws a native indicator. On Auto axes it is its spelling wide and
// one line tall.
type Toggle struct {
	NodeBase
	Kind                       ToggleKind
	Label                      string
	Style                      TextStyle
	Checked, Disabled, Focused bool
}

// Slider is a value from 0 to 1 along a track: the terminal draws ━ up to a
// ● knob and ─ after it, across its width (the knob reversed when Focused);
// a GUI draws a native slider. Fill colors the filled part (nil: the
// default). On Auto axes it fills the width and is one line tall. Map a
// click or drag to a value with ValueAt.
type Slider struct {
	NodeBase
	Value             float64
	Fill              color.Color
	Disabled, Focused bool
}

// ValueAt is the value a press or drag at column x of a slider width
// columns wide asks for (x and width as in its rect, so x is node-local):
// the first column is 0, the last 1.
func (s Slider) ValueAt(x, width int) float64 {
	if width <= 1 {
		return 0
	}
	return min(max(float64(x)/float64(width-1), 0), 1)
}

// value is Value clamped to 0..1, with NaN as 0.
func (s Slider) value() float64 {
	if math.IsNaN(s.Value) {
		return 0
	}
	return min(max(s.Value, 0), 1)
}

func (b Button) spelling() string {
	open, closing := b.ends()
	return open + b.Label + closing
}

// ends are the terminal's brackets around a button's label: "[ " and " ]",
// tight for an icon button ("[▶]"), and spaces for a Plain one.
func (b Button) ends() (open, closing string) {
	switch {
	case b.Plain:
		return " ", " "
	case b.Icon != IconNone:
		return "[", "]"
	}
	return "[ ", " ]"
}

func (t Toggle) indicator() string {
	switch t.Kind {
	case ToggleSwitch:
		if t.Checked {
			return "[ ●]"
		}
		return "[○ ]"
	case ToggleRadio:
		if t.Checked {
			return "(•)"
		}
		return "( )"
	}
	if t.Checked {
		return "[x]"
	}
	return "[ ]"
}

func (t Toggle) spelling() string {
	if t.Label == "" {
		return t.indicator()
	}
	return t.indicator() + " " + t.Label
}

// lineSize is a one-line leaf text cells wide.
func lineSize(nb NodeBase, text string) (Size, Size) {
	w, h := nb.Size()
	ins := nb.insetSides()
	if w.Kind == SizeAuto {
		w = Size{Kind: SizeContent, Value: satAdd(ansi.StringWidth(text), ins.horiz())}
	}
	if h.Kind == SizeAuto {
		h = Size{Kind: SizeContent, Value: satAdd(1, ins.vert())}
	}
	return w, h
}

func (b Button) Size() (Size, Size) { return lineSize(b.NodeBase, b.spelling()) }
func (t Toggle) Size() (Size, Size) { return lineSize(t.NodeBase, t.spelling()) }

func (s Slider) Size() (Size, Size) {
	w, h := lineSize(s.NodeBase, "")
	if sw, _ := s.NodeBase.Size(); sw.Kind == SizeAuto {
		w = Grow(1)
	}
	return w, h
}

// controlStyle is a control's terminal style: subtle when disabled.
func controlStyle(s TextStyle, disabled bool) lipgloss.Style {
	if disabled {
		s.Role, s.Fg = TextSubtle, nil
	}
	return s.lipgloss()
}

func (b Button) Render(r Rect) string {
	st := b.Style
	st.Bold = st.Bold || b.Default
	base := controlStyle(st, b.Disabled)
	label := base.Render(b.Label)
	if b.Focused && !b.Disabled {
		label = base.Reverse(true).Render(b.Label)
	}
	open, closing := b.ends()
	return fillBox(base.Render(open)+label+base.Render(closing), r, b.padSides(), b.box())
}

func (t Toggle) Render(r Rect) string {
	base := controlStyle(t.Style, t.Disabled)
	ind := base.Render(t.indicator())
	if t.Focused && !t.Disabled {
		ind = base.Reverse(true).Render(t.indicator())
	}
	text := ind
	if t.Label != "" {
		text += base.Render(" " + t.Label)
	}
	return fillBox(text, r, t.padSides(), t.box())
}

func (s Slider) Render(r Rect) string {
	inner := r.insetBy(s.insetSides())
	if inner.W <= 0 {
		return fillBox("", r, s.padSides(), s.box())
	}
	knob := int(math.Round(s.value() * float64(inner.W-1)))
	fill := s.Fill
	if s.Disabled {
		fill = nil
	}
	knobStyle := lipgloss.NewStyle()
	if fill != nil {
		knobStyle = knobStyle.Foreground(fill)
	}
	if s.Focused && !s.Disabled {
		knobStyle = knobStyle.Reverse(true)
	}
	track := TextStyle{Role: TextSubtle}.lipgloss()
	bar := tinted(strings.Repeat("━", knob), fill) + knobStyle.Render("●") + track.Render(strings.Repeat("─", inner.W-1-knob))
	return fillBox(bar, r, s.padSides(), s.box())
}
