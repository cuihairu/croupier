// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BackupsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取备份列表
func NewBackupsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupsListLogic {
	return &BackupsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupsListLogic) BackupsList(req *types.BackupsListRequest) (resp *types.BackupsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
