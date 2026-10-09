//go:build unix

package agent

import (
	"os/exec"
	"syscall"
)

// exitSignal returns the signal name when the process died from a signal,
// "" otherwise (unix only — Windows builds report no signal, see
// supervisor_exit_windows.go).
func exitSignal(ee *exec.ExitError) string {
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return ws.Signal().String()
	}
	return ""
}
