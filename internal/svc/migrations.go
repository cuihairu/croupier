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

// This file turns the legacy compensation hooks (see
// docs/architecture/database-migration-strategy.md, phase 3) into numbered
// goose Go migrations. They reuse the exact same implementations as the
// baseline bridge in internal/model so the two paths can never drift.
//
// Version layout:
//   0001 (SQL)  baseline marker
//   0002 (Go)   functions openapi column backfill
//   0003 (Go)   legacy table/index/column cleanup
//   0004 (Go)   varchar→int enum column conversion
//   0005 (Go)   game-support context columns (docs/research/game-support-systems.md)
//   0006 (Go)   bug tracker baseline table (docs/research/bug-tracking-design.md)
//   0007 (Go)   tool registry baseline table (docs/research/tool-registry-design.md)
//   0008 (Go)   game release baseline table (docs/research/release-management-design.md)
//   0009 (Go)   config namespace column (docs/research/config-hot-reload-design.md)
//   0010 (Go)   ticket CSAT columns (docs/research/game-support-systems.md P2)
//   0011 (Go)   hotpatch baseline table (docs/research/hot-patch-design.md)
//   0012 (Go)   db source registry table (docs/research/db-monitoring-design.md)
//   0013 (Go)   platform settings table (docs/architecture/config-layering.md)
//   0015 (Go)   agent_sessions.addr column (HA 跨实例节点视图的 IP 展示)
//   0014 (Go)   task schedules tables (docs: cron 调度 task_schedules/run_logs)
//   0016 (Go)   function_contracts.timeout_ms column（声明式超时执行层接线）
//   0017 (Go)   admins 登录安全列（failed_attempts/locked_until/token_version，
//               见 todo.md T1/T2：登录失败锁定 + token 撤销）
//   0019 (Go)   announcements + announcement_reads（公告系统：markdown 公告 + 弹窗确认）
//   0018 (Go)   admins.otp_enabled 列（T3：MFA 按 provider 接线，local 账号
//               可启用 TOTP；ldap/oidc 登录跳过平台 MFA）
//   0020 (Go)   execution_logs 表（R1 执行留痕：payload 级请求/响应落库）
//   0021 (Go)   function_contracts.prev_input/output_schema 列（selector
//               一键同步的 prev schema 存储；sync-selectors 上线时漏配
//               存量库迁移，agent 注册 upsert 直接 SQL 报错）
//   0022 (Go)   term_dictionary.display JSON 列（57fac95df 双列→JSON 重构
//               只改了模型，存量库从未跑过 AutoMigrate，seed 持续报
//               column "display" does not exist）
//   0023 (Go)   component_templates.params/digest 列（U6 模板参数化加 Params、
//               U11 更新提醒加 Digest 时均只改了模型，存量 game 库过 baseline
//               后不再跑 AutoMigrate，创建/更新模板持续报 column "params"
//               does not exist）
//   0024 (Go)   软删除残留行清理（删除路径改硬删后，把 page_specs/
//               page_proposals/component_templates/openapi_source_bindings/
//               registration_warnings 里已软删的存量行物理清除——它们占着
//               物理唯一索引，同 key 重建 500）
//   0025 (Go)   function_contracts.execution_state 列（D2/T3：契约执行
//               状态 bound/unbound，存量行默认 bound，行为与现状一致）
//   0026 (Go)   roles/admins 软删除残留行清理（0024 同族漏网表：删除路径
//               改硬删后物理清除已软删的存量行——roles.name 与
//               admins.username 的物理唯一索引被软删行占位，同名重建 500）
//   0027 (Go)   menu_items 表 + page_specs.menu_id 列（T-M1/T-M4 菜单系统
//               落地时只改了模型，存量库过 baseline 后不再跑 AutoMigrate：
//               menus API 因表缺失 500，页面保存/发布链因 GORM 全字段
//               INSERT 报 column "menu_id" does not exist 中断——0021/0023
//               同族「模型改了迁移漏配」事故）
//   0028 (Go)   sdk_version_highwatermarks 表（SDK 滑动版本门槛的高水位
//               存储：per (game_id, env, sdk_language) 记见过的最高版本；
//               新建表无存量约束名漂移，0014/0027 建表同模式）
//   0029 (Go)   function_contract_versions 表（B2 函数契约变更历史：
//               per (game_id, env, function_id) 的内容变化快照流；
//               新建表无存量约束名漂移，0028 同模式）

func init() {
	registerSvcMigrations()
}

// registerSvcMigrations 注册本包的 goose 全局迁移。抽为具名函数是为可测性：
// init 在包加载时仅执行一次，测试经 goose.ResetGlobalMigrations 清空后
// 重新调用，可覆盖重复注册的 panic 分支（见 migrations_register_test.go）。
// panic 是 Go init 对注册失败的唯一表达方式，保留 fail-fast。
func registerSvcMigrations() {
	if err := goose.SetGlobalMigrations(
		openapiBackfillMigration(),
		legacyCleanupMigration(),
		enumColumnsMigration(),
		supportContextMigration(),
		bugTrackerMigration(),
		toolRegistryMigration(),
		releaseMigration(),
		configNamespaceMigration(),
		ticketCSATMigration(),
		hotpatchMigration(),
		dbSourceMigration(),
		platformSettingsMigration(),
		taskSchedulesMigration(),
		agentSessionAddrMigration(),
		contractTimeoutMigration(),
		announcementTablesMigration(),
		adminLoginSecurityMigration(),
		adminMfaMigration(),
		executionLogsTableMigration(),
		contractPrevSchemaMigration(),
		termDictionaryDisplayMigration(),
		componentTemplateColumnsMigration(),
		softDeleteResidueCleanupMigration(),
		contractExecutionStateMigration(),
		roleAdminSoftDeleteCleanupMigration(),
		menuItemTablesMigration(),
		sdkVersionHighwatermarkMigration(),
		contractVersionTableMigration(),
	); err != nil {
		panic(fmt.Sprintf("svc: register goose go migrations: %v", err))
	}
}

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

// announcementTablesMigration 为存量库补公告两表（0019）；新库由
// baseline AutoMigrate 带出。幂等：表已存在则跳过。
func announcementTablesMigration() *goose.Migration {
	return goose.NewGoMigration(19,
		&goose.GoFunc{RunDB: createAnnouncementTables},
		nil,
	)
}

func createAnnouncementTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	for _, m := range []interface{}{&model.Announcement{}, &model.AnnouncementRead{}} {
		if db.Migrator().HasTable(m) {
			continue
		}
		if err := db.Migrator().CreateTable(m); err != nil {
			return fmt.Errorf("migrate: 0019 create announcement tables: %w", err)
		}
	}
	return nil
}

// addContractTimeoutColumn 是 0016 的迁移体（抽出便于直测）：
// 存量库补 function_contracts.timeout_ms；幂等；缺表跳过。
func addContractTimeoutColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.FunctionContract{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.FunctionContract{}, "TimeoutMs") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.FunctionContract{}, "TimeoutMs"); err != nil {
		return fmt.Errorf("migrate: 0016 add function_contracts.timeout_ms: %w", err)
	}
	return nil
}

// adminLoginSecurityMigration 为存量库补 admins 登录安全列（0017）：
// failed_attempts/locked_until（T1 登录失败锁定）与 token_version
// （T2 token 撤销）一次补齐，避免两条迁移改同一张表。新库由 baseline
// AutoMigrate 直接带出；幂等：列已存在则跳过。admins 是 meta 库表，
// fanout 重放到 game 库时表不存在则直接跳过（与 0015 同理）。
func adminLoginSecurityMigration() *goose.Migration {
	return goose.NewGoMigration(17,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if !migrator.HasTable(&model.Admin{}) {
				return nil
			}
			for _, col := range []string{"FailedAttempts", "LockedUntil", "TokenVersion"} {
				if migrator.HasColumn(&model.Admin{}, col) {
					continue
				}
				if err := migrator.AddColumn(&model.Admin{}, col); err != nil {
					return fmt.Errorf("migrate: 0017 add admins.%s: %w", col, err)
				}
			}
			return nil
		}},
		nil,
	)
}

// adminMfaMigration 为存量库补 admins.otp_enabled 列（0018）；新库由
// baseline AutoMigrate 带出；幂等；game 库无 admins 表直接跳过。
func adminMfaMigration() *goose.Migration {
	return goose.NewGoMigration(18,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			if !migrator.HasTable(&model.Admin{}) {
				return nil
			}
			if migrator.HasColumn(&model.Admin{}, "OTPEnabled") {
				return nil
			}
			if err := migrator.AddColumn(&model.Admin{}, "OTPEnabled"); err != nil {
				return fmt.Errorf("migrate: 0018 add admins.otp_enabled: %w", err)
			}
			return nil
		}},
		nil,
	)
}

// executionLogsTableMigration creates the execution_logs table (0020, R1
// 执行留痕)：payload 级请求/响应落库。新库由 baseline AutoMigrate（已注册
// GameModels）带出；存量库经本迁移补齐。幂等：表已存在则跳过。
// 单库/多游戏两种模式都会在对应 scope 上执行本迁移。
func executionLogsTableMigration() *goose.Migration {
	return goose.NewGoMigration(20,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if db.Migrator().HasTable(&model.ExecutionLog{}) {
				return nil
			}
			if err := db.Migrator().CreateTable(&model.ExecutionLog{}); err != nil {
				return fmt.Errorf("migrate: 0020 create execution_logs: %w", err)
			}
			return nil
		}},
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

// softDeleteResidueCleanupMigration 为 0024：把删除路径改硬删（Unscoped）之前
// 遗留的软删行物理清除。这些表的唯一索引是物理索引，软删行占着索引位会导致
// 同 key 重建直接 duplicate-key 500（线上实证：page_specs id=60、
// component_templates 5 行）。纯 DELETE，不动表结构，幂等；缺表跳过。
// 注意 published_page_specs / page_versions 无 DeletedAt 列（一直就是硬删），
// 不在本清单内。
func softDeleteResidueCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(24,
		&goose.GoFunc{RunDB: migrateSoftDeleteResidue},
		nil,
	)
}

// migrateSoftDeleteResidue 是 0024 的迁移体（抽出便于直测）：
// 逐表 DELETE deleted_at IS NOT NULL 的残留行。
func migrateSoftDeleteResidue(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	// 迁移只允许逐操作 DELETE（同 0023 的教训：整模型 AutoMigrate 在存量
	// postgres 上因约束名漂移 panic）。表或 deleted_at 列缺失说明该库不含
	// 此模型的软删形态，跳过。
	targets := []struct {
		table string
		model interface{}
	}{
		{"page_specs", &model.PageSpec{}},
		{"page_proposals", &model.PageProposal{}},
		{"component_templates", &model.ComponentTemplate{}},
		{"openapi_source_bindings", &model.OpenAPISourceBinding{}},
		{"registration_warnings", &model.RegistrationWarningDB{}},
	}
	for _, target := range targets {
		if !db.Migrator().HasTable(target.model) {
			continue
		}
		if !db.Migrator().HasColumn(target.model, "DeletedAt") {
			continue
		}
		if err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE deleted_at IS NOT NULL", target.table)).Error; err != nil {
			return fmt.Errorf("migrate: 0024 purge soft-deleted rows from %s: %w", target.table, err)
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

// roleAdminSoftDeleteCleanupMigration 为 0026：roles/admins 删除路径改硬删
// （Unscoped）之后，把 0024 同族漏网的存量软删行物理清除。roles.name 与
// admins.username 是物理唯一索引，软删行占着索引位会导致同名重建直接
// duplicate-key 500（T-M9 菜单 E2E 实证：删除角色后重建报 UNIQUE
// constraint failed: roles.name）。纯 DELETE，不动表结构，幂等；缺表/缺列跳过。
// admin_roles 里指向被清行的悬挂关联一并清除（软删时代就存在，借迁移收口）。
func roleAdminSoftDeleteCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(26,
		&goose.GoFunc{RunDB: migrateRoleAdminSoftDelete},
		nil,
	)
}

// migrateRoleAdminSoftDelete 是 0026 的迁移体（抽出便于直测）。
func migrateRoleAdminSoftDelete(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	targets := []struct {
		table string
		model interface{}
	}{
		{"roles", &model.Role{}},
		{"admins", &model.Admin{}},
	}
	for _, target := range targets {
		if !db.Migrator().HasTable(target.model) {
			continue
		}
		if !db.Migrator().HasColumn(target.model, "DeletedAt") {
			continue
		}
		if err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE deleted_at IS NOT NULL", target.table)).Error; err != nil {
			return fmt.Errorf("migrate: 0026 purge soft-deleted rows from %s: %w", target.table, err)
		}
	}
	if db.Migrator().HasTable(&model.AdminRole{}) {
		if err := db.Exec(`DELETE FROM admin_roles WHERE role_id NOT IN (SELECT id FROM roles)`).Error; err != nil {
			return fmt.Errorf("migrate: 0026 purge dangling admin_roles: %w", err)
		}
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

// menuItemTablesMigration 为 0027：菜单管理系统（T-M1/T-M4）落地时只改了
// 模型（MenuItem 表 + PageSpec.MenuID 列），存量库过 baseline 后不再跑
// AutoMigrate，缺两样：menu_items 表（menus API 全部 500）与
// page_specs.menu_id 列（GORM 全字段 INSERT 直接 SQLSTATE 42703，页面
// 保存/发布链中断）。逐项幂等补齐：缺表 CreateTable（新建表，索引随建表
// 一次建出，无存量约束名漂移问题——0023 教训只针对改既有表）、缺列
// AddColumn（0015/0016/0021/0023/0025 同模式，不动既有约束）。menu_id 的
// 普通索引不补：级联清理查询低频且 page_specs 行数量级小，全表扫无感知；
// 避免 CreateIndex 与存量索引名对齐引入新风险面。
func menuItemTablesMigration() *goose.Migration {
	return goose.NewGoMigration(27,
		&goose.GoFunc{RunDB: migrateMenuItemTables},
		nil,
	)
}

// migrateMenuItemTables 是 0027 的迁移体（抽出便于直测）。
func migrateMenuItemTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.MenuItem{}) {
		if err := db.Migrator().CreateTable(&model.MenuItem{}); err != nil {
			return fmt.Errorf("migrate: 0027 create menu_items: %w", err)
		}
	}
	if db.Migrator().HasTable(&model.PageSpec{}) {
		if !db.Migrator().HasColumn(&model.PageSpec{}, "MenuID") {
			if err := db.Migrator().AddColumn(&model.PageSpec{}, "MenuID"); err != nil {
				return fmt.Errorf("migrate: 0027 add page_specs.menu_id: %w", err)
			}
		}
	}
	return nil
}

// taskSchedulesMigration creates the cron scheduling tables (0014):
// task_schedules + task_schedule_run_logs. 表随 6aba002b6 加入
// MetaModels，但已过 baseline 的部署库不会再跑 AutoMigrate——没有这条
// 迁移时 GET /api/v1/schedules 因表缺失 500。
func taskSchedulesMigration() *goose.Migration {
	return goose.NewGoMigration(14,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.TaskSchedule{}) {
				if err := db.Migrator().CreateTable(&model.TaskSchedule{}); err != nil {
					return fmt.Errorf("migrate: 0014 create task_schedules: %w", err)
				}
			}
			if !db.Migrator().HasTable(&model.TaskScheduleRunLog{}) {
				if err := db.Migrator().CreateTable(&model.TaskScheduleRunLog{}); err != nil {
					return fmt.Errorf("migrate: 0014 create task_schedule_run_logs: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
}

// platformSettingsMigration creates the platform_settings L3 table (0013).
func platformSettingsMigration() *goose.Migration {
	return goose.NewGoMigration(13,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.PlatformSetting{}) {
				if err := db.Migrator().CreateTable(&model.PlatformSetting{}); err != nil {
					return fmt.Errorf("migrate: 0013 create platform_settings: %w", err)
				}
			}
			return nil
		}},
		nil,
	)
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

// ticketCSATMigration adds the CSAT columns to tickets (0010).
func ticketCSATMigration() *goose.Migration {
	return goose.NewGoMigration(10,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			migrator := db.Migrator()
			for _, col := range []string{"rating", "rated_by", "rated_at"} {
				if migrator.HasTable(&model.Ticket{}) && !migrator.HasColumn(&model.Ticket{}, col) {
					if err := migrator.AddColumn(&model.Ticket{}, col); err != nil {
						return fmt.Errorf("migrate: 0010 add tickets.%s: %w", col, err)
					}
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

// releaseMigration creates the game_releases table (0008).
func releaseMigration() *goose.Migration {
	return goose.NewGoMigration(8,
		&goose.GoFunc{RunDB: func(ctx context.Context, sqlDB *sql.DB) error {
			db, err := wrapGorm(sqlDB)
			if err != nil {
				return err
			}
			if !db.Migrator().HasTable(&model.GameRelease{}) {
				if err := db.Migrator().CreateTable(&model.GameRelease{}); err != nil {
					return fmt.Errorf("migrate: 0008 create game_releases: %w", err)
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
