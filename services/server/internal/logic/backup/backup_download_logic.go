// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BackupDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 下载备份
func NewBackupDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupDownloadLogic {
	return &BackupDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupDownloadLogic) BackupDownload(req *types.BackupDownloadRequest) error {
	// todo: add your logic here and delete this line

	return nil
}
