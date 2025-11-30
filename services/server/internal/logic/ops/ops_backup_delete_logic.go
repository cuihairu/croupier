// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	backuplogic "github.com/cuihairu/croupier/services/server/internal/logic/backup"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除备份
func NewOpsBackupDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDeleteLogic {
	return &OpsBackupDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDeleteLogic) OpsBackupDelete(req *types.OpsBackupDeleteRequest) (*types.OpsBackupDeleteResponse, error) {
	deleteReq := &types.BackupDeleteRequest{ID: req.ID}
	if err := backuplogic.NewBackupDeleteLogic(l.ctx, l.svcCtx).BackupDelete(deleteReq); err != nil {
		return nil, err
	}

	return &types.OpsBackupDeleteResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
