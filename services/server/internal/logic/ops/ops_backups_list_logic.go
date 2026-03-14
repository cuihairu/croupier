// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsBackupsListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取备份列表
func NewOpsBackupsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupsListLogic {
	return &OpsBackupsListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupsListLogic) OpsBackupsList(req *types.OpsBackupsListRequest) (resp *types.OpsBackupsListResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupsList not implemented")
}
