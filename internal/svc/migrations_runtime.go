package svc

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/pressly/goose/v3"
)

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
