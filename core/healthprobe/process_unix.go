//go:build unix

package healthprobe

import (
	"context"
	"fmt"
	"os"
	"syscall"
)

// ProcessChecker 探测本机进程是否存活（Liveness：信号 0 探测，不注入信号）。
type ProcessChecker struct {
	PID int
}

// NewProcessChecker 构造进程存活探测。
func NewProcessChecker(pid int) *ProcessChecker { return &ProcessChecker{PID: pid} }

// Check implements Checker.
func (c *ProcessChecker) Check(_ context.Context) error {
	p, err := os.FindProcess(c.PID)
	if err != nil {
		return fmt.Errorf("find process %d: %w", c.PID, err)
	}
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("process %d not alive: %w", c.PID, err)
	}
	return nil
}
