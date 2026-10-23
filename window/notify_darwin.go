package window

import (
	"context"
	"os/exec"
	"time"

	"github.com/lunguini/flatte"
)

// osNotifier posts notifications through the system's notification
// centre. The user-notifications framework serves only app bundles (it
// aborts a bare binary), so this asks osascript, which any process may: the
// notification shows Script Editor's icon. Title and body travel as
// arguments, never as script text.
type osNotifier struct{}

// notifyTimeout bounds one osascript run.
const notifyTimeout = 10 * time.Second

func (osNotifier) notify(n flatte.Notification) {
	if n.Title == "" && n.Body == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		// Best effort, like a terminal's: nothing to tell the app.
		_ = exec.CommandContext(ctx, "osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			n.Title, n.Body).Run()
	}()
}
