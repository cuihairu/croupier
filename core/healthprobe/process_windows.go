//go:build windows

package healthprobe

import (
	"context"
	"fmt"
	"os"
)

// ProcessChecker 探测本机进程是否存活（Liveness）。
// Windows 无信号 0 探测，退化为 FindProcess 存在性检查（PID 复用窗口内
// 可能误报存活；精确判活需 x/sys/windows OpenProcess，按需再引）。
type ProcessChecker struct {
	PID int
}

// NewProcessChecker 构造进程存活探测。
func NewProcessChecker(pid int) *ProcessChecker { return &ProcessChecker{PID: pid} }

// Check implements Checker.
func (c *ProcessChecker) Check(_ context.Context) error {
	if _, err := os.FindProcess(c.PID); err != nil {
		return fmt.Errorf("find process %d: %w", c.PID, err)
	}
	return nil
}
