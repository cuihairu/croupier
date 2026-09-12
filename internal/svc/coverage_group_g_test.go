package svc

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRawSQLiteDBG 打开一个裸 *sql.DB（glebarez sqlite 驱动），供迁移体直测。
func openRawSQLiteDBG(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "g.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

// makeReadOnlySQLiteDBG 在单连接上打开 query_only，令后续一切 DDL/DML 以
// "attempt to write a readonly database" 失败而读探针（HasTable/HasColumn）照常。
func makeReadOnlySQLiteDBG(t *testing.T, ddl ...string) *sql.DB {
	t.Helper()
	sqlDB := openRawSQLiteDBG(t)
	for _, stmt := range ddl {
		_, err := sqlDB.Exec(stmt)
		require.NoError(t, err)
	}
	// 单连接池：PRAGMA 只落在这一条连接上，之后的迁移写必然复用它。
	sqlDB.SetMaxOpenConns(1)
	_, err := sqlDB.Exec("PRAGMA query_only = ON")
	require.NoError(t, err)
	return sqlDB
}

// ---- db.go 不可覆盖分支备忘 ----------------------------------------------
//
// L379-381（removeDBFromPostgresDSN 补回 "?" 后缀）：死分支。正则
// `[^/?]+` 不吞 '?'，ReplaceAllString 恒保留未匹配的 "?query" 尾巴，因此
// dsn 含 '?' 时结果必然也含 '?'，`!strings.Contains(result, "?")` 恒为
// false（coverage_gapfill_j_test.go 的同名行为用例亦只能停在条件为假）。
//
// L396-398（createPostgresDatabase 的 sql.Open 错误）：死分支。pgx v5
// stdlib 的 Driver.OpenConnector 不解析 DSN（解析推迟到 Connect），对已
// 注册驱动 sql.Open 恒返回 nil error；坏 DSN 的报错发生在其后的
// db.Exec（连接触发 parse），走的是 L406 一支。
//
// L95-97（内存 sqlite gorm.Open 失败）：DSN 是硬编码的合法值，无注入缝隙。
// L127/L160/L193（postgres/mysql/sqlserver 的 dsnWithoutDB=="" 兜底）：
// removeDB*/replaceDB* 对非空输入永不返回 ""，且外层还需真实数据库服务
// 报 "database does not exist" 才可达。
// L230-232（openReadOnlyGorm 的 PRAGMA 失败）：gorm.Open 阶段已完成连接
// 校验，成功后同一条 `PRAGMA query_only = ON` 无确定性失败输入。

// 0021/0022/0023/0024 迁移体在 wrapGorm 阶段失败：closed *sql.DB 上
// probeDialect 的全部方言探针查询报错。
func TestGoMigrationBodiesProbeFailureG(t *testing.T) {
	closed := openRawSQLiteDBG(t)
	require.NoError(t, closed.Close())

	ctx := context.Background()
	for name, fn := range map[string]func(context.Context, *sql.DB) error{
		"0021": addContractPrevSchemaColumns,
		"0022": migrateTermDictionaryDisplayColumn,
		"0023": migrateComponentTemplateColumns,
		"0024": migrateSoftDeleteResidue,
	} {
		err := fn(ctx, closed)
		require.Error(t, err, "migration %s", name)
		assert.Contains(t, err.Error(), "probe dialect")
	}
}

// 0021：表存在且列缺失时，AddColumn 落在 query_only 连接上失败。
func TestAddContractPrevSchemaColumnsWriteFailureG(t *testing.T) {
	sqlDB := makeReadOnlySQLiteDBG(t,
		"CREATE TABLE function_contracts (id INTEGER PRIMARY KEY)")
	err := addContractPrevSchemaColumns(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrate: 0021")
	assert.Contains(t, err.Error(), "readonly database")
}

// 0022：term_dictionary 存在而 display 列缺失，AddColumn 写失败。
func TestMigrateTermDictionaryDisplayWriteFailureG(t *testing.T) {
	sqlDB := makeReadOnlySQLiteDBG(t,
		"CREATE TABLE term_dictionary (id INTEGER PRIMARY KEY)")
	err := migrateTermDictionaryDisplayColumn(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrate: 0022")
	assert.Contains(t, err.Error(), "readonly database")
}

// 0023：component_templates 存在而 params/digest 缺失，AddColumn 写失败。
func TestMigrateComponentTemplateColumnsWriteFailureG(t *testing.T) {
	sqlDB := makeReadOnlySQLiteDBG(t,
		"CREATE TABLE component_templates (id INTEGER PRIMARY KEY)")
	err := migrateComponentTemplateColumns(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrate: 0023")
	assert.Contains(t, err.Error(), "readonly database")
}

// 0024：表存在但没有 deleted_at 列（该库不含此模型的软删形态）→ 跳过该表。
func TestMigrateSoftDeleteResidueSkipsTableWithoutDeletedAtG(t *testing.T) {
	sqlDB := openRawSQLiteDBG(t)
	_, err := sqlDB.Exec("CREATE TABLE page_specs (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	require.NoError(t, migrateSoftDeleteResidue(context.Background(), sqlDB))
}

// 0024：表带 deleted_at 列，但 DELETE 落在 query_only 连接上失败。
func TestMigrateSoftDeleteResidueDeleteFailureG(t *testing.T) {
	sqlDB := makeReadOnlySQLiteDBG(t,
		"CREATE TABLE page_specs (id INTEGER PRIMARY KEY, deleted_at DATETIME)",
		"INSERT INTO page_specs (deleted_at) VALUES ('2024-01-01 00:00:00')")
	err := migrateSoftDeleteResidue(context.Background(), sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migrate: 0024 purge soft-deleted rows from page_specs")
}

// saveLocked：Audit 元数据里塞入不可 JSON 序列化的值（chan），Update 持久化
// 失败并返回错误。
func TestOpsStateStoreSaveMarshalFailureG(t *testing.T) {
	store := NewOpsStateStore(t.TempDir())
	_, err := store.Update(func(state *OpsState) {
		state.Audit.Entries = []OpsAuditEntry{{
			ID:       "op-1",
			Action:   "purge",
			Metadata: map[string]interface{}{"bad": make(chan int)},
		}}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json: unsupported type")

	// 状态本体仍在内存中，Snapshot 可读回。
	snap := store.Snapshot()
	require.Len(t, snap.Audit.Entries, 1)
	assert.Equal(t, "op-1", snap.Audit.Entries[0].ID)
}

// toAbs：工作目录被删除后 filepath.Abs 失败，退回原样返回相对路径。
// 仅在顺序测试阶段短暂破坏 cwd（并行用例此时都暂停在 t.Parallel() 上，
// 结束后立即恢复），不触碰任何外部状态。
func TestToAbsFallsBackWhenGetwdFailsG(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	sub := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.Chdir(sub))
	defer func() { require.NoError(t, os.Chdir(wd)) }()
	require.NoError(t, os.Remove(sub))

	assert.Equal(t, "relative/path.db", toAbs("relative/path.db"))
}

// ---- 其余不可覆盖分支备忘 -------------------------------------------------
//
// game_seed.go L245-247（buildGameFromSeed 的 SetEnvs 错误）：
// model.Game.SetEnvs 只做 json.Marshal([]GameEnv)，字段全为 string，
// 不存在可构造的失败输入。
// game_seed.go L321-322（humanizeGameID 的空 token continue）：
// strings.Fields 不会产出空串 token。
//
// migrations.go L87-88（init 的 goose 注册失败 panic）：init 在包加载时
// 以固定的合法迁移列表执行且仅执行一次，无测试注入点。
//
// ops_state_store.go L266-269（cloneOpsState 的 Unmarshal 失败）：
// 输入是同函数内刚 json.Marshal 成功的产物，结构体各字段均可无损往返。
//
// service_context.go：
//   - L279-283（audit.NewSQLAuditStore 错误）：该构造函数仅对 nil db
//     报错，而此处 db 来自 openDatabase（失败早在 L163 panic）。
//   - L470-472 / L476-478 / L479-481（seedBootstrapPermissions/Roles/Admins
//     错误日志）：三个函数所有路径都 return nil。
//   - L482-484（seedBootstrapGames 错误日志）：唯一错误源是启动中段
//     games 表 Count 失败，需注入真实数据库故障；任何能破坏该查询的
//     库形态（如 games 视图）都会先在 L234 BackfillEnvBindings panic。
//   - L512-513（JWT secret 错误后的开发态兜底）：jwtutil.ResolveSecret
//     的 dev 判定与 svc.isDevelopmentConfig 逐条等价，err!=nil 时必然
//     判为非开发态而走 panic，永不到达兜底行。
//   - L725-727（resolveBootstrapAuthDir 空 base 兜底）：
//     resolveBootstrapBaseDir 末端恒返回 runtime.DefaultBootstrapDataDir()
//     的非空结果。
//   - L1004-1006（derivePermissionResourceAction 的空 action 兜底）：
//     action 初值 "*"，splitPermissionCode 返回的 action 恒非空。
//   - L1228-1229（initObjectStore 未知驱动）：objstore.Validate 的
//     default 分支先于 switch 拒绝未知驱动。
