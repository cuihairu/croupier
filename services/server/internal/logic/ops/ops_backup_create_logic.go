// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建备份
func NewOpsBackupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupCreateLogic {
	return &OpsBackupCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupCreateLogic) OpsBackupCreate(req *types.OpsBackupCreateRequest) (resp *types.OpsBackupCreateResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
