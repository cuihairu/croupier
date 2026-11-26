package model

import (
	"context"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewAgentSessionModel(t *testing.T) {
	db := setupAgentSessionTestDB(t)
	model := NewAgentSessionModel(db)
	assert.NotNil(t, model)
}

func TestAgentSessionModel_Upsert(t *testing.T) {
	db := setupAgentSessionTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	session := &registry.AgentSession{
		AgentID: "agent-001",
		GameID:  "game001",
		Env:     "prod",
		Addr:    "192.168.1.1:8080",
		Version: "1.0.0",
		Region:  "us-east",
		Zone:    "zone1",
		Labels:  map[string]string{"rack": "r1"},
		Providers: []registry.ProviderSession{
			{ProviderID: "provider1", GameID: "game001", Env: "prod", Addr: "192.168.1.10:8080"},
			{ProviderID: "provider2", GameID: "game001", Env: "prod", Addr: "192.168.1.11:8080"},
		},
		ExpireAt: time.Now().Add(1 * time.Hour),
		LastSeen: time.Now(),
	}

	err := model.Upsert(ctx, session)
	require.NoError(t, err)

	// Update via upsert
	session.Version = "1.0.1"
	err = model.Upsert(ctx, session)
	require.NoError(t, err)
}

func TestAgentSessionModel_LoadActiveSessions(t *testing.T) {
	db := setupAgentSessionTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	// Create active and expired sessions
	active := &registry.AgentSession{
		AgentID:  "active-agent",
		GameID:   "game001",
		Env:      "prod",
		Addr:     "192.168.1.1:8080",
		ExpireAt: time.Now().Add(1 * time.Hour),
		LastSeen: time.Now(),
	}
	expired := &registry.AgentSession{
		AgentID:  "expired-agent",
		GameID:   "game001",
		Env:      "prod",
		Addr:     "192.168.1.2:8080",
		ExpireAt: time.Now().Add(-1 * time.Hour),
		LastSeen: time.Now().Add(-2 * time.Hour),
	}

	err := model.Upsert(ctx, active)
	require.NoError(t, err)
	err = model.Upsert(ctx, expired)
	require.NoError(t, err)

	// Load active sessions
	sessions, err := model.LoadActiveSessions(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 1)

	// Verify only active sessions are returned
	for _, s := range sessions {
		assert.True(t, s.ExpireAt.After(time.Now()))
	}
}

func TestAgentSessionModel_DeleteExpired(t *testing.T) {
	db := setupAgentSessionTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	// Create expired sessions
	expired1 := &registry.AgentSession{
		AgentID:  "expired-1",
		GameID:   "game001",
		Env:      "prod",
		Addr:     "192.168.1.1:8080",
		ExpireAt: time.Now().Add(-1 * time.Hour),
		LastSeen: time.Now().Add(-2 * time.Hour),
	}
	expired2 := &registry.AgentSession{
		AgentID:  "expired-2",
		GameID:   "game001",
		Env:      "prod",
		Addr:     "192.168.1.2:8080",
		ExpireAt: time.Now().Add(-30 * time.Minute),
		LastSeen: time.Now().Add(-1 * time.Hour),
	}

	err := model.Upsert(ctx, expired1)
	require.NoError(t, err)
	err = model.Upsert(ctx, expired2)
	require.NoError(t, err)

	// Delete expired
	count, err := model.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2))
}

func TestToDomainSession(t *testing.T) {
	labelsJSON := JSON([]byte(`{"rack":"r1","zone":"z1"}`))
	providersJSON := JSON([]byte(`[{"ProviderID":"provider1","GameID":"game001","Env":"prod","Addr":"192.168.1.10:8080"},{"ProviderID":"provider2","GameID":"game001","Env":"prod","Addr":"192.168.1.11:8080"}]`))

	dbSess := &AgentSessionDB{
		ID:        1,
		AgentID:   "agent-001",
		GameID:    "game001",
		Env:       "prod",
		Version:   "1.0.0",
		Region:    "us-east",
		Zone:      "zone1",
		Labels:    labelsJSON,
		Providers: providersJSON,
		ExpireAt:  time.Now().Add(1 * time.Hour),
		LastSeen:  time.Now(),
	}

	sess, err := toDomainSession(dbSess)
	require.NoError(t, err)
	assert.Equal(t, "agent-001", sess.AgentID)
	assert.Equal(t, "game001", sess.GameID)
	assert.Equal(t, "r1", sess.Labels["rack"])
	assert.Equal(t, "z1", sess.Labels["zone"])
	assert.Len(t, sess.Providers, 2)
	if len(sess.Providers) >= 1 {
		assert.Equal(t, "provider1", sess.Providers[0].ProviderID)
	}
	if len(sess.Providers) >= 2 {
		assert.Equal(t, "provider2", sess.Providers[1].ProviderID)
	}

	// Test invalid JSON
	dbSess.Labels = JSON([]byte(`invalid json`))
	_, err = toDomainSession(dbSess)
	assert.Error(t, err)
}

func TestToDBSession(t *testing.T) {
	sess := &registry.AgentSession{
		AgentID: "agent-001",
		GameID:  "game001",
		Env:     "prod",
		Addr:    "192.168.1.1:8080",
		Version: "1.0.0",
		Region:  "us-east",
		Zone:    "zone1",
		Labels:  map[string]string{"rack": "r1", "zone": "z1"},
		Providers: []registry.ProviderSession{
			{ProviderID: "provider1", GameID: "game001", Env: "prod", Addr: "192.168.1.10:8080"},
			{ProviderID: "provider2", GameID: "game001", Env: "prod", Addr: "192.168.1.11:8080"},
		},
		ExpireAt: time.Now().Add(1 * time.Hour),
		LastSeen: time.Now(),
	}

	dbSess, err := toDBSession(sess)
	require.NoError(t, err)
	assert.Equal(t, "agent-001", dbSess.AgentID)
	assert.Equal(t, "game001", dbSess.GameID)
	assert.NotEmpty(t, dbSess.Labels)
	assert.NotEmpty(t, dbSess.Providers)
}
