// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateAlertAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加证书告警
func NewCertificateAlertAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateAlertAddLogic {
	return &CertificateAlertAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateAlertAddLogic) CertificateAlertAdd(req *types.CertificateAlertAddRequest) (resp *types.CertificateAlertAddResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
