package registry

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Use unique in-memory database per test
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
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
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
	})
	require.NoError(t, err)

	// Insert expired session
	err = model.Upsert(ctx, &AgentSession{
		AgentID:  "load-expired-1",
		GameID:   "game-1",
		Env:      "prod",
		Addr:     "localhost:19091",
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
		ExpireAt: now.Add(time.Hour),
		LastSeen: now,
	})
	require.NoError(t, err)

	// Insert expired session
	err = model.Upsert(ctx, &AgentSession{
		AgentID:  "del-expired-1",
		GameID:   "game-1",
		Env:      "prod",
		Addr:     "localhost:19091",
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
		labelsJSON := `{"tier":"premium"}`
		funcsJSON := `{"player.get":{"description":"Get player"}}`

		dbSess := &AgentSessionDB{
			AgentID:   "agent-1",
			GameID:    "game-1",
			Env:       "prod",
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
		}

		sess, err := toDomainSession(dbSess)
		require.NoError(t, err)
		assert.NotNil(t, sess.Labels)
		assert.NotNil(t, sess.Functions)
	})

	t.Run("invalid labels JSON", func(t *testing.T) {
		dbSess := &AgentSessionDB{
			AgentID: "agent-3",
			Labels:  "invalid",
		}

		_, err := toDomainSession(dbSess)
		assert.Error(t, err)
	})

	t.Run("invalid functions JSON", func(t *testing.T) {
		dbSess := &AgentSessionDB{
			AgentID:   "agent-4",
			Functions: "invalid",
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
		assert.NotEmpty(t, dbSess.Labels)
		assert.NotEmpty(t, dbSess.Functions)
	})

	t.Run("nil maps", func(t *testing.T) {
		sess := &AgentSession{
			AgentID:  "agent-2",
			GameID:   "game-1",
			Env:      "prod",
			ExpireAt: time.Now().Add(time.Hour),
			LastSeen: time.Now(),
		}

		dbSess, err := toDBSession(sess)
		require.NoError(t, err)
		assert.Equal(t, "{}", dbSess.Labels)
		assert.Equal(t, "{}", dbSess.Functions)
		assert.Equal(t, "[]", dbSess.Providers)
	})
}

// createLegacyAgentSessions builds the pre-drop table shape (rpc_addr still
// present, Addr column absent) so migration tests exercise the real catch-up
// path instead of an AutoMigrate-fresh table.
func createLegacyAgentSessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	// NOT NULL 列（last_seen）必须带默认值：AutoMigrate 补列走 ALTER ADD，
	// sqlite 不允许「无默认值的 NOT NULL 新列」加在非空表上。
	require.NoError(t, db.Exec(`CREATE TABLE agent_sessions (
		id INTEGER PRIMARY KEY,
		agent_id TEXT NOT NULL,
		game_id TEXT,
		env TEXT,
		rpc_addr TEXT,
		expire_at DATETIME NOT NULL,
		last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_sessions
		(agent_id, game_id, env, rpc_addr, expire_at, last_seen)
		VALUES ('legacy-1', 'demo', 'prod', '10.0.0.1:19091', '2030-01-01 00:00:00', CURRENT_TIMESTAMP)`).Error)
}

func sqliteColumns(t *testing.T, db *gorm.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Raw("PRAGMA table_info(" + table + ")").Rows()
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt interface{}
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk))
		cols[name] = true
	}
	require.NoError(t, rows.Err())
	return cols
}

// CR-FIX-15：迁移后 rpc_addr 必须实际不存在（原生 DROP COLUMN，不走 GORM
// sqlite 的建新表-拷贝-改名重建路径），且存量数据原样保留。
// （与 extra_test 的同名旧用例互补：那边只断言迁移不报错，这边断言列真没了。）
func TestMigrateAgentSessions_NativeDropActuallyRemovesRPCAddr(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	createLegacyAgentSessions(t, db)

	require.NoError(t, MigrateAgentSessions(db))

	cols := sqliteColumns(t, db, "agent_sessions")
	assert.NotContains(t, cols, "rpc_addr", "legacy rpc_addr column must actually be gone")
	assert.Contains(t, cols, "addr", "migration must add the live-session Addr column")

	// addr 是 AutoMigrate 追加列，存量行为 NULL——只断言行与映射列数据仍在。
	var agentID, gameID string
	require.NoError(t, db.Raw("SELECT agent_id, game_id FROM agent_sessions WHERE agent_id = 'legacy-1'").
		Row().Scan(&agentID, &gameID))
	assert.Equal(t, "legacy-1", agentID)
	assert.Equal(t, "demo", gameID)
	var rows int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM agent_sessions").Row().Scan(&rows))
	assert.EqualValues(t, 1, rows)
}

// CR-FIX-15：删列失败不得被吞——引用 rpc_addr 的视图会让 SQLite 拒绝
// DROP COLUMN；迁移必须照常返回（未映射的遗留列保留无害，下次启动重试）
// 但 WARN 日志必须点名 rpc_addr。
func TestMigrateAgentSessions_DropFailureIsLoggedNotSwallowed(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	require.NoError(t, err)
	// 先把表迁到当前模型形态（此时无 rpc_addr，AutoMigrate 干净完成），
	// 再把 rpc_addr 加回去并立视图挡删：AutoMigrate 不删模型外列，
	// 于是 migration 只剩我们的原生 DROP——视图让它失败，进入告警分支。
	// （直接在 legacy 形态上立视图会先炸 AutoMigrate 的建新表-拷贝-改名
	// 重建路径——那正是 CR-FIX-15 要绕开的坑，不属于本用例靶点。）
	require.NoError(t, MigrateAgentSessions(db))
	require.NoError(t, db.Exec("ALTER TABLE agent_sessions ADD COLUMN rpc_addr TEXT").Error)
	require.True(t, db.Migrator().HasColumn(&AgentSessionDB{}, "rpc_addr"))
	require.NoError(t, db.Exec("CREATE VIEW agent_sessions_v AS SELECT rpc_addr FROM agent_sessions").Error)

	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	require.NoError(t, MigrateAgentSessions(db), "drop failure must not fail the whole migration")

	assert.Contains(t, logBuf.String(), "rpc_addr", "drop failure must be logged, not swallowed")
	assert.Contains(t, sqliteColumns(t, db, "agent_sessions"), "rpc_addr",
		"blocked drop keeps the legacy column in place")
}
