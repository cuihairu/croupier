package migrate

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateDownLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMigrateDownLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateDownLogic {
	return &MigrateDownLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateDownLogic) MigrateDown(_ *types.MigrateDownRequest) (*types.MigrateDownResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权执行迁移回滚", "admin:all"); err != nil {
		return nil, err
	}

	start := time.Now()
	result := types.MigrationResult{
		MigrationName: "gorm_auto_migrate",
		Direction:     "down",
		Status:        "failed",
		DryRun:        true,
		SQL:           "",
		Error:         "当前版本暂不支持自动 down 回滚，请使用数据库备份恢复",
		Duration:      nowDurationMS(start),
	}
	_ = appendMigrateHistory(l.svcCtx, []types.MigrationResult{result})

	return &types.MigrateDownResponse{
		Success: false,
		Message: "当前版本暂不支持自动 down 回滚",
		Results: []types.MigrationResult{result},
	}, nil
}
