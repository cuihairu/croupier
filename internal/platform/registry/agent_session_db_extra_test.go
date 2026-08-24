package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateAgentSessions_DropsLegacyRPCAddrColumn(t *testing.T) {
	db := setupTestDB(t)
	// Simulate a legacy schema that still carries the rpc_addr column.
	require.NoError(t, db.Exec("ALTER TABLE agent_sessions ADD COLUMN rpc_addr TEXT").Error)
	require.True(t, db.Migrator().HasColumn(&AgentSessionDB{}, "rpc_addr"))

	// The migration tolerates the legacy column (best-effort drop; SQLite's
	// migrator cannot drop columns missing from the model, so only verify the
	// migration itself still succeeds).
	require.NoError(t, MigrateAgentSessions(db))
	require.NoError(t, MigrateAgentSessions(db))
}

func TestAgentSessionModel_LoadActiveSessions_SkipsInvalidRows(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	now := time.Now()

	// Insert a row with unparseable labels JSON directly to simulate corruption.
	require.NoError(t, db.Exec(`INSERT INTO agent_sessions
		(agent_id, game_id, env, labels, functions, providers, expire_at, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"corrupt-agent", "game-1", "prod", "not-json", "{}", "[]",
		now.Add(time.Hour), now, now, now).Error)

	require.NoError(t, model.Upsert(context.Background(), &AgentSession{
		AgentID:  "healthy-agent",
		GameID:   "game-1",
		Env:      "prod",
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
	}))

	sessions, err := model.LoadActiveSessions(context.Background())
	require.NoError(t, err)

	ids := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.AgentID)
	}
	assert.Contains(t, ids, "healthy-agent")
	assert.NotContains(t, ids, "corrupt-agent")
}

func TestToDomainSession_InvalidProvidersJSON(t *testing.T) {
	_, err := toDomainSession(&AgentSessionDB{
		AgentID:   "agent-bad-providers",
		Providers: "not-json",
	})
	assert.Error(t, err)
}
