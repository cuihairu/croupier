// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

)

type CertificateStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取证书统计
func NewCertificateStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateStatsLogic {
	return &CertificateStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateStatsLogic) CertificateStats(req *types.CertificateStatsRequest) (*types.CertificateStatsResponse, error) {
	stats, err := l.svcCtx.CertificateModel.Stats(l.ctx)
	if err != nil {
		return nil, err
	}

	return &types.CertificateStatsResponse{
		Code:    0,
		Message: "OK",
		Data:    stats,
	}, nil
}
