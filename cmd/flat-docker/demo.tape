# flat-docker demo — recorded with Charm VHS (https://github.com/charmbracelet/vhs)
#
# Regenerate from the repo root:
#   vhs cmd/flat-docker/demo.tape
#
# flat-docker runs on a simulated Docker session (no daemon needed), so the
# recording is reproducible anywhere.

Output docs/demo.gif

Require go

Set Shell "bash"
Set FontSize 16
Set Width 1240
Set Height 760
Set Padding 24
Set Theme "Catppuccin Mocha"
Set Framerate 24
Set TypingSpeed 55ms

# --- boot (hide the compile; wait for the first frame deterministically) ---
Hide
Type "cd cmd && go run ./flat-docker"  Enter
Wait+Screen@30s /containers/
Show
Sleep 1.5s

# discover the command vocabulary via the : palette
Type ":help"   Sleep 400ms  Enter  Sleep 1.8s

# browse the container list — the detail pane (stats) tracks the selection
Type "j"  Sleep 450ms
Type "j"  Sleep 450ms
Type "j"  Sleep 650ms

# jump into the detail pane and open the live logs tab
Tab             Sleep 600ms
Type "l"        Sleep 1.4s

# stream a burst of log lines into the selected container
Type ":stream"  Sleep 400ms  Enter  Sleep 2.2s

# filter the list straight from the palette
Type ":filter nginx"  Sleep 400ms  Enter  Sleep 1.8s

# back to the list, then stop a container through the confirm modal
Tab  Tab        Sleep 500ms
Type "s"        Sleep 1.4s
Type "y"        Sleep 1.6s

# hop across the top-level tabs
Type "2"  Sleep 1.1s
Type "3"  Sleep 1.1s
Type "1"  Sleep 1.2s

Sleep 1.2s
Type "q"
Sleep 800ms
