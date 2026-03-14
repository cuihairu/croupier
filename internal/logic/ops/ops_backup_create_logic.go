
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
)

type OpsBackupCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建备份
func NewOpsBackupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupCreateLogic {
	return &OpsBackupCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupCreateLogic) OpsBackupCreate(req *OpsBackupCreateRequest) (resp *OpsBackupCreateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupCreate not implemented")
}
