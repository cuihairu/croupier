// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateAlertsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书告警列表
func NewCertificateAlertsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertsListLogic {
	return &CertificateAlertsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAlertsListLogic) CertificateAlertsList(req *types.CertificateAlertsListRequest) (resp *types.CertificateAlertsListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
