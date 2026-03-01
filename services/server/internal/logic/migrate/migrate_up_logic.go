package migrate

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateUpLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMigrateUpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateUpLogic {
	return &MigrateUpLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateUpLogic) MigrateUp(_ *types.MigrateUpRequest) (*types.MigrateUpResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权执行迁移", "admin:all"); err != nil {
		return nil, err
	}

	start := time.Now()
	result := types.MigrationResult{
		MigrationName: "gorm_auto_migrate",
		Direction:     "up",
		Status:        "success",
		DryRun:        false,
		SQL:           "gorm auto migrate",
	}
	if err := model.AutoMigrate(l.svcCtx.DB); err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	}
	result.Duration = nowDurationMS(start)
	_ = appendMigrateHistory(l.svcCtx, []types.MigrationResult{result})

	return &types.MigrateUpResponse{
		Success: result.Status == "success",
		Message: map[bool]string{true: "迁移执行成功", false: "迁移执行失败"}[result.Status == "success"],
		Results: []types.MigrationResult{result},
	}, nil
}
