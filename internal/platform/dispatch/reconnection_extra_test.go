package dispatch

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconnectionState_NextDelay(t *testing.T) {
	policy := &ReconnectionPolicy{
		MaxRetries:          3,
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		Multiplier:          2.0,
		Jitter:              0,
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

func TestNewReconnectionPolicy_EdgeCases(t *testing.T) {
	t.Run("zero initial delay uses default", func(t *testing.T) {
		p := NewReconnectionPolicy(3, 0, time.Second, 2.0, 0.1, true)
		if p.InitialDelay != 500*time.Millisecond {
			t.Errorf("expected default initial delay, got %v", p.InitialDelay)
		}
	})

	t.Run("negative initial delay uses default", func(t *testing.T) {
		p := NewReconnectionPolicy(3, -1*time.Second, time.Second, 2.0, 0.1, true)
		if p.InitialDelay != 500*time.Millisecond {
			t.Errorf("expected default initial delay, got %v", p.InitialDelay)
		}
	})

	t.Run("zero max delay uses default", func(t *testing.T) {
		p := NewReconnectionPolicy(3, time.Second, 0, 2.0, 0.1, true)
		if p.MaxDelay != 30*time.Second {
			t.Errorf("expected default max delay, got %v", p.MaxDelay)
		}
	})

	t.Run("multiplier <= 1 uses default", func(t *testing.T) {
		p := NewReconnectionPolicy(3, time.Second, time.Second, 1.0, 0.1, true)
		if p.Multiplier != 2.0 {
			t.Errorf("expected default multiplier, got %v", p.Multiplier)
		}
	})

	t.Run("negative jitter clamped to 0", func(t *testing.T) {
		p := NewReconnectionPolicy(3, time.Second, time.Second, 2.0, -0.5, true)
		if p.Jitter != 0 {
			t.Errorf("expected jitter 0, got %v", p.Jitter)
		}
	})

	t.Run("jitter > 1 clamped to 1", func(t *testing.T) {
		p := NewReconnectionPolicy(3, time.Second, time.Second, 2.0, 1.5, true)
		if p.Jitter != 1 {
			t.Errorf("expected jitter 1, got %v", p.Jitter)
		}
	})
}

func TestReconnectionPolicy_NextDelay_MaxRetriesExceeded(t *testing.T) {
	p := NewReconnectionPolicy(2, 100*time.Millisecond, time.Second, 2.0, 0, true)
	_, err := p.NextDelay(3)
	if err == nil {
		t.Error("expected error when max retries exceeded")
	}
}

func TestReconnectionPolicy_NextDelay_CapsAtMaxDelay(t *testing.T) {
	p := NewReconnectionPolicy(10, 100*time.Millisecond, 500*time.Millisecond, 2.0, 0, true)
	delay, err := p.NextDelay(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delay > 500*time.Millisecond {
		t.Errorf("delay %v exceeds max delay 500ms", delay)
	}
}

func TestReconnectionPolicy_ShouldRetry_Infinite(t *testing.T) {
	p := NewReconnectionPolicy(-1, time.Second, time.Second, 2.0, 0, true)
	if !p.ShouldRetry(100) {
		t.Error("ShouldRetry should return true for infinite retries")
	}
}

func TestPowFloat64(t *testing.T) {
	tests := []struct {
		base, exp, want float64
	}{
		{2, 0, 1},
		{2, 1, 2},
		{2, 3, 8},
		{0, 5, 0},
		{3, 2, 9},
		{1.5, 2, 2.25},
	}
	for _, tt := range tests {
		got := powFloat64(tt.base, tt.exp)
		if got != tt.want {
			t.Errorf("powFloat64(%v, %v) = %v, want %v", tt.base, tt.exp, got, tt.want)
		}
	}
}

func TestNewReconnectionState_NilPolicy(t *testing.T) {
	state := NewReconnectionState(nil)
	if state.policy == nil {
		t.Error("nil policy should use default")
	}
	if state.policy.MaxRetries != 5 {
		t.Errorf("expected default MaxRetries 5, got %d", state.policy.MaxRetries)
	}
}

func TestReconnectionState_ShouldRetry_Disabled(t *testing.T) {
	p := NewReconnectionPolicy(5, time.Second, time.Second, 2.0, 0, true)
	state := NewReconnectionState(p)
	state.SetEnabled(false)
	if state.ShouldRetry() {
		t.Error("ShouldRetry should return false when disabled")
	}
}

func TestReconnectionState_GetAttempt(t *testing.T) {
	p := NewReconnectionPolicy(5, time.Second, time.Second, 2.0, 0, true)
	state := NewReconnectionState(p)
	if state.GetAttempt() != 0 {
		t.Error("initial attempt should be 0")
	}
	state.RecordAttempt(nil)
	if state.GetAttempt() != 1 {
		t.Errorf("expected attempt 1, got %d", state.GetAttempt())
	}
}

func TestReconnectionState_IsEnabled(t *testing.T) {
	p := NewReconnectionPolicy(5, time.Second, time.Second, 2.0, 0, true)
	state := NewReconnectionState(p)
	if !state.IsEnabled() {
		t.Error("should be enabled by default")
	}
	state.SetEnabled(false)
	if state.IsEnabled() {
		t.Error("should be disabled after SetEnabled(false)")
	}
}

func TestNewReconnectionManager_NilPolicy(t *testing.T) {
	m := NewReconnectionManager(nil)
	if m.policy == nil {
		t.Error("nil policy should use default")
	}
}

func TestReconnectionManager_GetOrCreateState(t *testing.T) {
	m := NewReconnectionManager(nil)
	s1 := m.GetOrCreateState("key1")
	s2 := m.GetOrCreateState("key1")
	if s1 != s2 {
		t.Error("GetOrCreateState should return same instance")
	}
	s3 := m.GetOrCreateState("key2")
	if s1 == s3 {
		t.Error("different keys should return different instances")
	}
}

func TestReconnectionManager_RemoveState(t *testing.T) {
	m := NewReconnectionManager(nil)
	m.GetOrCreateState("key1")
	m.RemoveState("key1")
	s := m.GetOrCreateState("key1")
	if s.GetAttempt() != 0 {
		t.Error("removed state should be recreated fresh")
	}
}

func TestReconnectionManager_ResetAll(t *testing.T) {
	m := NewReconnectionManager(nil)
	s := m.GetOrCreateState("key1")
	s.RecordAttempt(errors.New("err"))
	m.ResetAll()
	if s.GetAttempt() != 0 {
		t.Error("ResetAll should reset attempts")
	}
}

func TestReconnectionManager_Stats(t *testing.T) {
	m := NewReconnectionManager(nil)
	m.GetOrCreateState("a")
	m.GetOrCreateState("b")
	stats := m.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 stats, got %d", len(stats))
	}
	if _, ok := stats["a"]; !ok {
		t.Error("missing stats for key a")
	}
}

func TestReconnectionStats_JSON(t *testing.T) {
	stats := ReconnectionStats{
		Attempt:     3,
		LastAttempt: time.Now(),
		LastError:   errors.New("test"),
		Enabled:     true,
	}
	if stats.Attempt != 3 {
		t.Error("expected attempt 3")
	}
	if !stats.Enabled {
		t.Error("expected enabled true")
	}
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

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	if id1 == "" {
		t.Error("generateTaskID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateTaskID() should generate unique IDs")
	}
	if !strings.HasPrefix(id1, "task-") {
		t.Errorf("generateTaskID() = %q, should start with 'task-'", id1)
	}
}

func TestNoHealthyAgentError(t *testing.T) {
	err := noHealthyAgentError("fn1", "game1", "prod", true)
	if err == nil {
		t.Fatal("noHealthyAgentError() returned nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "fn1") {
		t.Errorf("error should contain function ID, got %q", msg)
	}
	if !strings.Contains(msg, "no healthy agents") {
		t.Errorf("error should contain 'no healthy agents', got %q", msg)
	}
}
