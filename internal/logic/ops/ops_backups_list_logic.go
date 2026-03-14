package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
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

func (l *OpsBackupsListLogic) OpsBackupsList(req *OpsBackupsListRequest) (resp *OpsBackupsListResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupsList not implemented")
}
