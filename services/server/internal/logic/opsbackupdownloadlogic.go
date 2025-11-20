package logic

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsBackupDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDownloadLogic {
	return &OpsBackupDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDownloadLogic) OpsBackupDownload(id string) (string, bool) {
	entry, ok := l.svcCtx.BackupFile(id)
	if !ok {
		return "", false
	}
	return entry.Path, true
}
