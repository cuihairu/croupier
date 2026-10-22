package process

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSupervisorStateRestartPolicy(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	state := NewSupervisorState(RestartConfig{
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
		BreakerLimit:   3,
	})

	first := state.DecideExit(true, now)
	assert.Equal(t, SupervisorBackoff, first.Action)
	assert.Equal(t, 1, first.ConsecutiveFails)
	assert.Equal(t, 10*time.Millisecond, first.Delay)
	assert.Equal(t, now.Add(first.Delay), state.NextRestartAt())

	assert.False(t, state.ConsumeBackoff(now.Add(5*time.Millisecond)))
	assert.True(t, state.ConsumeBackoff(now.Add(first.Delay)))
	assert.True(t, state.NextRestartAt().IsZero())
	state.MarkRestartAttempted()
	assert.Equal(t, 1, state.ConsecutiveFails())

	second := state.DecideExit(true, now)
	assert.Equal(t, SupervisorBackoff, second.Action)
	assert.Equal(t, 2, second.ConsecutiveFails)
	assert.Equal(t, 20*time.Millisecond, second.Delay)

	third := state.DecideExit(true, now)
	assert.Equal(t, SupervisorBreak, third.Action)
	assert.Equal(t, 3, third.ConsecutiveFails)
	assert.True(t, state.NextRestartAt().IsZero())
}

func TestSupervisorStateDisallowAndRecovery(t *testing.T) {
	now := time.Now()
	state := NewSupervisorState(RestartConfig{BreakerLimit: 2})

	assert.Equal(t, SupervisorStop, state.DecideExit(false, now).Action)
	assert.Equal(t, 0, state.ConsecutiveFails())

	state.RecordStart()
	assert.Equal(t, 0, state.ConsecutiveFails())
	assert.True(t, state.NextRestartAt().IsZero())
	state.RecordSuccess()
	assert.Equal(t, 0, state.ConsecutiveFails())
	assert.Equal(t, 2*time.Second, state.SuccessWindow())
}
