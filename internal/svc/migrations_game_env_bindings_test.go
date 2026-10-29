package svc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 0049（ARCH-006 物理路由统一真值）：存量 game_envs 的空/纯空白
// database_name 被回填为规范默认名 game_<gameID>_<env>；已有合法名逐字保留
// （含自定义名与前缀历史名），幂等重跑 no-op。
func TestMigrateGameEnvBindingsDBNameBackfill(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	_, err := sqlDB.Exec(`CREATE TABLE game_envs (
		id INTEGER PRIMARY KEY,
		game_id TEXT,
		env TEXT,
		database_name TEXT NOT NULL,
		deleted_at DATETIME)`)
	require.NoError(t, err)
	rows := []struct {
		id   int
		game string
		env  string
		name string
	}{
		{1, "demo", "prod", ""},                     // 空串：回填
		{2, "demo", "staging", "   "},               // 纯空白：回填
		{3, "legacy", "prod", "game_legacy_prod"},   // 旧默认名：保留
		{4, "custom", "prod", "my_custom_db"},       // 自定义名：保留
		{5, "prefixed", "prod", "gp_prefixed_prod"}, // 旧前缀名：保留
	}
	for _, r := range rows {
		_, err := sqlDB.Exec(
			"INSERT INTO game_envs (id, game_id, env, database_name) VALUES (?, ?, ?, ?)",
			r.id, r.game, r.env, r.name)
		require.NoError(t, err)
	}

	require.NoError(t, migrateGameEnvBindingsDBNameBackfill(context.Background(), sqlDB))

	type row struct {
		id   int
		name string
	}
	var got []row
	rs, err := sqlDB.Query("SELECT id, database_name FROM game_envs ORDER BY id")
	require.NoError(t, err)
	for rs.Next() {
		var r row
		require.NoError(t, rs.Scan(&r.id, &r.name))
		got = append(got, r)
	}
	require.NoError(t, rs.Err())
	require.Len(t, got, 5)

	assert.Equal(t, "game_demo_prod", got[0].name, "空名回填为默认名")
	assert.Equal(t, "game_demo_staging", got[1].name, "空白名回填为默认名")
	assert.Equal(t, "game_legacy_prod", got[2].name, "已有合法名逐字保留")
	assert.Equal(t, "my_custom_db", got[3].name, "自定义名保留")
	assert.Equal(t, "gp_prefixed_prod", got[4].name, "前缀历史名保留")

	// 幂等：回填后无空名行，重跑 no-op、名字不变。
	require.NoError(t, migrateGameEnvBindingsDBNameBackfill(context.Background(), sqlDB))
	var name string
	require.NoError(t, sqlDB.QueryRow("SELECT database_name FROM game_envs WHERE id = 1").Scan(&name))
	assert.Equal(t, "game_demo_prod", name)
}

// 0049：软删行也回填——恢复绑定时得到合法名字（无害且幂等）。
func TestMigrateGameEnvBindingsDBNameBackfillCoversSoftDeleted(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	_, err := sqlDB.Exec(`CREATE TABLE game_envs (
		id INTEGER PRIMARY KEY,
		game_id TEXT,
		env TEXT,
		database_name TEXT NOT NULL,
		deleted_at DATETIME)`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO game_envs (id, game_id, env, database_name, deleted_at)
		VALUES (1, 'gone', 'prod', '', '2026-10-01 00:00:00')`)
	require.NoError(t, err)

	require.NoError(t, migrateGameEnvBindingsDBNameBackfill(context.Background(), sqlDB))

	var name string
	require.NoError(t, sqlDB.QueryRow("SELECT database_name FROM game_envs WHERE id = 1").Scan(&name))
	assert.Equal(t, "game_gone_prod", name)
}

// 0049：缺表跳过（multiGame fanout 到 game 物理库的库没有 game_envs）。
func TestMigrateGameEnvBindingsDBNameBackfillSkipsMissingTable(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	require.NoError(t, migrateGameEnvBindingsDBNameBackfill(context.Background(), sqlDB))
}
