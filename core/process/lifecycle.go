package process

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

const (
	DefaultRestartBackoffInitial = time.Second
	DefaultRestartBackoffMax     = 60 * time.Second
)

type RestartConfig struct {
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BreakerLimit   int
}

type ExitStatus struct {
	Code   int32
	Signal string
}

func RestartDelay(config RestartConfig, consecutiveFailures int) time.Duration {
	initial := config.InitialBackoff
	if initial <= 0 {
		initial = DefaultRestartBackoffInitial
	}
	max := config.MaxBackoff
	if max <= 0 {
		max = DefaultRestartBackoffMax
	}
	if max < initial {
		max = initial
	}

	delay := initial
	for range consecutiveFailures - 1 {
		if delay >= max {
			break
		}
		delay *= 2
	}
	if delay > max {
		delay = max
	}
	return delay
}

func BreakerTripped(config RestartConfig, consecutiveFailures int32) bool {
	return config.BreakerLimit > 0 && consecutiveFailures >= int32(config.BreakerLimit)
}

func BreakerMessage(config RestartConfig, consecutiveFailures int32) string {
	return fmt.Sprintf("连续失败 %d 次达到熔断阈值 %d，已停止自动拉起，请人工介入", consecutiveFailures, config.BreakerLimit)
}

func ExitDetail(waitErr error) ExitStatus {
	if waitErr == nil {
		return ExitStatus{}
	}
	var exitError *exec.ExitError
	if errors.As(waitErr, &exitError) {
		if signal := exitSignal(exitError); signal != "" {
			return ExitStatus{Code: -1, Signal: signal}
		}
		return ExitStatus{Code: int32(exitError.ExitCode())}
	}
	return ExitStatus{Code: -1}
}

func IsOOMSuspect(signal string, lastRSSBytes int64) bool {
	if lastRSSBytes <= 0 || !strings.Contains(strings.ToLower(signal), "kill") {
		return false
	}
	virtualMemory, err := mem.VirtualMemory()
	if err != nil || virtualMemory.Available <= 0 {
		return false
	}
	return float64(lastRSSBytes) >= 0.9*float64(virtualMemory.Available)
}
