// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package migrate

import (
	"context"
	"fmt"

	migratesvc "github.com/cuihairu/croupier/internal/repo/gorm/migrate"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateUpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 执行迁移
func NewMigrateUpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateUpLogic {
	return &MigrateUpLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateUpLogic) MigrateUp(req *types.MigrateUpRequest) (resp *types.MigrateUpResponse, err error) {
	// Initialize migration manager if not done
	migrateManager := migratesvc.NewMigrationManager(l.svcCtx.DB)

	// Get all migrations to run (auto-discover from code or from config)
	migrations, err := l.getMigrationsToRun()
	if err != nil {
		l.Errorf("Failed to get migrations: %v", err)
		return nil, l.errMigration("Failed to get migrations", err)
	}

	l.Infof("Starting migration up with %d migrations", len(migrations))

	// Run all up migrations
	err = migrateManager.RunMigrations(l.ctx, migrations, migratesvc.DirectionUp)
	if err != nil {
		l.Errorf("Migration failed: %v", err)
		return nil, l.errMigration("Migration failed", err)
	}

	resp = &types.MigrateUpResponse{
		Success: true,
		Message: "Migration completed successfully",
		Results: nil,
	}

	l.Infof("Migration up completed successfully")
	return resp, nil
}

func (l *MigrateUpLogic) errMigration(message string, err error) error {
	l.Errorf("Migration error: %s - %v", message, err)
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	return fmt.Errorf(message)
}

// getMigrationsToRun discovers and returns migrations to run
func (l *MigrateUpLogic) getMigrationsToRun() ([]migratesvc.Migration, error) {
	// For now, return empty - migrations can be discovered from code
	// In the future, this could scan for migration files or read from config
	return []migratesvc.Migration{}, nil
}
