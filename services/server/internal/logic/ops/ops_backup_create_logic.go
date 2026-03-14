// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
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

func (l *OpsBackupCreateLogic) OpsBackupCreate(req *types.OpsBackupCreateRequest) (resp *types.OpsBackupCreateResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupCreate not implemented")
}
