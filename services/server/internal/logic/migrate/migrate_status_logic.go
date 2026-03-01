package migrate

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMigrateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateStatusLogic {
	return &MigrateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateStatusLogic) MigrateStatus(_ *types.MigrationStatusRequest) (*types.MigrationStatusResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看迁移状态", "admin:all"); err != nil {
		return nil, err
	}

	history, err := loadMigrateHistory(migrateHistoryPath(l.svcCtx))
	if err != nil {
		return nil, errorx.NewInternalError("读取迁移历史失败")
	}

	latest := ""
	if len(history) > 0 {
		latest = history[0].MigrationName
	}
	// Best-effort estimate based on migration sql files.
	migrations, _ := filepath.Glob("migrations/*.sql")
	sort.Strings(migrations)
	pending := len(migrations)

	return &types.MigrationStatusResponse{
		LatestVersion: latest,
		PendingCount:  pending,
		HistoryItems:  historySlice(history, 20),
	}, nil
}

func historySlice(items []types.MigrationResult, n int) []types.MigrationResult {
	if n <= 0 || len(items) <= n {
		return items
	}
	return append([]types.MigrationResult(nil), items[:n]...)
}
