package flatte

// Desktop configures an optional background status-menu utility in a window
// host. The existing app state, semantic Frame, and Menu declarations are
// shared with ordinary windows and terminal apps. Unsupported hosts may
// reject this option; importing flatte alone adds no native dependencies.
type Desktop struct {
	Title string
	// Shortcut captures selected text only when explicitly invoked. Nil
	// disables global activation. Hosts report registration failure as a
	// DesktopEvent with Source "shortcut" and a nonempty Error.
	Shortcut *KeyEvent
}

// DesktopEvent reports a native utility action. Source is service, shortcut,
// window, or host. Capture delivers selected Text or an Error; it never polls
// application contents. ID is "capture", "close", "permission", or a host
// failure identifier. Status-menu choices use the existing MenuEvent.
type DesktopEvent struct{ ID, Text, Error, Source string }

func (DesktopEvent) isEvent() {}

// DesktopCommand requests show, hide, shortcut, or permission from a desktop
// utility host. Shortcut replaces global activation (nil disables it).
// Permission explicitly requests selection-capture permission; capture never
// prompts silently. Other hosts ignore these commands.
type DesktopCommand struct {
	Kind     string
	Shortcut *KeyEvent
}

// withHostDesktop routes Desktop effects to a host. Like withHostClipboard,
// it is host plumbing; applications use Effects.Desktop.
func withHostDesktop[S any](fx Effects[S], command func(DesktopCommand)) Effects[S] {
	fx.desktop = command
	return fx
}

// Desktop requests a native utility operation. Like Quit it is loop-only;
// workers send a named update whose fold invokes it. Safe on zero Effects.
func (fx Effects[S]) Desktop(command DesktopCommand) {
	if fx.desktop != nil {
		if command.Shortcut != nil {
			key := *command.Shortcut
			command.Shortcut = &key
		}
		fx.desktop(command)
	}
}
