// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type BackupsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取备份列表
func NewBackupsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupsListLogic {
	return &BackupsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupsListLogic) BackupsList(req *types.BackupsListRequest) (*types.BackupsListResponse, error) {
	opts := model.ListBackupsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Type: strings.TrimSpace(req.Type),
	}

	backups, total, err := l.svcCtx.BackupModel.List(l.ctx, opts)
	if err != nil {
		return nil, err
	}

	return &types.BackupsListResponse{
		Items: utils.BuildBackupList(backups),
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}
