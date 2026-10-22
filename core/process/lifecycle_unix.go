//go:build unix

package process

import (
	"os/exec"
	"syscall"
)

func exitSignal(exitError *exec.ExitError) string {
	if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok && waitStatus.Signaled() {
		return waitStatus.Signal().String()
	}
	return ""
}
