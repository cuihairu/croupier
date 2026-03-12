// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书详情
func NewCertificateDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateDetailLogic {
	return &CertificateDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateDetailLogic) CertificateDetail(req *types.CertificateDetailRequest) (resp *types.CertificateDetailResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
