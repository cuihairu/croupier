package svc

// 覆盖率补洞（gap-fill）：只针对尚未覆盖的可达分支。不修改产品代码与
// 既有测试。

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- db.go 纯函数 / DSN 处理 ---------------------------------------------

// L379-381: removeDBFromPostgresDSN 保留 URL 查询参数。
func TestRemoveDBFromPostgresDSN_KeepsQueryParams(t *testing.T) {
	out := removeDBFromPostgresDSN("postgres://u:p@h:5432/game_x?sslmode=disable&x=1", "postgres")
	assert.Equal(t, "postgres://u:p@h:5432/postgres?sslmode=disable&x=1", out)

	// 结果已含 '?' 时不重复追加。
	out2 := removeDBFromPostgresDSN("postgres://u@h/db?sslmode=disable", "postgres")
	assert.Equal(t, "postgres://u@h/postgres?sslmode=disable", out2)
}

// L339-341: createMySQLDatabase 的 sql.Open DSN 解析失败。
func TestCreateMySQLDatabase_BadDSN(t *testing.T) {
	err := createMySQLDatabase("://bad-dsn", "db")
	require.Error(t, err)
}

// L396-398: createPostgresDatabase 的 sql.Open DSN 解析失败。
func TestCreatePostgresDatabase_BadDSN(t *testing.T) {
	err := createPostgresDatabase("://bad-dsn", "db")
	require.Error(t, err)
}

// L458-460: createSQLServerDatabase 的 sql.Open DSN 解析失败。
func TestCreateSQLServerDatabase_BadDSN(t *testing.T) {
	err := createSQLServerDatabase("://bad-dsn", "db")
	require.Error(t, err)
}

// ---- migrations.go -------------------------------------------------------

// L333-335: execution_logs 表建表失败（同名视图占位，HasTable 为 false 而
// CreateTable 撞名）。
func TestExecutionLogsMigration_CreateTableError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/m.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE VIEW execution_logs AS SELECT 1 AS id").Error)

	m := executionLogsTableMigration()
	require.NotNil(t, m.UpFnNoTxContext)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	err = m.UpFnNoTxContext(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0020 create execution_logs")
}

// ---- fanout.go -----------------------------------------------------------

// L138-140: currentVersionOf 的 db.DB() 失败（ConnPool 非 *sql.DB）。
func TestCurrentVersionOf_InvalidConnPool(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/f.db"), &gorm.Config{})
	require.NoError(t, err)
	db.ConnPool = stubConnPool{}
	if db.Statement != nil {
		db.Statement.ConnPool = stubConnPool{}
	}
	_, _, err = currentVersionOf(context.Background(), db, "sqlite")
	require.Error(t, err)
}

type stubConnPool struct{}

func (stubConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return nil, errors.New("not supported")
}
func (p stubConnPool) Prepare(query string) (*sql.Stmt, error) {
	return nil, errors.New("not supported")
}
func (p stubConnPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, errors.New("not supported")
}
func (p stubConnPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("not supported")
}
func (p stubConnPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}
func (p stubConnPool) Close() error            { return nil }
func (p stubConnPool) Begin() (*sql.Tx, error) { return nil, errors.New("not supported") }

// ---- game_seed.go --------------------------------------------------------

// L149-152: resolveGamesConfigPath 在 base 目录命中 games.json。
// （L143-148 的空 base 兜底不可达：resolveBootstrapBaseDir 末端有
// runtime.DefaultBootstrapDataDir() 兜底，恒返回非空。）
func TestResolveGamesConfigPath_BaseDirGamesJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{}
	cfg.Auth.UsersConfig = filepath.Join(dir, "users.json")

	// 无 games.json → 空。
	assert.Equal(t, "", resolveGamesConfigPath(cfg))

	// base 目录中存在 games.json → 命中。
	require.NoError(t, os.WriteFile(filepath.Join(dir, "games.json"), []byte(`{}`), 0o644))
	assert.Equal(t, filepath.Join(dir, "games.json"), resolveGamesConfigPath(cfg))
}

// L213-215: buildGameFromSeed 无任何别名输入时回落 humanize。
func TestBuildGameFromSeed_AliasHumanizeFallback(t *testing.T) {
	game, err := buildGameFromSeed(bootstrapGameSeedEntry{GameID: "player_mgr"}, nil, 0)
	require.NoError(t, err)
	assert.Equal(t, "Player Mgr", game.AliasName)
}

// ---- admin_manager.go ----------------------------------------------------

// L93-96 的 loadDefaultAdmins 错误传播分支不可达：读文件与 JSON 解析失败
// 均被容错（continue 尝试下一个配置文件），最终恒 return nil。

// ---- service_context.go --------------------------------------------------

// L605-607: autoMigrateServerModels 中 MigrateAgentSessions 失败
// （agent_sessions 被同名视图占位）。
func TestAutoMigrateServerModels_AgentSessionsError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/s.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrateMeta(db))
	require.NoError(t, db.Exec("DROP VIEW IF EXISTS agent_sessions").Error)
	require.NoError(t, db.Migrator().DropTable("agent_sessions"))
	require.NoError(t, db.Exec("CREATE VIEW agent_sessions AS SELECT 1 AS id").Error)

	err = autoMigrate(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to migrate agent sessions")
}

// L641-643: autoMigrateMeta 中 MigrateAgentSessions 失败。
func TestAutoMigrateMeta_AgentSessionsError(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/s2.db"), &gorm.Config{})
	require.NoError(t, err)
	// 先完整迁移一次。
	require.NoError(t, autoMigrateMeta(db))
	// agent_registration_operations 不在 MetaModels：换成同名视图后
	// migrateModels 对 MetaModels no-op，但 MigrateAgentSessions 建表失败。
	require.NoError(t, db.Migrator().DropTable("agent_registration_operations"))
	require.NoError(t, db.Exec("CREATE VIEW agent_registration_operations AS SELECT 1 AS id").Error)

	err = autoMigrateMeta(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to migrate agent sessions")
}

// L1461-1467: listGameEnvScopes 闭包的 Find 错误与数据映射。
func TestListGameEnvScopes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(t.TempDir()+"/scopes.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.GameEnvBinding{}))

	// 空表 → 空 scopes（覆盖循环体不执行）。
	scopes, err := listGameEnvScopes(db)(context.Background())
	require.NoError(t, err)
	assert.Empty(t, scopes)

	// 有数据 → 映射（覆盖 append 循环）。
	require.NoError(t, db.Create(&model.GameEnvBinding{GameID: "demo", Env: "prod", DatabaseName: "game_demo_prod"}).Error)
	scopes, err = listGameEnvScopes(db)(context.Background())
	require.NoError(t, err)
	require.Len(t, scopes, 1)
	assert.Equal(t, "demo", scopes[0].GameID)
	assert.Equal(t, "prod", scopes[0].Env)

	// 表缺失 → Find 错误。
	require.NoError(t, db.Migrator().DropTable("game_envs"))
	_, err = listGameEnvScopes(db)(context.Background())
	require.Error(t, err)
}
