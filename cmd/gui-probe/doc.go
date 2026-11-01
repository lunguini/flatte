// Command gui-probe is a feasibility probe for a native GUI render mode. It
// checks, on macOS and with CGO_ENABLED=0, that a Go program can:
//
//   - open an AppKit window and subclass NSView through purego/objc;
//   - draw text with CoreText into a CoreGraphics context;
//   - receive keyboard input and input-method composition through
//     NSTextInputClient, whose methods pass and return C structs by value.
//
// It is not a Flatte sample and exposes no API. Run it from the cmd module:
//
//	CGO_ENABLED=0 go run ./gui-probe            # interactive window
//	CGO_ENABLED=0 go run ./gui-probe -v         # also log text-input callbacks
//	CGO_ENABLED=0 go run ./gui-probe -snapshot frame.png
//
// On other platforms the command exits with an error.
package main
