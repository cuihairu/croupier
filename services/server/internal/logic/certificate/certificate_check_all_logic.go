// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateCheckAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 检查所有证书
func NewCertificateCheckAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateCheckAllLogic {
	return &CertificateCheckAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateCheckAllLogic) CertificateCheckAll(req *types.CertificateCheckAllRequest) (resp *types.CertificateCheckAllResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
