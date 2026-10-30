# Menu-bar utilities

Use the existing native host with `Options.Desktop` to run a background utility:

```go
shortcut := flatte.KeyEvent{
    Key: flatte.KeyCharacter, Rune: ' ',
    Mod: flatte.ModSuper | flatte.ModShift,
}
err := window.Run(ctx, app, window.Options{
    Title: "My app", Width: 80, Height: 37,
    Desktop: &flatte.Desktop{Title: "My app", Shortcut: &shortcut},
})
```

The same `Frame.Root` and flatui controls render through the native scene
renderer. No cgo, WebKit, local server, or alternate renderer is required.
Ordinary window and terminal applications are unchanged when they do not opt in.
The current desktop implementation is macOS-only. `Run` must start on the main
goroutine, as for any native window app.

The utility starts hidden, with a status item and no Dock icon. Its status menu
uses the existing `App.Menus` declaration; menu choices deliver `MenuEvent`.
Closing the main window hides it and delivers
`DesktopEvent{ID:"close", Source:"window"}`. Apps clear transient drafts in
response. Call `fx.Quit()` for process exit.

Use loop-owned `fx.Desktop(flatte.DesktopCommand{Kind: ...})` for `show`, `hide`,
`permission`, or `shortcut`. The last takes `Shortcut: &key`; nil disables the
global binding. Permission is explicitly requested; capture never silently
prompts or simulates Copy. Unsupported hosts ignore these effects.

Services and the shortcut deliver `DesktopEvent` with `ID:"capture"`, Source
`"service"` or `"shortcut"`, and selected `Text` or an actionable `Error`.
Selection is explicit, capped at 1 MiB, and never continuously monitored.
Shortcut registration errors and permission replies are also events. Handle
errors before enabling a write action, and never log or persist selected text.

Services require an application bundle with an `NSServices` Info.plist entry
whose `NSMessage` is `receiveText`, `NSUserData` is an application action ID,
and `NSSendTypes` includes `public.utf8-plain-text` and `NSStringPboardType`.
Set `LSUIElement` for background operation. The host installs its native
`NSApplication.servicesProvider`; source applications decide where the Service
is exposed. Some require enabling it in Keyboard Shortcuts > Services.
Accessibility permission is needed for shortcut selection capture, not Services.

Use a stable application install path and local signing for development.
Distribution signing and notarization remain the application's responsibility.
Services compatibility and global shortcut/permission UX need live checks in
the target source applications; direct provider tests do not prove those.

Capture events do not show or focus the window automatically. Apps can paste
in the background or explicitly request `show` for settings, errors, or a
preview. This keeps presentation and forwarding policy in the application.
