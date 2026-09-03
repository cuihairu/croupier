package registry

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_UpsertRegistrationWarning(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	t.Run("empty message is ignored", func(t *testing.T) {
		store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			Message: "",
		})
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{})
		assert.Empty(t, warnings)
	})

	t.Run("new warning is added", func(t *testing.T) {
		store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			AgentID:    "agent-1",
			FunctionID: "func-1",
			Code:       "ERR_001",
			Message:    "test error",
		})
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{})
		require.Len(t, warnings, 1)
		assert.Equal(t, "agent-1", warnings[0].AgentID)
		assert.Equal(t, "func-1", warnings[0].FunctionID)
		assert.Equal(t, "ERR_001", warnings[0].Code)
		assert.Equal(t, "test error", warnings[0].Message)
		assert.Equal(t, 1, warnings[0].Count)
	})

	t.Run("duplicate warning increments count", func(t *testing.T) {
		store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			AgentID:    "agent-1",
			FunctionID: "func-1",
			Code:       "ERR_001",
			Message:    "test error",
		})
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{})
		require.Len(t, warnings, 1)
		assert.Equal(t, 2, warnings[0].Count)
	})

	t.Run("warning with explicit key", func(t *testing.T) {
		store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
			Key:     "custom-key",
			AgentID: "agent-2",
			Message: "custom key warning",
		})
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{
			AgentID: "agent-2",
		})
		require.Len(t, warnings, 1)
		assert.Equal(t, "custom-key", warnings[0].Key)
	})
}

func TestStore_UpsertAgentUsesScopedContextForContractRebuild(t *testing.T) {
	store := NewStore()
	recorder := &recordingContractService{}
	store.SetContractService(recorder)
	store.SetScopeContextResolver(func(gameID, env string) context.Context {
		return context.WithValue(context.Background(), registryTestScopeKey{}, seenScope{gameID: gameID, env: env})
	})

	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]FunctionMeta{
			"player.query": {
				Enabled:    true,
				Resource:   "player",
				Capability: "collection_query",
				Execution:  "sync",
			},
		},
	})

	require.NotEmpty(t, recorder.scopes)
	for _, seen := range recorder.scopes {
		assert.Equal(t, "demo-game", seen.gameID)
		assert.Equal(t, "development", seen.env)
	}
}

func TestUpsertAgentClassifiesFunctionSnapshotDiff(t *testing.T) {
	previous := map[string]FunctionMeta{
		"z.removed": {Resource: "z"},
		"b.changed": {Version: "1", Resource: "old"},
		"a.same":    {Version: "1", Resource: "same"},
	}
	current := map[string]FunctionMeta{
		"c.added":   {Resource: "new"},
		"b.changed": {Version: "2", Resource: "new"},
		"a.same":    {Version: "1", Resource: "same"},
	}
	diff := classifyFunctionSnapshot(previous, current)
	assert.Equal(t, []string{"c.added"}, diff.Added)
	assert.Equal(t, []string{"b.changed"}, diff.Changed)
	assert.Equal(t, []string{"z.removed"}, diff.Removed)
	assert.Equal(t, []string{"new", "old", "same", "z"}, diff.Resources)
	assert.Empty(t, classifyFunctionSnapshot(previous, nil).Removed)
}

type recordingContractService struct {
	scopes []seenScope
}

type seenScope struct {
	gameID string
	env    string
}

type registryTestScopeKey struct{}

func (r *recordingContractService) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta spec.FunctionContractInput) error {
	r.record(ctx)
	return nil
}

func (r *recordingContractService) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	r.record(ctx)
	return "", nil
}

func (r *recordingContractService) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	r.record(ctx)
	return nil
}

func (r *recordingContractService) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	r.record(ctx)
	return nil
}

func (r *recordingContractService) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	r.record(ctx)
	return nil
}

func (r *recordingContractService) record(ctx context.Context) {
	seen, _ := ctx.Value(registryTestScopeKey{}).(seenScope)
	r.scopes = append(r.scopes, seen)
}

func TestStore_UpsertAgentFailsWhenContractRebuildFails(t *testing.T) {
	store := NewStore()
	store.SetContractService(failingContractService{err: assert.AnError})

	err := store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]FunctionMeta{
			"player.query": {
				Enabled:    true,
				Version:    "1.0.0",
				Resource:   "player",
				Capability: "collection_query",
				Execution:  "sync",
			},
		},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "agent registration contract rebuild failed")
	assert.Nil(t, store.AgentsUnsafe()["agent-1"])
}

func TestStore_UpsertAgentRejectsPresentationSchemaBeforePersistence(t *testing.T) {
	store := NewStore()
	recorder := &recordingContractService{}
	store.SetContractService(recorder)

	err := store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "demo-game",
		Env:     "development",
		Functions: map[string]FunctionMeta{
			"player.list": {
				Enabled:     true,
				InputSchema: `{"type":"object","x-menu":"Players"}`,
			},
		},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, `function "player.list" inputSchema.x-menu`)
	assert.ErrorContains(t, err, `forbidden presentation field "x-menu"`)
	assert.Empty(t, store.AgentsUnsafe())
	assert.Empty(t, recorder.scopes)
}

type failingContractService struct {
	err error
}

func (f failingContractService) RebuildContractFromFunctionMeta(context.Context, string, string, string, spec.FunctionContractInput) error {
	return f.err
}

func (f failingContractService) RemoveFunctionContract(context.Context, string, string, string) (string, error) {
	return "", f.err
}

func (f failingContractService) RebuildResourceCapability(context.Context, string, string, string) error {
	return f.err
}

func (f failingContractService) RebuildProposalsForResource(context.Context, string, string, string) error {
	return f.err
}

func (f failingContractService) RebuildProposalForFunction(context.Context, string, string, string) error {
	return f.err
}

func TestStore_ListRegistrationWarnings_Filter(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Add multiple warnings
	store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		AgentID:    "agent-1",
		FunctionID: "func-1",
		Code:       "ERR_001",
		Message:    "error 1",
	})
	store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		AgentID:    "agent-2",
		FunctionID: "func-2",
		Code:       "ERR_002",
		Message:    "error 2",
	})
	store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		AgentID:    "agent-1",
		FunctionID: "func-3",
		Code:       "ERR_001",
		Message:    "error 3",
	})

	t.Run("filter by agent ID", func(t *testing.T) {
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{
			AgentID: "agent-1",
		})
		assert.Len(t, warnings, 2)
	})

	t.Run("filter by function ID", func(t *testing.T) {
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{
			FunctionID: "func-2",
		})
		assert.Len(t, warnings, 1)
		assert.Equal(t, "func-2", warnings[0].FunctionID)
	})

	t.Run("filter by code", func(t *testing.T) {
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{
			Code: "ERR_001",
		})
		assert.Len(t, warnings, 2)
	})

	t.Run("filter with limit", func(t *testing.T) {
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{
			Limit: 1,
		})
		assert.Len(t, warnings, 1)
	})

	t.Run("no filter returns all", func(t *testing.T) {
		warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{})
		assert.Len(t, warnings, 3)
	})
}

func TestStore_Mu(t *testing.T) {
	store := NewStore()
	mu := store.Mu()
	assert.NotNil(t, mu)
}

func TestStore_AgentsUnsafe(t *testing.T) {
	store := NewStore()

	// Add an agent
	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		GameID:  "game-1",
	})

	// Get agents map
	agents := store.AgentsUnsafe()
	assert.Len(t, agents, 1)
	assert.Contains(t, agents, "agent-1")
}

func TestStore_cleanupExpiredSessions(t *testing.T) {
	store := NewStore()

	// Add active and expired sessions
	store.UpsertAgent(&AgentSession{
		AgentID:  "active-agent",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(time.Hour),
	})
	store.UpsertAgent(&AgentSession{
		AgentID:  "expired-agent",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(-time.Hour),
	})

	// Run cleanup
	store.cleanupExpiredSessions()

	// Verify expired agent was removed
	agents := store.AgentsUnsafe()
	assert.Len(t, agents, 1)
	assert.Contains(t, agents, "active-agent")
	assert.NotContains(t, agents, "expired-agent")
}

func TestStore_StartCleanupRoutine(t *testing.T) {
	store := NewStore()

	// Add an expired session
	store.UpsertAgent(&AgentSession{
		AgentID:  "expired-agent",
		GameID:   "game-1",
		ExpireAt: time.Now().Add(-time.Hour),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cleanup routine with short interval
	store.StartCleanupRoutine(ctx, 50*time.Millisecond)

	// Wait for cleanup to run
	time.Sleep(150 * time.Millisecond)

	// Verify expired agent was removed (hold the lock: the cleanup routine
	// mutates the map concurrently).
	store.Mu().RLock()
	agents := store.AgentsUnsafe()
	assert.Len(t, agents, 0)
	store.Mu().RUnlock()
}

func TestStore_LoadFromDB_NoDB(t *testing.T) {
	store := NewStore()

	// LoadFromDB should fail when db is nil
	err := store.LoadFromDB(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not enabled")
}

func TestStore_UpsertRegistrationWarning_UpdateExisting(t *testing.T) {
	store := NewStore()
	ctx := context.Background()

	// Add initial warning
	store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		AgentID:    "agent-1",
		FunctionID: "func-1",
		Code:       "ERR_001",
		Message:    "test error",
		Version:    "",
	})

	// Update with version info
	store.UpsertRegistrationWarning(ctx, FunctionRegistrationWarning{
		AgentID:    "agent-1",
		FunctionID: "func-1",
		Code:       "ERR_001",
		Message:    "test error",
		Version:    "1.0.0",
	})

	warnings := store.ListRegistrationWarnings(RegistrationWarningFilter{})
	require.Len(t, warnings, 1)
	assert.Equal(t, 2, warnings[0].Count)
	assert.Equal(t, "1.0.0", warnings[0].Version)
}

func TestNewStoreWithDB(t *testing.T) {
	store := NewStoreWithDB(nil)
	assert.NotNil(t, store)
	assert.NotNil(t, store.agents)
	assert.NotNil(t, store.openapiOperations)
	assert.NotNil(t, store.openapiProviders)
	assert.NotNil(t, store.registrationWarnings)
}

func TestStore_writeToDB_NilDB(t *testing.T) {
	store := NewStore()

	// writeToDB with nil db should return nil
	err := store.writeToDB(context.Background(), &AgentSession{
		AgentID: "agent-1",
	})
	assert.NoError(t, err)
}

func TestStore_UpsertAgent_NilSession(t *testing.T) {
	store := NewStore()

	// Should not panic with nil session
	store.UpsertAgent(nil)

	// Should not panic with empty agent ID
	store.UpsertAgent(&AgentSession{})
}

func TestStore_UpsertAgent_MergeLabels(t *testing.T) {
	store := NewStore()

	// First upsert
	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		Labels:  map[string]string{"key1": "value1"},
	})

	// Second upsert with new labels
	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		Labels:  map[string]string{"key2": "value2"},
	})

	agents := store.AgentsUnsafe()
	agent := agents["agent-1"]
	require.NotNil(t, agent)
	assert.Equal(t, "value1", agent.Labels["key1"])
	assert.Equal(t, "value2", agent.Labels["key2"])
}

func TestStore_UpsertAgent_UpdateFunctions(t *testing.T) {
	store := NewStore()

	// First upsert
	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		Functions: map[string]FunctionMeta{
			"func-1": {Enabled: true, Version: "1.0"},
		},
	})

	// Second upsert with different functions
	store.UpsertAgent(&AgentSession{
		AgentID: "agent-1",
		Functions: map[string]FunctionMeta{
			"func-2": {Enabled: true, Version: "2.0"},
		},
	})

	agents := store.AgentsUnsafe()
	agent := agents["agent-1"]
	require.NotNil(t, agent)
	assert.Len(t, agent.Functions, 1)
	assert.Contains(t, agent.Functions, "func-2")
}
