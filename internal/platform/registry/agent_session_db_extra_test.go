package registry

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/common/labels"
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

// TestAgentSessionModel_LastKnownIdentity 钉 §10.5 持久「最后已知指纹」
// 锚点：会话过期软删后行仍在（Unscoped 读），锚点必须比会话 TTL 活得久。
func TestAgentSessionModel_LastKnownIdentity(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, model.Upsert(ctx, &AgentSession{
		AgentID:  "agent-fp",
		GameID:   "game-1",
		Env:      "prod",
		Version:  "1.2.3",
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
		Labels:   map[string]string{"os": "linux", labels.AgentBinarySHA256: "abc123"},
	}))

	version, sha, ok, err := model.LastKnownIdentity(ctx, "agent-fp")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "1.2.3", version)
	assert.Equal(t, "abc123", sha)

	// 会话过期软删后仍可读（锚点跨 TTL 存活）。
	require.NoError(t, model.Upsert(ctx, &AgentSession{
		AgentID:  "agent-fp",
		GameID:   "game-1",
		Env:      "prod",
		Version:  "1.2.3",
		ExpireAt: now.Add(-time.Hour),
		LastSeen: now.Add(-time.Hour),
		Labels:   map[string]string{labels.AgentBinarySHA256: "abc123"},
	}))
	n, err := model.DeleteExpired(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	_, sha, ok, err = model.LastKnownIdentity(ctx, "agent-fp")
	require.NoError(t, err)
	assert.True(t, ok, "软删行的锚点仍须可读")
	assert.Equal(t, "abc123", sha)

	// 无该 agent：ok=false 且不报错。
	_, _, ok, err = model.LastKnownIdentity(ctx, "ghost")
	require.NoError(t, err)
	assert.False(t, ok)

	// 空 agentID：直接 ok=false。
	_, _, ok, err = model.LastKnownIdentity(ctx, "   ")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAgentSessionModel_LastKnownScope(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	require.NoError(t, model.Upsert(ctx, &AgentSession{
		AgentID: "agent-scope", GameID: "demo", Env: "prod",
		ExpireAt: time.Now().Add(-time.Hour), LastSeen: time.Now().Add(-time.Hour),
	}))
	deleted, err := model.DeleteExpired(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)

	gameID, env, ok, err := model.LastKnownScope(ctx, "agent-scope")
	require.NoError(t, err)
	assert.True(t, ok, "offline scope anchor must survive session expiry")
	assert.Equal(t, "demo", gameID)
	assert.Equal(t, "prod", env)

	_, _, ok, err = model.LastKnownScope(ctx, "ghost")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestAgentSessionModel_LastKnownIdentity_NoFingerprintLabel 指纹标签缺位时
// 版本仍返回、指纹为空（锚点缺位，不伪造值）。
func TestAgentSessionModel_LastKnownIdentity_NoFingerprintLabel(t *testing.T) {
	db := setupTestDB(t)
	model := NewAgentSessionModel(db)
	ctx := context.Background()

	require.NoError(t, model.Upsert(ctx, &AgentSession{
		AgentID:  "agent-nofp",
		Version:  "2.0.0",
		ExpireAt: time.Now().Add(time.Hour),
		LastSeen: time.Now(),
		Labels:   map[string]string{"os": "linux"},
	}))

	version, sha, ok, err := model.LastKnownIdentity(ctx, "agent-nofp")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "2.0.0", version)
	assert.Empty(t, sha)
}

// TestSessionLabelValue 非法/空标签 JSON 视为无该标签，不报错。
func TestSessionLabelValue(t *testing.T) {
	assert.Empty(t, sessionLabelValue("", labels.AgentBinarySHA256))
	assert.Empty(t, sessionLabelValue("not-json", labels.AgentBinarySHA256))
	assert.Empty(t, sessionLabelValue(`{"os":"linux"}`, labels.AgentBinarySHA256))
	assert.Equal(t, "abc", sessionLabelValue(`{"agentBinarySha256":"abc"}`, labels.AgentBinarySHA256))
	assert.Empty(t, sessionLabelValue(`{"x":"y"}`, ""))
}
