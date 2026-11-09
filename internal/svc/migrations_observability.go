package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

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

// cicdTablesMigration 为 0038：建 cicd_integrations + cicd_builds 表（CI/CD
// 可插拔 provider 接入 #58）。两张均为新表（无存量约束名漂移问题），
// HasTable 检查后 CreateTable，幂等；单库/多游戏两种模式都会在对应
// scope 上执行本迁移（0020 execution_logs 同模式）。
func cicdTablesMigration() *goose.Migration {
	return goose.NewGoMigration(38,
		&goose.GoFunc{RunDB: migrateCicdTables},
		nil,
	)
}

// migrateCicdTables 是 0038 的迁移体（抽出便于直测）。
func migrateCicdTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.CicdIntegration{}) {
		if err := migrator.CreateTable(&model.CicdIntegration{}); err != nil {
			return fmt.Errorf("migrate: 0038 create cicd_integrations: %w", err)
		}
	}
	if !migrator.HasTable(&model.CicdBuild{}) {
		if err := migrator.CreateTable(&model.CicdBuild{}); err != nil {
			return fmt.Errorf("migrate: 0038 create cicd_builds: %w", err)
		}
	}
	return nil
}

// alertHandlingMigration 为 0042：alerts 风险事件工作流状态四列
// （handle_status/handled_by/handled_remark/handled_at，risk-control-panel
// 设计 §3.3 风险事件扩展）。HandleStatus 独立于既有 status 列——后者被
// AlertManager 入站 lifecycle（firing/resolved）占用，人工处理状态混列会
// 互相覆盖。HasColumn+AddColumn 逐列补齐（0033 同模式；不整模型
// AutoMigrate——0023 教训）；缺表跳过（fanout 不建空壳）；存量行回填
// handle_status='pending'（与模型 default 对齐，NULL 不参与语义）。
func alertHandlingMigration() *goose.Migration {
	return goose.NewGoMigration(42,
		&goose.GoFunc{RunDB: migrateAlertHandlingColumns},
		nil,
	)
}

// migrateAlertHandlingColumns 是 0042 的迁移体（抽出便于直测）。
func migrateAlertHandlingColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.Alert{}) {
		return nil
	}
	for _, col := range []string{"HandleStatus", "HandledBy", "HandledRemark", "HandledAt"} {
		if migrator.HasColumn(&model.Alert{}, col) {
			continue
		}
		if err := migrator.AddColumn(&model.Alert{}, col); err != nil {
			return fmt.Errorf("migrate: 0042 add alerts.%s: %w", col, err)
		}
	}
	// 存量行回填：待处理是唯一诚实的初始态（幂等，仅 NULL 行）。
	if err := db.WithContext(ctx).Model(&model.Alert{}).
		Where("handle_status IS NULL OR handle_status = ''").
		Update("handle_status", model.AlertHandlePending).Error; err != nil {
		return fmt.Errorf("migrate: 0042 backfill alerts.handle_status: %w", err)
	}
	return nil
}

// alertScopeMigration 为 0043：alerts scope 两列 game_id/env（4862 ops 告警
// 列表按 game/env 过滤）。空 = 平台级（全局可见），非空 = game/env 级。
// HasColumn+AddColumn 逐列补齐（0042 同模式）；缺表跳过；存量行不回填——
// 历史告警无 scope 归属，诚实留空（平台级）。
func alertScopeMigration() *goose.Migration {
	return goose.NewGoMigration(43,
		&goose.GoFunc{RunDB: migrateAlertScopeColumns},
		nil,
	)
}

// migrateAlertScopeColumns 是 0043 的迁移体（抽出便于直测）。
func migrateAlertScopeColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&model.Alert{}) {
		return nil
	}
	for _, col := range []string{"GameID", "Env"} {
		if migrator.HasColumn(&model.Alert{}, col) {
			continue
		}
		if err := migrator.AddColumn(&model.Alert{}, col); err != nil {
			return fmt.Errorf("migrate: 0043 add alerts.%s: %w", col, err)
		}
	}
	// 存量行不回填：历史告警无 scope 归属，留空（平台级）。
	return nil
}

func agentMetricsProbeJSONMigration() *goose.Migration {
	return goose.NewGoMigration(44,
		&goose.GoFunc{RunDB: migrateAgentMetricsProbeJSON},
		nil,
	)
}

// migrateAgentMetricsProbeJSON 为 0044：agent_metrics_history 加 probe_json
// 列（探测统计持久化，评审简档 §13.1）。模型定义在 platform/registry（svc
// 可 import，无环）。存量行不回填——历史窗口本就无探测统计快照，留空。
func migrateAgentMetricsProbeJSON(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&reg.AgentMetricsHistory{}) {
		return nil
	}
	if migrator.HasColumn(&reg.AgentMetricsHistory{}, "ProbeJSON") {
		return nil
	}
	if err := migrator.AddColumn(&reg.AgentMetricsHistory{}, "ProbeJSON"); err != nil {
		return fmt.Errorf("migrate: 0044 add agent_metrics_history.probe_json: %w", err)
	}
	return nil
}

// probeEventsTableMigration 为 0046：probe_events 建表（synthetics 探测事件
// 持久化，item 31/36 交付的模型 998e79f 起只在 AutoMigrate 路径建表，线上
// postgres 从未有过该表——2026-10-20 实证 information_schema 无 probe_events，
// 探测事件写入在线上必报错，页面侧永远查不到数据）。新建表无存量约束名漂移
// 问题，HasTable 检查后 CreateTable；payload 列类型见 model.ProbeEvent 的
// 注释（不用 MySQL 专有的 `type:blob`）。
func probeEventsTableMigration() *goose.Migration {
	return goose.NewGoMigration(46,
		&goose.GoFunc{RunDB: migrateProbeEventsTable},
		nil,
	)
}

func migrateProbeEventsTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if db.Migrator().HasTable(&model.ProbeEvent{}) {
		return nil
	}
	if err := db.Migrator().CreateTable(&model.ProbeEvent{}); err != nil {
		return fmt.Errorf("migrate: 0046 create probe_events: %w", err)
	}
	return nil
}

// probeEventTargetCategoryMigration 为 0047：已有 probe_events 表补目标
// 类别列。0046 新表走 GORM CreateTable 已包含模型字段，HasColumn 后跳过。
func probeEventTargetCategoryMigration() *goose.Migration {
	return goose.NewGoMigration(47,
		&goose.GoFunc{RunDB: migrateProbeEventTargetCategory},
		nil,
	)
}

func migrateProbeEventTargetCategory(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.ProbeEvent{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.ProbeEvent{}, "TargetCategory") {
		if err := db.Migrator().AddColumn(&model.ProbeEvent{}, "TargetCategory"); err != nil {
			return fmt.Errorf("migrate: 0047 add probe_events.target_category: %w", err)
		}
	}
	return nil
}

// auditChainVersionMigration adds audit_records.chain_version (ARCH-008):
// pins the hash algorithm/field set per row (1 = legacy DJB2 over the
// 7-field subset, 2 = canonical SHA-256 over the full record). Existing rows
// backfill to 1 via the column DEFAULT; existing hashes are never rewritten
// because each later row's chain_prev_hash references the previous hash
// value. AddColumn 补列模式（0015/0016/0021/0025/0027）；multiGame fanout
// 到无 audit_records 表的库由 HasTable 跳过。
func auditChainVersionMigration() *goose.Migration {
	return goose.NewGoMigration(49,
		&goose.GoFunc{RunDB: migrateAuditChainVersionColumn},
		nil,
	)
}

func migrateAuditChainVersionColumn(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&audit.AuditModel{}) {
		return nil
	}
	if migrator.HasColumn(&audit.AuditModel{}, "ChainVersion") {
		return nil
	}
	if err := migrator.AddColumn(&audit.AuditModel{}, "ChainVersion"); err != nil {
		return fmt.Errorf("migrate: 0049 add audit_records.chain_version: %w", err)
	}
	return nil
}

// approvalDispatchClaimColumnsMigration creates the approvals table when it is
// missing and adds the durable dispatch-claim columns used by ARCH-002. This
// migration is included in both meta and game fan-out because the approvals
// store is scoped by game_id/env on the same server database.
func approvalDispatchClaimColumnsMigration() *goose.Migration {
	return goose.NewGoMigration(50,
		&goose.GoFunc{RunDB: migrateApprovalDispatchClaimColumns},
		nil,
	)
}

func migrateApprovalDispatchClaimColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&approvals.ApprovalModel{}) {
		if err := migrator.CreateTable(&approvals.ApprovalModel{}); err != nil {
			return fmt.Errorf("migrate: 0050 create approvals: %w", err)
		}
		return nil
	}
	columns := map[string]string{
		"ClaimStatus":   "claim_status",
		"ClaimToken":    "claim_token",
		"ClaimedAt":     "claimed_at",
		"PayloadDigest": "payload_digest",
	}
	for field, column := range columns {
		if migrator.HasColumn(&approvals.ApprovalModel{}, column) {
			continue
		}
		if err := migrator.AddColumn(&approvals.ApprovalModel{}, field); err != nil {
			return fmt.Errorf("migrate: 0050 add approvals.%s: %w", column, err)
		}
	}
	return nil
}

// agentMetricsHistoryEnrichmentMigration adds the OPS-010 faces to the raw
// metrics history table and creates the 90-day hourly rollup table. Included
// in meta scope only — agent_metrics_history lives in the meta database.
func agentMetricsHistoryEnrichmentMigration() *goose.Migration {
	return goose.NewGoMigration(51,
		&goose.GoFunc{RunDB: migrateAgentMetricsHistoryEnrichment},
		nil,
	)
}

func migrateAgentMetricsHistoryEnrichment(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	migrator := db.Migrator()

	// 指标特性不在此库（fanout 重放到无该表的库）：不报错、不建空壳。
	if !migrator.HasTable(&reg.AgentMetricsHistory{}) {
		return nil
	}

	columns := map[string]string{
		"NetworksJSON":  "networks_json",
		"ProcessesJSON": "processes_json",
		"PortsJSON":     "ports_json",
	}
	for field, column := range columns {
		if migrator.HasColumn(&reg.AgentMetricsHistory{}, column) {
			continue
		}
		if err := migrator.AddColumn(&reg.AgentMetricsHistory{}, field); err != nil {
			return fmt.Errorf("migrate: 0051 add agent_metrics_history.%s: %w", column, err)
		}
	}

	if !migrator.HasTable(&reg.AgentMetricsHourly{}) {
		if err := migrator.CreateTable(&reg.AgentMetricsHourly{}); err != nil {
			return fmt.Errorf("migrate: 0051 create agent_metrics_hourly: %w", err)
		}
	}
	return nil
}

// agentMetricsHourlyUniqueIndexMigration converts the OPS-010 hourly rollup's
// (agent_id, hour_start) index from non-unique to unique. RollupHourly
// previously used non-atomic find-or-create; concurrent servers could insert
// duplicate rows and corrupt 90-day aggregates. Existing duplicate rows are
// collapsed deterministically before the unique index is created.
func agentMetricsHourlyUniqueIndexMigration() *goose.Migration {
	return goose.NewGoMigration(61,
		&goose.GoFunc{RunDB: migrateAgentMetricsHourlyUniqueIndex},
		nil,
	)
}

func migrateAgentMetricsHourlyUniqueIndex(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	return ensureAgentMetricsHourlyConverged(ctx, db)
}

// ensureAgentMetricsHourlyConverged converges an existing agent_metrics_hourly
// table to the current schema: backfills agent_id to VARCHAR(64), collapses
// duplicate (agent_id, hour_start) rows, and ensures the unique index exists.
// Shared by 0061 (new install) and 0063 (already-upgraded instances, where
// editing 0061 in place never re-runs). Idempotent and skips a missing table.
func ensureAgentMetricsHourlyConverged(ctx context.Context, db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&reg.AgentMetricsHourly{}) {
		return nil
	}
	// 收敛（列回填 + 去重 + 索引重建）须避免「去重已生效但唯一索引已丢」的
	// 窗口：先建唯一索引 uidx_agent_hour，再删旧非唯一索引 idx_agent_hour，
	// 任一时刻 (agent_id, hour_start) 上都有索引。SQLite 的 AlterColumn 会重建
	// 表并删除原有索引，故整体包裹单事务，任一步失败回滚，原索引与数据不变；
	// MySQL/PostgreSQL 的 DDL（ALTER/CREATE/DROP INDEX）会隐式提交、事务对 DDL
	// 不具原子性，此时依赖「先建后删」顺序 + 失败上抛（迁移失败即启动失败，
	// 不静默留下无唯一约束状态）来保证安全。
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 存量库（已跑 0051 建表）的 agent_id 无尺寸（默认为数据库默认），与新建
		// 库（0051 CreateTable 按 size:64 建 VARCHAR(64)）schema 分叉；且 MySQL 下
		// TEXT 无前缀不能建索引，0061 建 uidx_agent_hour 会在存量库失败。统一回填
		// 为 VARCHAR(64)：先列类型迁移（幂等），再建唯一索引。
		if err := alterAgentMetricsHourlyAgentID(ctx, tx); err != nil {
			return fmt.Errorf("migrate: agent_metrics_hourly alter agent_id: %w", err)
		}
		if err := tx.Exec(`
DELETE FROM agent_metrics_hourly
WHERE id NOT IN (
  SELECT keep_id FROM (
    SELECT MIN(id) AS keep_id FROM agent_metrics_hourly GROUP BY agent_id, hour_start
  ) AS keep_rows
)`).Error; err != nil {
			return fmt.Errorf("migrate: agent_metrics_hourly dedupe: %w", err)
		}
		// 审核发现注入点：去重后、索引重建前模拟崩溃/失败，验证整体回滚。
		if agentMetricsHourlyConvergeAfterDedupe != nil {
			if err := agentMetricsHourlyConvergeAfterDedupe(); err != nil {
				return err
			}
		}
		txMigrator := tx.Migrator()
		// 先建唯一索引（若失败，旧 idx_agent_hour 仍在，无无索引窗口）。
		if !txMigrator.HasIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour") {
			if err := txMigrator.CreateIndex(&reg.AgentMetricsHourly{}, "uidx_agent_hour"); err != nil {
				return fmt.Errorf("migrate: agent_metrics_hourly create uidx_agent_hour: %w", err)
			}
		}
		// 唯一索引就绪后再删旧非唯一索引。
		if txMigrator.HasIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour") {
			if err := txMigrator.DropIndex(&reg.AgentMetricsHourly{}, "idx_agent_hour"); err != nil {
				return fmt.Errorf("migrate: agent_metrics_hourly drop idx_agent_hour: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// agentMetricsHourlyConvergeAfterDedupe 测试注入点：在去重之后、索引重建之前
// 被调用。测试用它模拟「表已重建、唯一索引已丢、建索引前崩溃」的窗口，断言
// 整个收敛在单事务内回滚，原有唯一索引与数据不丢失。生产恒为 nil。
var agentMetricsHourlyConvergeAfterDedupe func() error

// alterAgentMetricsHourlyAgentID converges agent_metrics_hourly.agent_id to
// VARCHAR(64) (the schema tag) unless existing rows already contain longer
// values: narrowing past the longest value would fail on strict databases
// (PostgreSQL) and prevent startup. In that case it widens to fit the longest
// value instead, preserving legacy data; new registrations are capped at 64 by
// the register handler, so the column never grows further.
func alterAgentMetricsHourlyAgentID(ctx context.Context, db *gorm.DB) error {
	var maxLen sql.NullInt64
	lengthFn := "CHAR_LENGTH"
	switch db.Dialector.Name() {
	case "sqlite":
		lengthFn = "LENGTH"
	case "sqlserver":
		// SQL Server 没有 CHAR_LENGTH，字符串长度函数是 LEN。
		lengthFn = "LEN"
	}
	if err := db.WithContext(ctx).Raw(
		"SELECT MAX(" + lengthFn + "(agent_id)) FROM agent_metrics_hourly",
	).Scan(&maxLen).Error; err != nil {
		return fmt.Errorf("probe agent_metrics_hourly max agent_id length: %w", err)
	}
	required := int64(64)
	if maxLen.Valid && maxLen.Int64 > required {
		required = maxLen.Int64
	}

	migrator := db.Migrator()
	columnTypes, err := migrator.ColumnTypes(&reg.AgentMetricsHourly{})
	if err != nil {
		return fmt.Errorf("read agent_metrics_hourly column types: %w", err)
	}
	for _, col := range columnTypes {
		if col.Name() != "agent_id" {
			continue
		}
		currentLen, _ := col.Length()
		if currentLen >= required {
			return nil
		}
		if required > 64 {
			switch db.Dialector.Name() {
			case "mysql":
				err = db.WithContext(ctx).Exec(
					"ALTER TABLE agent_metrics_hourly MODIFY agent_id VARCHAR(?) NOT NULL",
					required,
				).Error
			case "postgres":
				err = db.WithContext(ctx).Exec(
					"ALTER TABLE agent_metrics_hourly ALTER COLUMN agent_id TYPE VARCHAR(?)",
					required,
				).Error
			default:
				// SQLite 等不强制 VARCHAR 长度的引擎：缩窄到 size:64 不会因
				// 存量超长行失败（长度不校验），走常规 AlterColumn 即可。
				return migrator.AlterColumn(&reg.AgentMetricsHourly{}, "AgentID")
			}
			return err
		}
	}
	return migrator.AlterColumn(&reg.AgentMetricsHourly{}, "AgentID")
}

// agentMetricsHourlyAgentIDSizeMigration backfills agent_metrics_hourly.agent_id
// to VARCHAR(64) on databases that already ran migration 0061. The original
// 0061 added the AlterColumn inside the existing migration, which never re-runs
// on upgraded instances: their agent_id remains an unsized column, diverging
// from fresh databases created after 0051 (which already used size:64), and
// breaking the MySQL unique index creation. A new version migration covers
// those already-upgraded instances (idempotent, skips missing table).
func agentMetricsHourlyAgentIDSizeMigration() *goose.Migration {
	return goose.NewGoMigration(63,
		&goose.GoFunc{RunDB: migrateAgentMetricsHourlyAgentIDSize},
		nil,
	)
}

func migrateAgentMetricsHourlyAgentIDSize(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	return ensureAgentMetricsHourlyConverged(ctx, db)
}
