//go:build unix

package flatcore

import (
	"os"
	"os/signal"
	"syscall"
)

// suspendProcess sends SIGTSTP to the whole process group (job-control
// suspend, exactly what the shell's Ctrl-Z would do) and blocks until a
// SIGCONT resumes us.
func suspendProcess() {
	cont := make(chan os.Signal, 1)
	signal.Notify(cont, syscall.SIGCONT)
	defer signal.Stop(cont)
	_ = syscall.Kill(0, syscall.SIGTSTP)
	<-cont
}
