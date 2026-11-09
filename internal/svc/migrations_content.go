package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/pressly/goose/v3"
)

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

// announcementGamesTableMigration 为 0036：建 announcement_games 表
// （公告↔游戏多对多绑定，#45：一公告可绑定多游戏，未绑定=全服可见）。
// 新表无存量数据，HasTable 检查后 CreateTable（0019 公告两表同模式；
// 不整模型 AutoMigrate——0023 教训）。
func announcementGamesTableMigration() *goose.Migration {
	return goose.NewGoMigration(36,
		&goose.GoFunc{RunDB: migrateAnnouncementGamesTable},
		nil,
	)
}

// migrateAnnouncementGamesTable 是 0036 的迁移体（抽出便于直测）。
func migrateAnnouncementGamesTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if db.Migrator().HasTable(&model.AnnouncementGame{}) {
		return nil
	}
	if err := db.Migrator().CreateTable(&model.AnnouncementGame{}); err != nil {
		return fmt.Errorf("migrate: 0036 create announcement_games: %w", err)
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

// pageExecuteIdempotencyTableMigration 为 0048：页面绑定执行的服务端幂等
// 占位表（D4 安全重试）。新表无存量数据与约束名漂移，HasTable 检查后
// CreateTable（0027/0038 建表模式）；multiGame 模式下迁移 fanout 到各
// game 物理库，缺表跳过语义由本函数 HasTable 保证。
func pageExecuteIdempotencyTableMigration() *goose.Migration {
	return goose.NewGoMigration(48,
		&goose.GoFunc{RunDB: migratePageExecuteIdempotencyTable},
		nil,
	)
}

func migratePageExecuteIdempotencyTable(ctx context.Context, sqlDB *sql.DB) error {
	db, err := wrapGorm(sqlDB)
	if err != nil {
		return err
	}
	if db.Migrator().HasTable(&model.PageExecuteIdempotency{}) {
		return nil
	}
	if err := db.Migrator().CreateTable(&model.PageExecuteIdempotency{}); err != nil {
		return fmt.Errorf("migrate: 0048 create page_execute_idempotencies: %w", err)
	}
	return nil
}
