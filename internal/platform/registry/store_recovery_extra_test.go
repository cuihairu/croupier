package registry

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrOpenAPIString(v string) *string { return &v }

func TestStore_SessionPersistenceEnabled(t *testing.T) {
	assert.False(t, NewStore().SessionPersistenceEnabled())
	assert.False(t, NewStoreWithDB(nil).SessionPersistenceEnabled())
	assert.True(t, NewStoreWithDB(setupTestDB(t)).SessionPersistenceEnabled())

	var nilStore *Store
	assert.False(t, nilStore.SessionPersistenceEnabled())
}

func TestStore_SetScopeContextResolver_NilRestoresDefault(t *testing.T) {
	store := NewStore()
	custom := func(gameID, env string) context.Context { return context.Background() }
	store.SetScopeContextResolver(custom)
	store.SetScopeContextResolver(nil)
	assert.NotNil(t, store.scopeContext)
}

func TestStore_RebuildContext_FallsBackWhenResolverReturnsNil(t *testing.T) {
	store := NewStore()
	store.SetScopeContextResolver(func(gameID, env string) context.Context { return nil })
	assert.NotNil(t, store.rebuildContext("game", "env"))
}

func TestStore_PreviousFunctions(t *testing.T) {
	store := NewStore()

	assert.Nil(t, store.previousFunctions("missing"))

	require.NoError(t, store.UpsertAgent(&AgentSession{AgentID: "agent-no-funcs"}))
	assert.Nil(t, store.previousFunctions("agent-no-funcs"))

	require.NoError(t, store.UpsertAgent(&AgentSession{
		AgentID: "agent-funcs",
		Functions: map[string]FunctionMeta{
			"fn.a": {Version: "1"},
		},
	}))
	previous := store.previousFunctions("agent-funcs")
	require.Len(t, previous, 1)
	assert.Equal(t, "1", previous["fn.a"].Version)
}

func TestStore_DeleteOpenAPI_Lifecycle(t *testing.T) {
	store := NewStore()

	assert.Error(t, store.DeleteOpenAPI(""))
	assert.Error(t, store.DeleteOpenAPI("missing"))

	op := &openapi3.Operation{
		Summary: "list players",
		Responses: openapi3.NewResponses(func(responses *openapi3.Responses) {
			responses.Set("200", &openapi3.ResponseRef{
				Value: &openapi3.Response{Description: ptrOpenAPIString("ok")},
			})
		}),
	}
	require.NoError(t, store.UpsertOpenAPI("player.list", op))
	require.NoError(t, store.DeleteOpenAPI("player.list"))
	_, err := store.GetOpenAPI("player.list")
	assert.Error(t, err)
}

func TestStore_DeleteOpenAPIProvider_Lifecycle(t *testing.T) {
	store := NewStore()

	assert.Error(t, store.DeleteOpenAPIProvider(""))
	assert.Error(t, store.DeleteOpenAPIProvider("missing"))

	require.NoError(t, store.UpsertOpenAPIProvider(OpenAPIProviderCaps{ID: "provider-1"}))
	require.NoError(t, store.DeleteOpenAPIProvider("provider-1"))
	_, err := store.GetOpenAPIProvider("provider-1")
	assert.Error(t, err)
}

func TestCloneOpenAPIOperation_Nil(t *testing.T) {
	_, err := cloneOpenAPIOperation(nil)
	assert.Error(t, err)
}

type fakeSessionLoader struct {
	sessions []*AgentSession
	err      error
}

func (f *fakeSessionLoader) LoadActiveSessions(ctx context.Context) ([]*AgentSession, error) {
	return f.sessions, f.err
}

func TestStore_LoadFromDB_RequiresLoader(t *testing.T) {
	store := NewStoreWithDB(setupTestDB(t))
	err := store.LoadFromDB(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent session loader is required")
}

func TestStore_LoadFromDB_LoaderError(t *testing.T) {
	store := NewStoreWithDB(setupTestDB(t))
	loader := &fakeSessionLoader{err: assert.AnError}
	err := store.LoadFromDB(context.Background(), loader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load sessions")
}

func TestStore_RecoverPendingRegistrationOperations_Branches(t *testing.T) {
	t.Run("no contract service is a no-op", func(t *testing.T) {
		store := NewStoreWithDB(setupTestDB(t))
		require.NoError(t, store.recoverPendingRegistrationOperations(context.Background()))
	})

	t.Run("invalid previous session marks compensation required", func(t *testing.T) {
		db := setupTestDB(t)
		store := NewStoreWithDB(db)
		store.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:     "op-bad-previous",
			AgentID:         "agent-1",
			GameID:          "game",
			Env:             "dev",
			PreviousSession: "{not-json",
			TargetSession:   `{}`,
			Status:          "pending",
		}).Error)

		err := store.recoverPendingRegistrationOperations(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode previous registration session")

		var op AgentRegistrationOperationDB
		require.NoError(t, db.Where("operation_id = ?", "op-bad-previous").First(&op).Error)
		assert.Equal(t, "compensation_required", op.Status)
	})

	t.Run("invalid target session marks compensation required", func(t *testing.T) {
		db := setupTestDB(t)
		store := NewStoreWithDB(db)
		store.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:   "op-bad-target",
			AgentID:       "agent-1",
			GameID:        "game",
			Env:           "dev",
			TargetSession: "{not-json",
			Status:        "pending",
		}).Error)

		err := store.recoverPendingRegistrationOperations(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode target registration session")
	})

	t.Run("null target session reports empty error", func(t *testing.T) {
		db := setupTestDB(t)
		store := NewStoreWithDB(db)
		store.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:   "op-null-target",
			AgentID:       "agent-1",
			GameID:        "game",
			Env:           "dev",
			TargetSession: "null",
			Status:        "pending",
		}).Error)

		err := store.recoverPendingRegistrationOperations(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "target session is empty")
	})

	t.Run("materialization failure marks compensation required", func(t *testing.T) {
		db := setupTestDB(t)
		store := NewStoreWithDB(db)
		store.SetContractService(failingContractService{err: assert.AnError})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:   "op-materialize-fail",
			AgentID:       "agent-1",
			GameID:        "game",
			Env:           "dev",
			TargetSession: `{"AgentID":"agent-1","Functions":{"fn":{"Enabled":true}}}`,
			Status:        "pending",
		}).Error)

		err := store.recoverPendingRegistrationOperations(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recover registration operation")

		var op AgentRegistrationOperationDB
		require.NoError(t, db.Where("operation_id = ?", "op-materialize-fail").First(&op).Error)
		assert.Equal(t, "compensation_required", op.Status)
	})

	t.Run("successful recovery compensates pending operation", func(t *testing.T) {
		db := setupTestDB(t)
		store := NewStoreWithDB(db)
		store.SetContractService(&recordingContractService{})
		require.NoError(t, db.Create(&AgentRegistrationOperationDB{
			OperationID:   "op-ok",
			AgentID:       "agent-1",
			GameID:        "game",
			Env:           "dev",
			TargetSession: `{"AgentID":"agent-1","Functions":{"fn":{"Enabled":true}}}`,
			Status:        "compensation_required",
		}).Error)

		require.NoError(t, store.recoverPendingRegistrationOperations(context.Background()))

		var op AgentRegistrationOperationDB
		require.NoError(t, db.Where("operation_id = ?", "op-ok").First(&op).Error)
		assert.Equal(t, "compensated", op.Status)
	})
}

func TestDecodeRegistrationSession(t *testing.T) {
	t.Run("empty raw returns nil session", func(t *testing.T) {
		sess, err := decodeRegistrationSession("")
		require.NoError(t, err)
		assert.Nil(t, sess)
	})

	t.Run("null raw returns nil session", func(t *testing.T) {
		sess, err := decodeRegistrationSession("null")
		require.NoError(t, err)
		assert.Nil(t, sess)
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		_, err := decodeRegistrationSession("{bad")
		assert.Error(t, err)
	})

	t.Run("valid json returns session", func(t *testing.T) {
		sess, err := decodeRegistrationSession(`{"AgentID":"agent-9"}`)
		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, "agent-9", sess.AgentID)
	})
}

func TestStore_MarkRegistrationOperation_Guards(t *testing.T) {
	store := NewStore()
	// Empty operation ID and missing db must be safe no-ops.
	store.markRegistrationOperation("", "committed", nil)
	store.markRegistrationOperation("op-1", "committed", assert.AnError)
}

func TestMarkRegistrationOperationWithDB_NilDB(t *testing.T) {
	store := NewStore()
	err := store.markRegistrationOperationWithDB(nil, "op-1", "committed", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
