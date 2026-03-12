// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

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

func (l *OpsBackupsListLogic) OpsBackupsList(req *types.OpsBackupsListRequest) (resp *types.OpsBackupsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
