package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

type OpsBackupDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除备份
func NewOpsBackupDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDeleteLogic {
	return &OpsBackupDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDeleteLogic) OpsBackupDelete(req *OpsBackupDeleteRequest) (resp *OpsBackupDeleteResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupDelete not implemented")
}
