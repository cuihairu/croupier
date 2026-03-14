// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package ops

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type OpsBackupDownloadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 下载备份
func NewOpsBackupDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsBackupDownloadLogic {
	return &OpsBackupDownloadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsBackupDownloadLogic) OpsBackupDownload(req *types.OpsBackupDownloadRequest) (resp *types.OpsBackupDownloadResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupDownload not implemented")
}
