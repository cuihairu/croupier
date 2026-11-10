package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/pressly/goose/v3"
)

// gameSoftDeleteResidueCleanupMigration 为 0039：games 删除路径改硬删
// （Unscoped）之后，把 0024/0026 同族的存量软删游戏行物理清除。
// games.game_id 与 games.alias_name 是含软删行的物理唯一索引，墓碑行占住
// 索引位会导致同名游戏重建直接 duplicate-key 500（线上实证：删除
// e2e_walkthrough_tmp 后重建报 idx_games_game_id 冲突）。纯 DELETE，
// 不动表结构，幂等；缺表/缺列跳过。
func gameSoftDeleteResidueCleanupMigration() *goose.Migration {
	return goose.NewGoMigration(39,
		&goose.GoFunc{RunDB: migrateGameSoftDeleteResidue},
		nil,
	)
}

// migrateGameSoftDeleteResidue 是 0039 的迁移体（抽出便于直测）。
func migrateGameSoftDeleteResidue(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.Game{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.Game{}, "DeletedAt") {
		return nil
	}
	if err := db.Exec("DELETE FROM games WHERE deleted_at IS NOT NULL").Error; err != nil {
		return fmt.Errorf("migrate: 0039 purge soft-deleted rows from games: %w", err)
	}
	return nil
}

// incidentTablesMigration 为 0040：事故报表体系批 1（docs/design/
// incident-reports.md）——incident_categories + incidents 建表（HasTable
// 检查后 CreateTable，0019/0038 新表模式，无存量约束名漂移）+ bugs 加
// category_id/subcategory 两列（HasColumn+AddColumn，0025 加列模式）+
// 播种默认七行类别（未分类兜底 + 六类职能域，按 slug 幂等缺行才插，
// 已有行不回改）。
func incidentTablesMigration() *goose.Migration {
	return goose.NewGoMigration(40,
		&goose.GoFunc{RunDB: migrateIncidentTables},
		nil,
	)
}

// migrateIncidentTables 是 0040 的迁移体（抽出便于直测）。
func migrateIncidentTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	m := db.Migrator()
	if !m.HasTable(&model.IncidentCategory{}) {
		if err := m.CreateTable(&model.IncidentCategory{}); err != nil {
			return fmt.Errorf("migrate: 0040 create incident_categories: %w", err)
		}
	}
	if !m.HasTable(&model.Incident{}) {
		if err := m.CreateTable(&model.Incident{}); err != nil {
			return fmt.Errorf("migrate: 0040 create incidents: %w", err)
		}
	}
	// bugs 缺表的库（multiGame fanout 重放到 game 库）整段跳过：不建空壳
	if m.HasTable(&model.Bug{}) {
		for _, col := range []string{"CategoryID", "Subcategory"} {
			if !m.HasColumn(&model.Bug{}, col) {
				if err := m.AddColumn(&model.Bug{}, col); err != nil {
					return fmt.Errorf("migrate: 0040 add bugs.%s: %w", col, err)
				}
			}
		}
	}
	if err := model.SeedIncidentCategories(ctx, db); err != nil {
		return fmt.Errorf("migrate: 0040 seed incident categories: %w", err)
	}
	return nil
}

// reportDistributionMigration 为 0041：事故报表体系批 4（docs/design/
// incident-reports.md §2.4/§2.5）——incident_reports + external_tokens 建
// 表（HasTable 检查后 CreateTable，新表无存量约束名漂移）+ messages 加
// level/source/ref_type/ref_id/scope 五列（HasColumn+AddColumn，0025 模
// 式；level 历史行回填 info）+ task_schedules 加 kind 列（回填 function，
// 存量行全是 function 形态）。调度行播种不在迁移内：Manager 支持 kind
// 路由（批 4c）之前落行会被误派发到占位 function_id 进 dead_letter。
func reportDistributionMigration() *goose.Migration {
	return goose.NewGoMigration(41,
		&goose.GoFunc{RunDB: migrateReportDistribution},
		nil,
	)
}

// migrateReportDistribution 是 0041 的迁移体（抽出便于直测）。
func migrateReportDistribution(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	m := db.Migrator()
	if !m.HasTable(&model.IncidentReport{}) {
		if err := m.CreateTable(&model.IncidentReport{}); err != nil {
			return fmt.Errorf("migrate: 0041 create incident_reports: %w", err)
		}
	}
	if !m.HasTable(&model.ExternalToken{}) {
		if err := m.CreateTable(&model.ExternalToken{}); err != nil {
			return fmt.Errorf("migrate: 0041 create external_tokens: %w", err)
		}
	}
	// 缺表的库（multiGame fanout 重放到 game 库）整段跳过：不建空壳
	if m.HasTable(&model.Message{}) {
		for _, col := range []string{"Level", "Source", "RefType", "RefID", "Scope"} {
			if !m.HasColumn(&model.Message{}, col) {
				if err := m.AddColumn(&model.Message{}, col); err != nil {
					return fmt.Errorf("migrate: 0041 add messages.%s: %w", col, err)
				}
			}
		}
		// level 回填：ADD COLUMN 的 default 只保证新行，存量行跨驱动保险
		if err := db.WithContext(ctx).Exec(
			"UPDATE messages SET level = ? WHERE level IS NULL OR level = ''",
			model.MessageLevelInfo,
		).Error; err != nil {
			return fmt.Errorf("migrate: 0041 backfill messages.level: %w", err)
		}
	}
	if m.HasTable(&model.TaskSchedule{}) {
		if !m.HasColumn(&model.TaskSchedule{}, "Kind") {
			if err := m.AddColumn(&model.TaskSchedule{}, "Kind"); err != nil {
				return fmt.Errorf("migrate: 0041 add task_schedules.kind: %w", err)
			}
		}
		if err := db.WithContext(ctx).Exec(
			"UPDATE task_schedules SET kind = ? WHERE kind IS NULL OR kind = ''",
			model.ScheduleKindFunction,
		).Error; err != nil {
			return fmt.Errorf("migrate: 0041 backfill task_schedules.kind: %w", err)
		}
	}
	return nil
}

func provisionRunsTableMigration() *goose.Migration {
	return goose.NewGoMigration(52,
		&goose.GoFunc{RunDB: migrateProvisionRunsTable},
		nil,
	)
}

func migrateProvisionRunsTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	// 新表 CreateTable：索引随建表一次建出，无存量约束名漂移（0023 教训）。
	// 缺表跳过（fanout 重放到无该特性的库不报错、不建空壳）。
	if !db.Migrator().HasTable(&model.ProvisionRun{}) {
		if err := db.Migrator().CreateTable(&model.ProvisionRun{}); err != nil {
			return fmt.Errorf("migrate: 0052 create provision_runs: %w", err)
		}
	}
	return nil
}

// provisionRunDestroyProtectionMigration adds the OPS-002 data-disk guard
// column to provision_runs: destroy_protected=true rejects automatic destroy.
// AddColumn 补列模式（0025/0043/0044/0049/0051）；缺表跳过（fanout 重放到
// 无该表的库不报错、不建空壳）；存量行默认 false=未护栏，行为与现状一致。
func provisionRunDestroyProtectionMigration() *goose.Migration {
	return goose.NewGoMigration(53,
		&goose.GoFunc{RunDB: migrateProvisionRunDestroyProtection},
		nil,
	)
}

func migrateProvisionRunDestroyProtection(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	// 供给特性不在此库（fanout 重放到无该表的库）：不报错、不建空壳。
	if !db.Migrator().HasTable(&model.ProvisionRun{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_protected") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.ProvisionRun{}, "DestroyProtected"); err != nil {
		return fmt.Errorf("migrate: 0053 add provision_runs.destroy_protected: %w", err)
	}
	return nil
}

// provisionRunDestroyApprovalMigration adds the OPS-003 destroy-approval link
// column to provision_runs: destroy_approval_id links the workflow-mode destroy
// ticket. AddColumn 补列模式（0025/0043/0044/0049/0051）；缺表跳过（fanout
// 重放到无该表的库不报错、不建空壳）；存量行空=未申请销毁审批，行为与现状一致。
func provisionRunDestroyApprovalMigration() *goose.Migration {
	return goose.NewGoMigration(54,
		&goose.GoFunc{RunDB: migrateProvisionRunDestroyApproval},
		nil,
	)
}

// provisionRunCloudInitExtraMigration adds the non-sensitive cloud-init extra
// snapshot column so ticket-mode runs can resume with the submitted fragment.
// AddColumn 补列模式（0025/0053/0054）；缺表跳过（fanout 重放到无该特性的库
// 不报错、不建空壳）；存量行空=未提供，行为与现状一致。
func provisionRunCloudInitExtraMigration() *goose.Migration {
	return goose.NewGoMigration(59,
		&goose.GoFunc{RunDB: migrateProvisionRunCloudInitExtra},
		nil,
	)
}

func migrateProvisionRunDestroyApproval(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	// 供给特性不在此库（fanout 重放到无该表的库）：不报错、不建空壳。
	if !db.Migrator().HasTable(&model.ProvisionRun{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.ProvisionRun{}, "destroy_approval_id") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.ProvisionRun{}, "DestroyApprovalID"); err != nil {
		return fmt.Errorf("migrate: 0054 add provision_runs.destroy_approval_id: %w", err)
	}
	return nil
}

func migrateProvisionRunCloudInitExtra(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.ProvisionRun{}) {
		return nil
	}
	if db.Migrator().HasColumn(&model.ProvisionRun{}, "cloud_init_extra") {
		return nil
	}
	if err := db.Migrator().AddColumn(&model.ProvisionRun{}, "CloudInitExtra"); err != nil {
		return fmt.Errorf("migrate: 0059 add provision_runs.cloud_init_extra: %w", err)
	}
	return nil
}

// backupScopeColumnsMigration adds game/env/source ownership columns to backups
// (OPS-008). 游戏库备份（ops/backups）归属经 capture 所见数据源登记；平台级
// 备份（meta DB）三列空。AddColumn 补列模式（0025/0053/0054/0059）；缺表跳过
// （fanout 重放到无该表的库不报错、不建空壳）；存量行三列空=平台级备份，
// 行为与现状一致。复合索引经 HasIndex 守卫后 CreateIndex。
func backupScopeColumnsMigration() *goose.Migration {
	return goose.NewGoMigration(60,
		&goose.GoFunc{RunDB: migrateBackupScopeColumns},
		nil,
	)
}

func migrateBackupScopeColumns(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.Backup{}) {
		return nil
	}
	cols := []struct {
		column string
		field  string
	}{
		{column: "game_id", field: "GameID"},
		{column: "env", field: "Env"},
		{column: "source_id", field: "SourceID"},
		{column: "source_name", field: "SourceName"},
	}
	for _, c := range cols {
		if db.Migrator().HasColumn(&model.Backup{}, c.column) {
			continue
		}
		if err := db.Migrator().AddColumn(&model.Backup{}, c.field); err != nil {
			return fmt.Errorf("migrate: 0060 add backups.%s: %w", c.column, err)
		}
	}
	if !db.Migrator().HasIndex(&model.Backup{}, "idx_backup_scope") {
		if err := db.Migrator().CreateIndex(&model.Backup{}, "idx_backup_scope"); err != nil {
			return fmt.Errorf("migrate: 0060 add backups.idx_backup_scope: %w", err)
		}
	}
	return nil
}

// deployTablesMigration creates the OPS-004 deploy_packages + deployments tables
// (包仓库 + 发版单 + 目标分批快照). 新表 CreateTable：索引随建表一次建出，无
// 存量约束名漂移（0023 教训）。缺表跳过（fanout 重放到无该特性的库不报错、
// 不建空壳）。
func deployTablesMigration() *goose.Migration {
	return goose.NewGoMigration(55,
		&goose.GoFunc{RunDB: migrateDeployTables},
		nil,
	)
}

func migrateDeployTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.DeployPackage{}) {
		if err := db.Migrator().CreateTable(&model.DeployPackage{}); err != nil {
			return fmt.Errorf("migrate: 0055 create deploy_packages: %w", err)
		}
	}
	if !db.Migrator().HasTable(&model.Deployment{}) {
		if err := db.Migrator().CreateTable(&model.Deployment{}); err != nil {
			return fmt.Errorf("migrate: 0055 create deployments: %w", err)
		}
	}
	return nil
}

// agentLogTablesMigration creates the OPS-005 agent_log_entries table.
// New table CreateTable: indexes are created with the table (no legacy name
// drift). Missing-table skip follows the deploy pattern.
func agentLogTablesMigration() *goose.Migration {
	return goose.NewGoMigration(56,
		&goose.GoFunc{RunDB: migrateAgentLogTables},
		nil,
	)
}

func migrateAgentLogTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&reg.AgentLogEntryRecord{}) {
		if err := db.Migrator().CreateTable(&reg.AgentLogEntryRecord{}); err != nil {
			return fmt.Errorf("migrate: 0056 create agent_log_entries: %w", err)
		}
	}
	return nil
}

// batchOpTablesMigration creates the OPS-006 batch_ops table.
func batchOpTablesMigration() *goose.Migration {
	return goose.NewGoMigration(57,
		&goose.GoFunc{RunDB: migrateBatchOpTables},
		nil,
	)
}

func migrateBatchOpTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.BatchOp{}) {
		if err := db.Migrator().CreateTable(&model.BatchOp{}); err != nil {
			return fmt.Errorf("migrate: 0057 create batch_ops: %w", err)
		}
	}
	return nil
}

// costBudgetTablesMigration creates the OPS-007 cost_budgets table.
func costBudgetTablesMigration() *goose.Migration {
	return goose.NewGoMigration(58,
		&goose.GoFunc{RunDB: migrateCostBudgetTables},
		nil,
	)
}

func migrateCostBudgetTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.CostBudget{}) {
		if err := db.Migrator().CreateTable(&model.CostBudget{}); err != nil {
			return fmt.Errorf("migrate: 0058 create cost_budgets: %w", err)
		}
	}
	return nil
}

// runbookTablesMigration creates the OPS-009 runbooks + runbook_runs tables
// (参数化步骤序列引用 OPS-004 部署 / OPS-006 批处理能力). 新表 CreateTable：
// 索引随建表一次建出，无存量约束名漂移问题（0019 announcements 先例）。
func runbookTablesMigration() *goose.Migration {
	return goose.NewGoMigration(62,
		&goose.GoFunc{RunDB: migrateRunbookTables},
		nil,
	)
}

func migrateRunbookTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&model.Runbook{}) {
		if err := db.Migrator().CreateTable(&model.Runbook{}); err != nil {
			return fmt.Errorf("migrate: 0062 create runbooks: %w", err)
		}
	}
	if !db.Migrator().HasTable(&model.RunbookRun{}) {
		if err := db.Migrator().CreateTable(&model.RunbookRun{}); err != nil {
			return fmt.Errorf("migrate: 0062 create runbook_runs: %w", err)
		}
	}
	return nil
}

// powerRequestTablesMigration creates the OPS power management tables:
// power_requests (approval-gated request header with state machine) +
// power_request_targets (per-machine outcome rows). 新表 CreateTable：
// 索引随建表一次建出，无存量约束名漂移问题（0019 announcements 先例）。
// 缺表跳过（fanout 重放到无该特性的库不报错、不建空壳）——本表与
// provision_runs 同族属可选特性，直建即可（无存量库携带该表）。
func powerRequestTablesMigration() *goose.Migration {
	return goose.NewGoMigration(64,
		&goose.GoFunc{RunDB: migratePowerRequestTables},
		nil,
	)
}

func migratePowerRequestTables(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	m := db.Migrator()
	if !m.HasTable(&model.PowerRequest{}) {
		if err := m.CreateTable(&model.PowerRequest{}); err != nil {
			return fmt.Errorf("migrate: 0064 create power_requests: %w", err)
		}
	}
	if !m.HasTable(&model.PowerRequestTarget{}) {
		if err := m.CreateTable(&model.PowerRequestTarget{}); err != nil {
			return fmt.Errorf("migrate: 0064 create power_request_targets: %w", err)
		}
	}
	return nil
}
