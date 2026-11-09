package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	gmysql "gorm.io/driver/mysql"
	gpostgres "gorm.io/driver/postgres"
	gsqlserver "gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openapiBackfillMigration() *goose.Migration {
	return goose.NewGoMigration(2,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			return model.MigrateFunctionOpenAPIColumns(db)
		}},
		nil,
	)
}

func legacyCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(3,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if err := model.RenameLegacyTables(db); err != nil {
				return err
			}
			if err := model.DropLegacyPageUniqueIndexes(db); err != nil {
				return err
			}
			return model.CleanupAllLegacy(db)
		}},
		nil,
	)
}

func enumColumnsMigration() *goose.Migration {
	return goose.NewGoMigration(4,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			return model.MigrateEnumColumns(db)
		}},
		nil,
	)
}

// supportContextColumns lists the game-support P1 columns added by migration
// 0005 (table model → column). GORM's AddColumn(model, columnName) emits the
// dialect-appropriate DDL from the model tags; HasColumn keeps it idempotent.
var supportContextColumns = []struct {
	model  func() interface{}
	column string
}{
	{func() interface{} { return &model.FAQ{} }, "slug"},
	{func() interface{} { return &model.FAQ{} }, "summary"},
	{func() interface{} { return &model.FAQ{} }, "helpful_count"},
	{func() interface{} { return &model.FAQ{} }, "unhelpful_count"},
	{func() interface{} { return &model.Ticket{} }, "server_id"},
	{func() interface{} { return &model.Ticket{} }, "player_level"},
	{func() interface{} { return &model.Ticket{} }, "device_os"},
	{func() interface{} { return &model.Ticket{} }, "device_model"},
	{func() interface{} { return &model.Ticket{} }, "language"},
	{func() interface{} { return &model.Ticket{} }, "extra"},
}

func supportContextMigration() *goose.Migration {
	return goose.NewGoMigration(5,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			for _, col := range supportContextColumns {
				mdl := col.model()
				table := mdl.(interface{ TableName() string }).TableName()
				if migrator.HasTable(table) && !migrator.HasColumn(mdl, col.column) {
					if err := migrator.AddColumn(mdl, col.column); err != nil {
						return fmt.Errorf("migrate: 0005 add %s.%s: %w", table, col.column, err)
					}
				}
			}
			return nil
		}},
		nil,
	)
}

// bugTrackerMigration creates the bugs table for the defect tracker
// (0006). HasTable keeps it idempotent; fresh databases already get the
// table from the AutoMigrate baseline.
func bugTrackerMigration() *goose.Migration {
	return goose.NewGoMigration(6,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.Bug{}) {
				if err := db.Migrator().CreateTable(&model.Bug{}); err != nil {
					return fmt.Errorf("migrate: 0006 create bugs: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// agentSessionAddrMigration 为存量库补 agent_sessions.addr 列
// （0015）：HA 跨实例节点视图的 IP 依赖该列；新库由 baseline AutoMigrate
// 直接带出。幂等：列已存在则跳过。
func agentSessionAddrMigration() *goose.Migration {
	return goose.NewGoMigration(15,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			// agent_sessions 是 meta/single 库的表——fanout 会在每个 game 库
			// 重放编号迁移，表不存在的库直接跳过（AddColumn 会凭空建出
			// 空壳表，破坏后续 enum 迁移的表结构预期）。
			if !db.Migrator().HasTable(&reg.AgentSessionDB{}) {
				return nil
			}
			if db.Migrator().HasColumn(&reg.AgentSessionDB{}, "Addr") {
				return nil
			}
			if err := db.Migrator().AddColumn(&reg.AgentSessionDB{}, "Addr"); err != nil {
				return fmt.Errorf("migrate: 0015 add agent_sessions.addr: %w", err)
			}
			return nil
		}},
		nil,
	)
}

// contractTimeoutMigration 为存量库补 function_contracts.timeout_ms 列
// （0016）：声明式同步调用预算的契约存储；新库由 baseline AutoMigrate 带
// 出。幂等：列已存在则跳过。
func contractTimeoutMigration() *goose.Migration {
	return goose.NewGoMigration(16,
		&goose.GoFunc{RunDB: addContractTimeoutColumn},
		nil,
	)
}

// contractPrevSchemaMigration 为存量库补 function_contracts 的 prev
// schema 两列（0021）：sync-selectors 用 prev→new 的字段 diff 做 rename
// 精确推断。新库由 baseline AutoMigrate 带出。幂等：列已存在则跳过。
func contractPrevSchemaMigration() *goose.Migration {
	return goose.NewGoMigration(21,
		&goose.GoFunc{RunDB: addContractPrevSchemaColumns},
		nil,
	)
}

// addContractPrevSchemaColumns 是 0021 的迁移体（抽出便于直测）：
// 存量库补 prev_input_schema/prev_output_schema；幂等；缺表跳过。
func addContractPrevSchemaColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionContract{}) {
		return nil
	}
	for _, col := range []string{"PrevInputSchema", "PrevOutputSchema"} {
		if db.Migrator().HasColumn(&model.FunctionContract{}, col) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.FunctionContract{}, col); err != nil {
			return fmt.Errorf("migrate: 0021 add function_contracts.%s: %w", col, err)
		}
	}
	return nil
}

// termDictionaryDisplayMigration 为存量库补 term_dictionary.display JSON
// 列并回填旧双列（0022）。MigrateTermDictionaryDisplay 不建 display 列
// （假设 AutoMigrate 已带出），存量库过 baseline 后不再跑 AutoMigrate，
// 因此先 AddColumn 再复用同一回填实现——与 baseline 路径共享实现，两个
// 路径不会漂移。幂等；缺表跳过。
func termDictionaryDisplayMigration() *goose.Migration {
	return goose.NewGoMigration(22,
		&goose.GoFunc{RunDB: migrateTermDictionaryDisplayColumn},
		nil,
	)
}

// migrateTermDictionaryDisplayColumn 是 0022 的迁移体（抽出便于直测）。
func migrateTermDictionaryDisplayColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.TermDictionary{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.TermDictionary{}, "Display") {
		if err := db.Migrator().AddColumn(&model.TermDictionary{}, "Display"); err != nil {
			return fmt.Errorf("migrate: 0022 add term_dictionary.display: %w", err)
		}
	}
	return model.MigrateTermDictionaryDisplay(db)
}

// componentTemplateColumnsMigration 为存量 game 库补 component_templates 的
// params（U6 模板参数化）与 digest（U11 更新提醒）列（0023）。两列加入模型时
// 均未随版本化迁移发布，存量库过 baseline 后不再跑 AutoMigrate——模板创建/
// 更新持续报 column "params" does not exist。逐列 AddColumn（同 0015/0016/
// 0021 模式），**不可**对整模型 AutoMigrate：存量库 key 列的唯一约束名是建表
// 期老写法（component_templates_key_key），与模型 uniqueIndex 默认名
// （uni_component_templates_key）不一致，AutoMigrate 的索引对齐在 postgres
// 上报 constraint "uni_component_templates_key" does not exist 直接 panic。
func componentTemplateColumnsMigration() *goose.Migration {
	return goose.NewGoMigration(23,
		&goose.GoFunc{RunDB: migrateComponentTemplateColumns},
		nil,
	)
}

// migrateComponentTemplateColumns 是 0023 的迁移体（抽出便于直测）：
// 存量库补 params/digest 两列；幂等；缺表跳过。
func migrateComponentTemplateColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.ComponentTemplate{}) {
		return nil
	}
	for _, col := range []string{"Params", "Digest"} {
		if db.Migrator().HasColumn(&model.ComponentTemplate{}, col) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.ComponentTemplate{}, col); err != nil {
			return fmt.Errorf("migrate: 0023 add component_templates.%s: %w", col, err)
		}
	}
	return nil
}

// contractExecutionStateMigration 为存量库补 function_contracts.execution_state
// 列（0025，D2/T3 契约与绑定正交化）：bound=可执行、unbound=纯物料。列带
// DEFAULT 'bound'，存量行随 ALTER TABLE 直接回填 bound（postgres/mysql 均如此，
// sqlite 亦支持常量默认值回填），行为与迁移前完全一致。新库由 baseline
// AutoMigrate 带出。逐列 AddColumn（0015/0016/0021/0023 同模式）；幂等；
// 缺表跳过。
func contractExecutionStateMigration() *goose.Migration {
	return goose.NewGoMigration(25,
		&goose.GoFunc{RunDB: addContractExecutionStateColumn},
		nil,
	)
}

// addContractExecutionStateColumn 是 0025 的迁移体（抽出便于直测）。
func addContractExecutionStateColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionContract{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.FunctionContract{}, "ExecutionState") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.FunctionContract{}, "ExecutionState"); err != nil {
		return fmt.Errorf("migrate: 0025 add function_contracts.execution_state: %w", err)
	}
	return nil
}

// sdkVersionHighwatermarkMigration 为 0028：SDK 滑动版本门槛的高水位表。
// 新表 CreateTable（索引随建表一次建出，无存量约束名漂移——0023 教训只
// 针对改既有表），幂等：已存在时跳过。
func sdkVersionHighwatermarkMigration() *goose.Migration {
	return goose.NewGoMigration(28,
		&goose.GoFunc{RunDB: migrateSDKVersionHighwatermark},
		nil,
	)
}

// migrateSDKVersionHighwatermark 是 0028 的迁移体（抽出便于直测）。
func migrateSDKVersionHighwatermark(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.SDKVersionHighwatermark{}) {
		if err := db.Migrator().CreateTable(&model.SDKVersionHighwatermark{}); err != nil {
			return fmt.Errorf("migrate: 0028 create sdk_version_highwatermarks: %w", err)
		}
	}
	return nil
}

// contractVersionTableMigration 为 0029：B2 函数契约变更历史表。
// 新表 CreateTable（索引随建表一次建出，无存量约束名漂移——0028 同模式），
// 幂等：已存在时跳过。
func contractVersionTableMigration() *goose.Migration {
	return goose.NewGoMigration(29,
		&goose.GoFunc{RunDB: migrateFunctionContractVersionTable},
		nil,
	)
}

// migrateFunctionContractVersionTable 是 0029 的迁移体（抽出便于直测）。
func migrateFunctionContractVersionTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionContractVersion{}) {
		if err := db.Migrator().CreateTable(&model.FunctionContractVersion{}); err != nil {
			return fmt.Errorf("migrate: 0029 create function_contract_versions: %w", err)
		}
	}
	return nil
}

// functionVersionFloorTableMigration 为 0030：函数级最低 SDK 版本门槛表
// （平台侧设置独立于注册物化，不随描述符回写被冲）。新表 CreateTable
// （索引随建表一次建出，无存量约束名漂移——0028/0029 同模式），幂等：
// 已存在时跳过。
func functionVersionFloorTableMigration() *goose.Migration {
	return goose.NewGoMigration(30,
		&goose.GoFunc{RunDB: migrateFunctionVersionFloorTable},
		nil,
	)
}

// migrateFunctionVersionFloorTable 是 0030 的迁移体（抽出便于直测）。
func migrateFunctionVersionFloorTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionVersionFloor{}) {
		if err := db.Migrator().CreateTable(&model.FunctionVersionFloor{}); err != nil {
			return fmt.Errorf("migrate: 0030 create function_version_floors: %w", err)
		}
	}
	return nil
}

// providerMetadataTableMigration 为 0034：provider 自报元数据 EAV 表
// （OPEN-ISSUES #11）。元数据此前纯内存态（agent agentlocal + server
// registry 快照，重启即失），落库后重启可查、聚合接口有源。新表
// CreateTable（索引随建表一次建出，无存量约束名漂移——0023 教训只针对
// 改既有表），幂等：已存在时跳过。
func providerMetadataTableMigration() *goose.Migration {
	return goose.NewGoMigration(34,
		&goose.GoFunc{RunDB: migrateProviderMetadataTable},
		nil,
	)
}

// migrateProviderMetadataTable 是 0034 的迁移体（抽出便于直测）。
func migrateProviderMetadataTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.ProviderMetadata{}) {
		if err := db.Migrator().CreateTable(&model.ProviderMetadata{}); err != nil {
			return fmt.Errorf("migrate: 0034 create provider_metadata: %w", err)
		}
	}
	return nil
}

// bugTicketLinkMigration 为 0035：建 bug_ticket_links 表（bug↔工单多对多
// 关联，#25）。新表，无存量数据约束；幂等（表已存在则跳过）。
// 与 0028/0030/0032 同模式：不整模型 AutoMigrate——存量库上会与既有约束
// 名漂移 panic（0023 教训）。0034 已被 provider_metadata 占用故顺延。
func bugTicketLinkMigration() *goose.Migration {
	return goose.NewGoMigration(35,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if migrator.HasTable(&model.BugTicketLink{}) {
				return nil
			}
			if err := migrator.CreateTable(&model.BugTicketLink{}); err != nil {
				return fmt.Errorf("migrate: 0035 create bug_ticket_links: %w", err)
			}
			return nil
		}},
		nil,
	)
}

// contractRemovalPendingColumnMigration 为 0031：function_contracts 加
// removal_pending_at 列 + 清扫候选过滤索引（摘除宽限——瞬态摘除不立即
// 真删，过期清扫）。改既有表走 HasColumn+AddColumn 逐列补齐（0021/0025
// 同模式；整模型 AutoMigrate 会与存量约束名漂移 panic——0023 教训），
// 幂等：列/索引已存在跳过。
func contractRemovalPendingColumnMigration() *goose.Migration {
	return goose.NewGoMigration(31,
		&goose.GoFunc{RunDB: migrateContractRemovalPendingColumn},
		nil,
	)
}

// migrateContractRemovalPendingColumn 是 0031 的迁移体（抽出便于直测）。
func migrateContractRemovalPendingColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionContract{}) {
		return nil // 无该表的库（fanout 重放）跳过，不建空壳
	}
	if !db.Migrator().HasColumn(&model.FunctionContract{}, "RemovalPendingAt") {
		if err := db.Migrator().AddColumn(&model.FunctionContract{}, "RemovalPendingAt"); err != nil {
			return fmt.Errorf("migrate: 0031 add function_contracts.removal_pending_at: %w", err)
		}
	}
	// 清扫循环每 5 分钟按 removal_pending_at IS NOT NULL 过滤候选，
	// 契约表大后全表扫放大；普通（非部分）索引保证三方言通用。
	if !db.Migrator().HasIndex(&model.FunctionContract{}, "idx_function_contracts_removal_pending") {
		if err := db.Migrator().CreateIndex(&model.FunctionContract{}, "idx_function_contracts_removal_pending"); err != nil {
			return fmt.Errorf("migrate: 0031 index function_contracts.removal_pending_at: %w", err)
		}
	}
	return nil
}

// dbSourceMigration creates the db_sources registry table (0012).
func dbSourceMigration() *goose.Migration {
	return goose.NewGoMigration(12,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.DBSource{}) {
				if err := db.Migrator().CreateTable(&model.DBSource{}); err != nil {
					return fmt.Errorf("migrate: 0012 create db_sources: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// hotpatchMigration creates the hotpatches table (0011).
func hotpatchMigration() *goose.Migration {
	return goose.NewGoMigration(11,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.Hotpatch{}) {
				if err := db.Migrator().CreateTable(&model.Hotpatch{}); err != nil {
					return fmt.Errorf("migrate: 0011 create hotpatches: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// configNamespaceMigration adds the namespace column to config_versions
// (0009) and backfills existing rows to the runtime default.
func configNamespaceMigration() *goose.Migration {
	return goose.NewGoMigration(9,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if migrator.HasTable(&model.ConfigVersion{}) && !migrator.HasColumn(&model.ConfigVersion{}, "namespace") {
				if err := migrator.AddColumn(&model.ConfigVersion{}, "namespace"); err != nil {
					return fmt.Errorf("migrate: 0009 add config_versions.namespace: %w", err)
				}
				if err := db.Model(&model.ConfigVersion{}).
					Where("namespace IS NULL OR namespace = ''").
					Update("namespace", model.ConfigNamespaceDefault).Error; err != nil {
					return fmt.Errorf("migrate: 0009 backfill namespace: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// toolRegistryMigration creates the tool_links table (0007, toolbox).
func toolRegistryMigration() *goose.Migration {
	return goose.NewGoMigration(7,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.ToolLink{}) {
				if err := db.Migrator().CreateTable(&model.ToolLink{}); err != nil {
					return fmt.Errorf("migrate: 0007 create tool_links: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// wrapGorm builds a *gorm.DB on top of the *sql.DB goose hands to Go
// migrations. The dialect is probed with cheap SQL pings because database/sql
// does not expose the driver name.
func wrapGorm(sqlDB *sql.DB) (*gorm.DB, error) {
	dialect, err := probeDialect(sqlDB)
	if err != nil {
		return nil, err
	}
	var dialector gorm.Dialector
	switch dialect {
	case "mysql":
		dialector = gmysql.New(gmysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true})
	case "postgres":
		dialector = gpostgres.New(gpostgres.Config{Conn: sqlDB})
	case "mssql":
		dialector = gsqlserver.New(gsqlserver.Config{Conn: sqlDB})
	default:
		dialector = gsqlite.Dialector{Conn: sqlDB}
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("svc: wrap gorm for migration: %w", err)
	}
	return db, nil
}

func probeDialect(sqlDB *sql.DB) (string, error) {
	var n int
	// SQLite: sqlite_master exists only on SQLite (COUNT works on empty DBs).
	if err := sqlDB.QueryRow("SELECT COUNT(1) FROM sqlite_master").Scan(&n); err == nil {
		return "sqlite", nil
	}
	// Postgres: CURRENT_SETTING is not a MySQL function.
	var s string
	if err := sqlDB.QueryRow("SELECT CURRENT_SETTING('server_version')").Scan(&s); err == nil {
		return "postgres", nil
	}
	// MySQL: server-side version comment.
	if err := sqlDB.QueryRow("SELECT @@version_comment LIMIT 1").Scan(&s); err == nil {
		return "mysql", nil
	}
	// SQL Server: @@VERSION 是 T-SQL 独有（MySQL 用 @@version_comment，
	// postgres 无系统变量语法，二者已在前面试过）。
	if err := sqlDB.QueryRow("SELECT @@VERSION").Scan(&s); err == nil {
		return "mssql", nil
	}
	return "", fmt.Errorf("svc: probe dialect: unsupported database")
}

// dbSourceOwnerMigration 为 0045：db_sources 加 owner_agent_id 列（R4：DB 源
// 周期探测下放归属 agent，M3 必须能把 DB 源指到具体 agent；空 = 未归属，
// server 诊断端点兜底）。HasTable 跳过、HasColumn 幂等、存量行不回填
// （历史源无归属=未归属，诚实留空）。不加索引：表小（注册源数十级）、
// 归属查询按 ID 主键过滤后行扫，不为它引入存量索引漂移面。
func dbSourceOwnerMigration() *goose.Migration {
	return goose.NewGoMigration(45,
		&goose.GoFunc{RunDB: migrateDBSourceOwnerColumn},
		nil,
	)
}

func migrateDBSourceOwnerColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.DBSource{}) {
		return nil
	}
	if migrator.HasColumn(&model.DBSource{}, "OwnerAgentID") {
		return nil
	}
	if err := migrator.AddColumn(&model.DBSource{}, "OwnerAgentID"); err != nil {
		return fmt.Errorf("migrate: 0045 add db_sources.owner_agent_id: %w", err)
	}
	return nil
}
