package svc

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
// L379-381（removeDBFromPostgresDSN 补回 "?" 后缀）：死分支【已删】。正则
// `[^/?]+` 不吞 '?'，ReplaceAllString 恒保留未匹配的 "?query" 尾巴，因此
// dsn 含 '?' 时结果必然也含 '?'，`!strings.Contains(result, "?")` 恒为
// false（coverage_gapfill_j_test.go 的同名行为用例亦只能停在条件为假）。
//
// L396-398（createPostgresDatabase 的 sql.Open 错误）：死分支，产品代码
// 已加 C 类论证注释。pgx v5 stdlib 的 Driver.OpenConnector 不解析 DSN
// （解析推迟到 Connect），对已注册驱动 sql.Open 恒返回 nil error；坏 DSN
// 的报错发生在其后的 db.Exec（连接触发 parse），走的是 CREATE 失败一支。
//
// L95-97（内存 sqlite gorm.Open 失败）：DSN 是硬编码的合法值，无注入缝隙
// 【已加 C 类论证注释】。
// postgres/mysql/sqlserver 的 dsnWithoutDB=="" 兜底【已删】：removeDB*/
// replaceDB* 对非空输入恒非空（等值替换或原样返回）。
// L230-232（openReadOnlyGorm 的 PRAGMA 失败）：gorm.Open 阶段已完成连接
// 校验，成功后同一条 `PRAGMA query_only = ON` 无确定性失败输入【已加
// C 类论证注释】。

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
// game_seed.go L245-247（buildGameFromSeed 的 SetEnvs 错误）【已删】：
// model.Game.SetEnvs 只做 json.Marshal([]GameEnv)，字段全为 string，
// 不存在可构造的失败输入。
// game_seed.go L321-322（humanizeGameID 的空 token continue）【已删】：
// strings.Fields 不会产出空串 token。
// game_seed.go resolveGamesConfigPath 的 base=="" 兜底 ×2【已删】：
// resolveBootstrapBaseDir 三个出口全部非空。
//
// migrations.go L87-88（init 的 goose 注册失败 panic）：init 在包加载时
// 以固定的合法迁移列表执行且仅执行一次，无测试注入点【已加 C 类论证
// 注释】。
//
// ops_state_store.go L266-269（cloneOpsState 的 Unmarshal 失败）【已删】：
// 输入是同函数内刚 json.Marshal 成功的产物，结构体各字段均为静态类型，
// 可无损往返。
//
// service_context.go：
//   - L279-283（audit.NewSQLAuditStore 错误）【已删】：该构造函数仅对
//     nil db 报错，而此处 db 来自 openDatabase（失败早在开头 panic）。
//   - L470-472 / L476-478 / L479-481（seedBootstrapPermissions/Roles/
//     Admins 错误日志）【已删】：三个函数所有路径都 return nil。
//   - L482-484（seedBootstrapGames 错误日志）【已补测试，见下方
//     TestNewServiceContext_SeedGamesCountFailureG】：唯一错误源是启动
//     中段 games 表 Count 失败；破坏库形态会先在 BackfillEnvBindings
//     panic，但 Option 注入点位于两者之间，可注册查询回调定点注错。
//   - L512-513（JWT secret 错误后的开发态兜底）【已删】：jwtutil.
//     ResolveSecret 的 dev 判定与 svc.isDevelopmentConfig 逐条等价，
//     err!=nil 时必然判为非开发态而走 panic，永不到达兜底行。
//   - L725-727（resolveBootstrapAuthDir 空 base 兜底）【已删】：
//     resolveBootstrapBaseDir 末端恒返回 runtime.DefaultBootstrapDataDir()
//     的非空结果。
//   - L1004-1006（derivePermissionResourceAction 的空 action 兜底）
//     【已删】：action 初值 "*"，splitPermissionCode 返回的 action 恒非空。
//   - L1228-1229（initObjectStore 未知驱动）：objstore.Validate 的
//     default 分支先于 switch 拒绝未知驱动【已加 C 类论证注释】。

// seedBootstrapGames 的 count games 失败（NewServiceContext 内的 error 日志
// 分支）：BackfillEnvBindings 在迁移后立即执行且对库形态敏感（坏表直接
// panic），无法用 DROP TABLE/视图注入；Option 的应用点位于
// BackfillEnvBindings 之后、seed 之前，恰好可以注册查询回调把 games 表的
// count(*) 定点替换为失败——普通 SELECT 不受影响，启动链其余部分照常。
func TestNewServiceContext_SeedGamesCountFailureG(t *testing.T) {
	cfg := newSvcConfig(t, false)
	opt := func(sc *ServiceContext) {
		require.NotNil(t, sc.DB)
		require.NoError(t, sc.DB.Callback().Query().After("gorm:query").Register("g:games_count_boom", func(tx *gorm.DB) {
			if tx.Statement == nil || tx.Statement.Table != "games" {
				return
			}
			// Statement.SQL 在 gorm:query 的 BuildQuerySQL 阶段填充，
			// 用语句文本区分 count 与 Find，避免误伤其他 games 查询
			//（大小写不敏感，防 gorm 升级改写 count 大小写）。
			if strings.Contains(strings.ToLower(tx.Statement.SQL.String()), "count(*)") {
				tx.Error = errors.New("injected games count failure")
			}
		}))
	}

	ctx := NewServiceContext(cfg, opt)
	require.NotNil(t, ctx)

	// count 失败使 seedBootstrapGames 提前返回：默认引导 game 未创建。
	games, err := ctx.GameModel.ListAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, games, "count 注错后 seed 应中断，不应创建引导 game")
}
