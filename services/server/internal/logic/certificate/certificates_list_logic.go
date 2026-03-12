// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificatesListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书列表
func NewCertificatesListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificatesListLogic {
	return &CertificatesListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificatesListLogic) CertificatesList(req *types.CertificatesListRequest) (resp *types.CertificatesListResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
