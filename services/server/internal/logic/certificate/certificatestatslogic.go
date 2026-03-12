// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书统计
func NewCertificateStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateStatsLogic {
	return &CertificateStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateStatsLogic) CertificateStats(req *types.CertificateStatsRequest) (resp *types.CertificateStatsResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
