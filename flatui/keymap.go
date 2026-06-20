package flatui

import "strings"

// KeyBinding is display metadata for a command binding. It is not input policy:
// apps still decide how terminal events map to behavior.
type KeyBinding struct {
	Keys     []string
	Help     string
	Disabled bool
}

// KeyMap is a small help-line renderer for key binding metadata.
type KeyMap []KeyBinding

// View renders enabled bindings as "keys help" pairs separated by two spaces.
func (m KeyMap) View() string {
	parts := make([]string, 0, len(m))
	for _, b := range m {
		if b.Disabled || len(b.Keys) == 0 || b.Help == "" {
			continue
		}
		parts = append(parts, strings.Join(b.Keys, "/")+" "+b.Help)
	}
	return strings.Join(parts, "  ")
}

// KeyMapMode selects how grouped key bindings are rendered.
type KeyMapMode int

const (
	// KeyMapShort renders the first enabled binding per group.
	KeyMapShort KeyMapMode = iota
	// KeyMapFull renders every enabled binding per group.
	KeyMapFull
)

// KeyMapOptions configures grouped key map rendering.
type KeyMapOptions struct {
	Width int
	Mode  KeyMapMode
}

// KeyGroup is a named group of command binding metadata.
type KeyGroup struct {
	Title    string
	Bindings KeyMap
}

// KeyGroups renders context-sensitive help across named groups.
type KeyGroups []KeyGroup

// ViewWithOptions renders grouped key metadata. It owns no key matching or
// input policy; apps still decide how events map to commands.
func (groups KeyGroups) ViewWithOptions(opts KeyMapOptions) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		bindings := groupBindings(group.Bindings, opts.Mode)
		if len(bindings) == 0 {
			continue
		}
		text := bindings.View()
		if group.Title != "" {
			text = group.Title + ": " + text
		}
		parts = append(parts, text)
	}
	return wrapKeyMapParts(parts, opts.Width)
}

func groupBindings(bindings KeyMap, mode KeyMapMode) KeyMap {
	out := make(KeyMap, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Disabled || len(binding.Keys) == 0 || binding.Help == "" {
			continue
		}
		out = append(out, binding)
		if mode == KeyMapShort {
			break
		}
	}
	return out
}

func wrapKeyMapParts(parts []string, width int) string {
	if width <= 0 {
		return strings.Join(parts, "  ")
	}
	var lines []string
	current := ""
	for _, part := range parts {
		for _, segment := range strings.Split(part, "  ") {
			if segment == "" {
				continue
			}
			next := segment
			if current != "" {
				next = current + "  " + segment
			}
			if current != "" && len(next) > width {
				lines = append(lines, current)
				current = segment
				continue
			}
			current = next
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}
