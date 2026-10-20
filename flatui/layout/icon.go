package layout

// IconName is a shape a GUI draws for an Icon. Names are shapes, not
// meanings: the app decides what a chevron or a dot stands for. The set
// grows when a sample needs a new shape.
type IconName int

const (
	// IconNone draws the Text, in a GUI too.
	IconNone IconName = iota
	IconChevronRight
	// IconDisclosureClosed and IconDisclosureOpen are the filled triangles
	// of a collapsed (▸) and expanded (▾) tree row.
	IconDisclosureClosed
	IconDisclosureOpen
	// IconDot is a filled circle (●), IconCircle an outlined one (○).
	IconDot
	IconCircle
	// IconBlank draws nothing in a GUI, keeping its columns: for terminal-only
	// decoration, such as the Powerline caps of a tab.
	IconBlank
)

// Icon is a small symbol with a terminal spelling: the terminal shows Text
// in Style, and a GUI draws the named shape in Style's color within the same
// columns, so rows line up in both. Unknown names draw Text.
type Icon struct {
	NodeBase
	Name  IconName
	Text  string
	Style TextStyle
}

func (i Icon) Size() (Size, Size) { return i.label().Size() }

func (i Icon) Render(r Rect) string { return i.label().Render(r) }

func (i Icon) label() Label { return Label{NodeBase: i.NodeBase, Text: i.Text, Style: i.Style} }
