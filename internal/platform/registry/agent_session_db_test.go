package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Use unique in-memory database per test
	db, err := gorm.Open(gsqlite.Open("file:" + t.Name() + "?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, MigrateAgentSessions(db))
	return db
}

func TestAgentSessionDB_TableName(t *testing.T) {
	assert.Equal(t, "agent_sessions", AgentSessionDB{}.TableName())
}

func TestNewAgentSessionModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	assert.NotNil(t, model)
}

func TestMigrateAgentSessions(t *testing.T) {
	db := setupTestDB(t)
	// Should not error on second migration
	err := MigrateAgentSessions(db)
	assert.NoError(t, err)
}

func TestAgentSessionModel_Upsert(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	now := time.Now()
	sess := &AgentSession{
		AgentID:  "agent-1",
		GameID:   "game-1",
		Env:      "prod",
		RPCAddr:  "localhost:19090",
		Version:  "1.0.0",
		Region:   "us-west",
		Zone:     "zone-a",
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
		Labels:   map[string]string{"tier": "premium"},
		Functions: map[string]FunctionMeta{
			"player.get": {Enabled: true, Version: "1.0"},
		},
	}

	err := model.Upsert(ctx, sess)
	require.NoError(t, err)

	// Update the same agent
	sess.Version = "2.0.0"
	sess.Region = "us-east"
	err = model.Upsert(ctx, sess)
	require.NoError(t, err)
}

func TestAgentSessionModel_LoadActiveSessions(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	now := time.Now()

	// Insert active session
	err := model.Upsert(ctx, &AgentSession{
		AgentID:  "load-active-1",
		GameID:   "game-1",
		Env:      "prod",
		RPCAddr:  "localhost:19090",
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
	})
	require.NoError(t, err)

	// Insert expired session
	err = model.Upsert(ctx, &AgentSession{
		AgentID:  "load-expired-1",
		GameID:   "game-1",
		Env:      "prod",
		RPCAddr:  "localhost:19091",
		ExpireAt: now.Add(-time.Hour),
		LastSeen: now.Add(-time.Hour),
	})
	require.NoError(t, err)

	sessions, err := model.LoadActiveSessions(ctx)
	require.NoError(t, err)

	// Only active session should be returned
	found := false
	for _, s := range sessions {
		if s.AgentID == "load-active-1" {
			found = true
		}
		assert.NotEqual(t, "load-expired-1", s.AgentID)
	}
	assert.True(t, found, "active session should be found")
}

func TestAgentSessionModel_DeleteExpired(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	now := time.Now()

	// Insert active session
	err := model.Upsert(ctx, &AgentSession{
		AgentID:  "del-active-1",
		GameID:   "game-1",
		Env:      "prod",
		RPCAddr:  "localhost:19090",
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
	})
	require.NoError(t, err)

	// Insert expired session
	err = model.Upsert(ctx, &AgentSession{
		AgentID:  "del-expired-1",
		GameID:   "game-1",
		Env:      "prod",
		RPCAddr:  "localhost:19091",
		ExpireAt: now.Add(-time.Hour),
		LastSeen: now.Add(-time.Hour),
	})
	require.NoError(t, err)

	affected, err := model.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	// Active session should still exist
	sessions, err := model.LoadActiveSessions(ctx)
	require.NoError(t, err)

	found := false
	for _, s := range sessions {
		if s.AgentID == "del-active-1" {
			found = true
		}
	}
	assert.True(t, found, "active session should still exist after DeleteExpired")
}

func TestToDomainSession(t *testing.T) {
	t.Run("full session", func(t *testing.T) {
		labelsJSON := []byte(`{"tier":"premium"}`)
		funcsJSON := []byte(`{"player.get":{"description":"Get player"}}`)

		dbSess := &AgentSessionDB{
			AgentID:   "agent-1",
			GameID:    "game-1",
			Env:       "prod",
			RPCAddr:   "localhost:19090",
			Version:   "1.0.0",
			Region:    "us-west",
			Zone:      "zone-a",
			Labels:    labelsJSON,
			Functions: funcsJSON,
			ExpireAt:  time.Now().Add(time.Hour),
			LastSeen:  time.Now(),
		}

		sess, err := toDomainSession(dbSess)
		require.NoError(t, err)
		assert.Equal(t, "agent-1", sess.AgentID)
		assert.Equal(t, "premium", sess.Labels["tier"])
		assert.Contains(t, sess.Functions, "player.get")
	})

	t.Run("empty JSON fields", func(t *testing.T) {
		dbSess := &AgentSessionDB{
			AgentID: "agent-2",
			GameID:  "game-1",
			Env:     "prod",
			RPCAddr: "localhost:19090",
		}

		sess, err := toDomainSession(dbSess)
		require.NoError(t, err)
		assert.NotNil(t, sess.Labels)
		assert.NotNil(t, sess.Functions)
	})

	t.Run("invalid labels JSON", func(t *testing.T) {
		dbSess := &AgentSessionDB{
			AgentID: "agent-3",
			Labels:  []byte("invalid"),
		}

		_, err := toDomainSession(dbSess)
		assert.Error(t, err)
	})

	t.Run("invalid functions JSON", func(t *testing.T) {
		dbSess := &AgentSessionDB{
			AgentID:   "agent-4",
			Functions: []byte("invalid"),
		}

		_, err := toDomainSession(dbSess)
		assert.Error(t, err)
	})
}

func TestToDBSession(t *testing.T) {
	t.Run("full session", func(t *testing.T) {
		sess := &AgentSession{
			AgentID:  "agent-1",
			GameID:   "game-1",
			Env:      "prod",
			RPCAddr:  "localhost:19090",
			Version:  "1.0.0",
			Region:   "us-west",
			Zone:     "zone-a",
			ExpireAt: time.Now().Add(time.Hour),
			LastSeen: time.Now(),
			Labels:   map[string]string{"tier": "premium"},
			Functions: map[string]FunctionMeta{
				"player.get": {Enabled: true, Version: "1.0"},
			},
		}

		dbSess, err := toDBSession(sess)
		require.NoError(t, err)
		assert.Equal(t, "agent-1", dbSess.AgentID)
		assert.NotNil(t, dbSess.Labels)
		assert.NotNil(t, dbSess.Functions)
	})

	t.Run("nil maps", func(t *testing.T) {
		sess := &AgentSession{
			AgentID:  "agent-2",
			GameID:   "game-1",
			Env:      "prod",
			RPCAddr:  "localhost:19090",
			ExpireAt: time.Now().Add(time.Hour),
			LastSeen: time.Now(),
		}

		dbSess, err := toDBSession(sess)
		require.NoError(t, err)
		assert.Nil(t, dbSess.Labels)
		assert.Nil(t, dbSess.Functions)
	})
}
