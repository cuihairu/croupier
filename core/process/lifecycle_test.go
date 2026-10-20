package process

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRestartDelayDefaultsAndCap(t *testing.T) {
	tests := []struct {
		name                string
		config              RestartConfig
		consecutiveFailures int
		expected            time.Duration
	}{
		{name: "first failure uses zero default", config: RestartConfig{}, consecutiveFailures: 1, expected: time.Second},
		{name: "zero default caps at one minute", config: RestartConfig{}, consecutiveFailures: 50, expected: time.Minute},
		{name: "exponential sequence", config: RestartConfig{InitialBackoff: time.Second, MaxBackoff: time.Minute}, consecutiveFailures: 3, expected: 4 * time.Second},
		{name: "max never reverses initial", config: RestartConfig{InitialBackoff: 10 * time.Second, MaxBackoff: 3 * time.Second}, consecutiveFailures: 5, expected: 10 * time.Second},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, RestartDelay(testCase.config, testCase.consecutiveFailures))
		})
	}
}

func TestBreakerTripped(t *testing.T) {
	config := RestartConfig{BreakerLimit: 5}
	assert.False(t, BreakerTripped(config, 4))
	assert.True(t, BreakerTripped(config, 5))
	assert.False(t, BreakerTripped(RestartConfig{}, 1000))
	assert.Contains(t, BreakerMessage(config, 5), "熔断阈值 5")
}

func TestExitDetailMapsWaitError(t *testing.T) {
	assert.Equal(t, ExitStatus{}, ExitDetail(nil))
	assert.Equal(t, ExitStatus{Code: -1}, ExitDetail(exec.ErrNotFound))
}

func TestOOMSuspectShortCircuits(t *testing.T) {
	assert.False(t, IsOOMSuspect("", 1<<40))
	assert.False(t, IsOOMSuspect("terminated", 1<<40))
	assert.False(t, IsOOMSuspect("SIGKILL", 0))
	assert.False(t, IsOOMSuspect("SIGKILL", 1))
}
