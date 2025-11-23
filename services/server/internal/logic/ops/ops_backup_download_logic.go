// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsBackupDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 下载备份
func NewOpsBackupDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDownloadLogic {
	return &OpsBackupDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDownloadLogic) OpsBackupDownload(req *types.OpsBackupDownloadRequest) (resp *types.OpsBackupDownloadResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
