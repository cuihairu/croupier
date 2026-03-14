
package ops

import (
	"context"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
	
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

func (l *OpsBackupDownloadLogic) OpsBackupDownload(req *OpsBackupDownloadRequest) (resp *OpsBackupDownloadResponse, err error) {
	return nil, errorx.NewNotImplemented("OpsBackupDownload not implemented")
}
