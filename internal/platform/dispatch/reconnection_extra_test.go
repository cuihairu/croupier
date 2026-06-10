package dispatch

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconnectionState_NextDelay(t *testing.T) {
	policy := &ReconnectionPolicy{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		Multiplier:    2.0,
		Jitter:        0,
		EnableAutoReconnect: true,
	}

	state := NewReconnectionState(policy)

	// First delay should be initial delay
	delay, err := state.NextDelay()
	require.NoError(t, err)
	assert.Equal(t, 100*time.Millisecond, delay)

	// Record attempt and check next delay
	state.RecordAttempt(nil)
	delay, err = state.NextDelay()
	require.NoError(t, err)
	assert.Equal(t, 200*time.Millisecond, delay) // 100ms * 2.0
}

func TestReconnectionState_GetLastError(t *testing.T) {
	policy := DefaultReconnectionPolicy()
	state := NewReconnectionState(policy)

	// Initially no error
	assert.Nil(t, state.GetLastError())

	// Record error
	testErr := errors.New("test error")
	state.RecordAttempt(testErr)
	assert.Equal(t, testErr, state.GetLastError())
}

func TestReconnectionManager_GetPolicy(t *testing.T) {
	policy := &ReconnectionPolicy{
		MaxRetries:   5,
		InitialDelay: 200 * time.Millisecond,
	}

	manager := NewReconnectionManager(policy)

	got := manager.GetPolicy()
	assert.Equal(t, policy, got)
}

func TestReconnectionManager_SetPolicy(t *testing.T) {
	policy1 := &ReconnectionPolicy{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
	}

	manager := NewReconnectionManager(policy1)

	// Create a state
	state1 := manager.GetOrCreateState("conn-1")
	state1.RecordAttempt(nil)
	assert.Equal(t, 1, state1.GetAttempt())

	// Set new policy - should reset all states
	policy2 := &ReconnectionPolicy{
		MaxRetries:   5,
		InitialDelay: 200 * time.Millisecond,
	}
	manager.SetPolicy(policy2)

	// Verify policy was updated
	got := manager.GetPolicy()
	assert.Equal(t, policy2, got)

	// Verify state was reset
	state1After := manager.GetOrCreateState("conn-1")
	assert.Equal(t, 0, state1After.GetAttempt())
}

func TestDispatcher_ListTaskRoutings(t *testing.T) {
	d := NewDispatcher(nil)

	// Register some tasks
	d.RegisterTask("task-1", "agent-1")
	d.RegisterTask("task-2", "agent-2")

	routings, err := d.ListTaskRoutings()
	require.NoError(t, err)
	assert.Len(t, routings, 2)
}

func TestDispatcher_ListTaskRoutings_Empty(t *testing.T) {
	d := NewDispatcher(nil)

	routings, err := d.ListTaskRoutings()
	require.NoError(t, err)
	assert.Empty(t, routings)
}
