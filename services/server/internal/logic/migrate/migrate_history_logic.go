package migrate

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MigrateHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMigrateHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MigrateHistoryLogic {
	return &MigrateHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MigrateHistoryLogic) MigrateHistory(req *types.MigrationHistoryRequest) (*types.MigrationHistoryResponse, error) {
	if _, _, err := utils.RequireAnyPermission(l.ctx, l.svcCtx, "无权查看迁移历史", "admin:all"); err != nil {
		return nil, err
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status := strings.TrimSpace(req.Status)

	items, err := loadMigrateHistory(migrateHistoryPath(l.svcCtx))
	if err != nil {
		return nil, errorx.NewInternalError("读取迁移历史失败")
	}

	filtered := make([]types.MigrationResult, 0, len(items))
	for _, item := range items {
		if status != "" && !strings.EqualFold(item.Status, status) {
			continue
		}
		filtered = append(filtered, item)
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &types.MigrationHistoryResponse{
		Items: filtered[start:end],
		Total: int64(total),
		Page:  page,
		Size:  pageSize,
	}, nil
}
