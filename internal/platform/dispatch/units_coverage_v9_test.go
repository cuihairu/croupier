package dispatch

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconnectionPolicyNegativeDelayClampedV9(t *testing.T) {
	p := &ReconnectionPolicy{
		MaxRetries:   3,
		InitialDelay: -time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2,
	}
	delay, err := p.NextDelay(0)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), delay)
}

func TestReconnectionStateShouldRetryEnabledV9(t *testing.T) {
	s := NewReconnectionState(&ReconnectionPolicy{MaxRetries: 2, EnableAutoReconnect: true})
	assert.True(t, s.ShouldRetry())

	s.SetEnabled(false)
	assert.False(t, s.ShouldRetry())
}

func TestSelectWeightedNaNFallsBackToLastV9(t *testing.T) {
	lb := NewLoadBalancer(StrategyWeighted, nil)
	state := NewAgentHealthState("agent-a", "", nil)
	state.healthScore.Store(int64(math.Float64bits(math.NaN())))

	got, err := lb.selectWeighted([]*Candidate{
		{AgentID: "agent-a", Health: state},
		{AgentID: "agent-b"},
	})
	require.NoError(t, err)
	assert.Equal(t, "agent-b", got.AgentID)
}

func TestTaskEventQueryAdapterStorageErrorsV9(t *testing.T) {
	db := openDispatchTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	adapter := NewTaskEventQueryAdapter(model.NewTaskEventModel(db), model.NewTaskRunModel(db))
	ctx := context.Background()

	_, err = adapter.ListEvents(ctx, "task-broken", 0)
	require.Error(t, err)

	_, err = adapter.GetRun(ctx, "task-broken")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrTaskRunNotFound)
}

func TestFileTaskRoutingStoreIOErrorsV9(t *testing.T) {
	t.Run("load fails when routing file is a directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "task_routing.json"), 0o755))

		_, err := NewFileTaskRoutingStore(dir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load task routing data")
	})

	t.Run("save fails when tmp path is a directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "task_routing.json.tmp"), 0o755))

		store, err := NewFileTaskRoutingStore(dir)
		require.NoError(t, err)

		err = store.Set("task-1", "agent-1")
		require.Error(t, err)
	})
}
