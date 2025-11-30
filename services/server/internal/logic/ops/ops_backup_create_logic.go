// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"
	"strings"

	backuplogic "github.com/cuihairu/croupier/services/server/internal/logic/backup"
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

func (l *OpsBackupCreateLogic) OpsBackupCreate(req *types.OpsBackupCreateRequest) (*types.OpsBackupCreateResponse, error) {
	createReq := &types.BackupCreateRequest{
		Name: strings.TrimSpace(req.Name),
		Type: "manual",
	}
	backupResp, err := backuplogic.NewBackupCreateLogic(l.ctx, l.svcCtx).BackupCreate(createReq)
	if err != nil {
		return nil, err
	}

	return &types.OpsBackupCreateResponse{
		Code:    0,
		Message: "OK",
		Data:    backupResp.Backup,
	}, nil
}
