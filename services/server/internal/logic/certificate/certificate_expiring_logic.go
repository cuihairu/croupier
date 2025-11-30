// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package certificate

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CertificateExpiringLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取即将过期的证书
func NewCertificateExpiringLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CertificateExpiringLogic {
	return &CertificateExpiringLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CertificateExpiringLogic) CertificateExpiring(req *types.CertificateExpiringRequest) (*types.CertificateExpiringResponse, error) {
	days := req.Days
	if days <= 0 {
		days = 30
	}

	certs, err := l.svcCtx.CertificateModel.ExpiringWithin(l.ctx, time.Hour*24*time.Duration(days))
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(certs))
	for i := range certs {
		items = append(items, utils.BuildCertificateDTO(&certs[i]))
	}

	return &types.CertificateExpiringResponse{
		Code:    0,
		Message: "OK",
		Data: map[string]interface{}{
			"items": items,
			"days":  days,
		},
	}, nil
}
