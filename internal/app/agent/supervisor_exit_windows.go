//go:build windows

package agent

import "os/exec"

// exitSignal is a no-op on Windows: syscall.WaitStatus carries no signal
// information there, so the oom_suspect heuristic stays unix-only (honest
// boundary, documented in the supervisor design §3.6).
func exitSignal(_ *exec.ExitError) string { return "" }
