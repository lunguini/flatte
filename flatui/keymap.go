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
