//go:build windows

package process

import "os/exec"

func exitSignal(_ *exec.ExitError) string { return "" }
