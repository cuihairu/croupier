// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BackupCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建备份
func NewBackupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupCreateLogic {
	return &BackupCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupCreateLogic) BackupCreate(req *types.BackupCreateRequest) (resp *types.BackupDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
