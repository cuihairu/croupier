package svc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 0039：存量软删游戏墓碑行被物理清除，活跃行保留——同名重建不再撞
// idx_games_game_id/idx_games_alias_name 物理唯一索引（0024/0026 同族）。
func TestMigrateGameSoftDeleteResiduePurgesTombstones(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	_, err := sqlDB.Exec(`CREATE TABLE games (
		id INTEGER PRIMARY KEY,
		game_id TEXT,
		name TEXT,
		alias_name TEXT,
		deleted_at DATETIME)`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO games (id, game_id, name, alias_name, deleted_at)
		VALUES (1, 'tombstone_game', 'tombstone_game', '墓碑', '2026-10-04 23:19:05')`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO games (id, game_id, name, alias_name, deleted_at)
		VALUES (2, 'live_game', 'live_game', '活游戏', NULL)`)
	require.NoError(t, err)

	require.NoError(t, migrateGameSoftDeleteResidue(context.Background(), sqlDB))

	var n int
	require.NoError(t, sqlDB.QueryRow("SELECT COUNT(1) FROM games").Scan(&n))
	assert.Equal(t, 1, n, "墓碑行应被清除，活跃行保留")
	var live string
	require.NoError(t, sqlDB.QueryRow("SELECT game_id FROM games").Scan(&live))
	assert.Equal(t, "live_game", live)

	// 幂等：重跑不报错、不误删。
	require.NoError(t, migrateGameSoftDeleteResidue(context.Background(), sqlDB))
	require.NoError(t, sqlDB.QueryRow("SELECT COUNT(1) FROM games").Scan(&n))
	assert.Equal(t, 1, n)
}

// 0039：缺表跳过（fanout 重放到无该表的库不报错、不建空壳）。
func TestMigrateGameSoftDeleteResidueSkipsMissingTable(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	require.NoError(t, migrateGameSoftDeleteResidue(context.Background(), sqlDB))
}

// 0039：表存在但没有 deleted_at 列（该库不含此模型的软删形态）→ 跳过。
func TestMigrateGameSoftDeleteResidueSkipsTableWithoutDeletedAt(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	_, err := sqlDB.Exec("CREATE TABLE games (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, migrateGameSoftDeleteResidue(context.Background(), sqlDB))
}

// 0039：wrapGorm 阶段失败（closed *sql.DB 上方言探针全部报错）。
func TestMigrateGameSoftDeleteResidueProbeFailure(t *testing.T) {
	closed := openRawSQLiteDBG(t)
	require.NoError(t, closed.Close())
	err := migrateGameSoftDeleteResidue(context.Background(), closed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "probe dialect")
}

// 0039：DELETE 落在 query_only 连接上失败时错误带表名（0024 同款断言）。
func TestMigrateGameSoftDeleteResidueDeleteFailure(t *testing.T) {
	sqlDB := makeReadOnlySQLiteDBG(t,
		"CREATE TABLE games (id INTEGER PRIMARY KEY, deleted_at DATETIME)",
		"INSERT INTO games (deleted_at) VALUES ('2026-10-04 23:19:05')")
	err := migrateGameSoftDeleteResidue(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrate: 0039 purge soft-deleted rows from games")
}
