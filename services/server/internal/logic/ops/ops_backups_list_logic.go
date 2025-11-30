// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取备份列表
func NewOpsBackupsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupsListLogic {
	return &OpsBackupsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupsListLogic) OpsBackupsList(req *types.OpsBackupsListRequest) (*types.OpsBackupsListResponse, error) {
	opts := model.ListBackupsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Type: "",
	}

	backups, total, err := l.svcCtx.BackupModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	return &types.OpsBackupsListResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"backups": utils.BuildBackupList(backups),
			"total":   total,
			"page":    opts.Page,
			"size":    opts.PageSize,
		},
	}, nil
}
