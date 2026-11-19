package flatte

// Appearance is a host's background: light or dark (see
// RenderContext.Appearance).
type Appearance int

const (
	// AppearanceUnknown: the host does not say (terminals).
	AppearanceUnknown Appearance = iota
	// AppearanceLight is dark text on a light background.
	AppearanceLight
	// AppearanceDark is light text on a dark background.
	AppearanceDark
)

// Dark reports whether a is AppearanceDark, treating AppearanceUnknown as
// dark — the common terminal default — so callers have one branch.
func (a Appearance) Dark() bool { return a != AppearanceLight }
