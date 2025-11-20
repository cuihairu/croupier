package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsBackupsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupsListLogic {
	return &OpsBackupsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupsListLogic) OpsBackupsList() (*types.OpsBackupsResponse, error) {
	backups := l.svcCtx.ListBackups()
	items := make([]types.BackupEntry, 0, len(backups))
	for _, b := range backups {
		items = append(items, types.BackupEntry{
			Id:        b.ID,
			Kind:      b.Kind,
			Target:    b.Target,
			Path:      b.Path,
			Size:      b.Size,
			Status:    b.Status,
			Error:     b.Error,
			CreatedAt: b.CreatedAt.Format(time.RFC3339),
		})
	}
	return &types.OpsBackupsResponse{Backups: items}, nil
}
